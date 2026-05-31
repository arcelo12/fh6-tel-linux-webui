package main

import (
	"database/sql"
	"encoding/json"
	"fh6-telemetry/parser"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

type UserTelemetryState struct {
	UserID             int64     `json:"userId"`
	Confirmed          bool      `json:"confirmed"`
	LastIP             string    `json:"lastIp"`
	LastSeen           time.Time `json:"lastSeen"`
	LastTelemetryEvent time.Time `json:"-"`
	PendingCarOrdinal  int32     `json:"pendingCarOrdinal"`
	PendingCarClass    int32     `json:"pendingCarClass"`
	PendingCarPi      int32     `json:"pendingCarPi"`
}

var (
	telemetryStatesMu sync.Mutex
	telemetryStates   = make(map[int64]*UserTelemetryState)

	portToUserMu sync.RWMutex
	portToUserID = make(map[int]*int64)

	userSessionManagersMu sync.Mutex
	userSessionManagers   = make(map[int64]*SessionManager)
)

const (
	portStart      = 20441
	portEnd        = 20540
	changeCooldown = 48 * time.Hour
)

// Initialize User Telemetry Subsystem
func initUserTelemetry(dbConn *sql.DB, hub *Hub) {
	// Load existing port assignments
	rows, err := dbConn.Query("SELECT id, assigned_port FROM users WHERE assigned_port IS NOT NULL")
	if err != nil {
		log.Printf("[Telemetry Init] Error querying ports: %v", err)
		return
	}
	defer rows.Close()

	portToUserMu.Lock()
	count := 0
	for rows.Next() {
		var uid int64
		var port int
		if err := rows.Scan(&uid, &port); err == nil {
			userIDVal := uid
			portToUserID[port] = &userIDVal
			count++
		}
	}
	portToUserMu.Unlock()
	log.Printf("[Telemetry Init] Loaded %d active port assignments", count)

	// Start static UDP listeners
	startUserTelemetryListeners(hub)
}

func getUserSessionManager(userID int64) *SessionManager {
	userSessionManagersMu.Lock()
	defer userSessionManagersMu.Unlock()
	sm, exists := userSessionManagers[userID]
	if !exists {
		settings := LoadSettings(userID)
		autoRecord, ok := settings["autoRecord"].(bool)
		if !ok {
			autoRecord = true
		}
		sm = NewSessionManager(autoRecord)
		userSessionManagers[userID] = sm
	}
	return sm
}

// Find a free port in 20441-20540 range
func getAvailablePort() (int, error) {
	rows, err := db.Query("SELECT assigned_port FROM users WHERE assigned_port IS NOT NULL")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	assigned := make(map[int]bool)
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err == nil {
			assigned[p] = true
		}
	}

	// Try random ports first to distribute ports nicely
	available := make([]int, 0)
	for p := portStart; p <= portEnd; p++ {
		if !assigned[p] {
			available = append(available, p)
		}
	}

	if len(available) == 0 {
		return 0, fmt.Errorf("no available ports in range %d-%d", portStart, portEnd)
	}

	// Pick a random port
	return available[rand.Intn(len(available))], nil
}

// Ensure logged in user has an assigned port (automatic provisioning)
func ensureUserHasPort(userID int64) (int, int64, error) {
	var assignedPort sql.NullInt64
	var portLastChanged int64
	err := db.QueryRow("SELECT assigned_port, COALESCE(port_last_changed, 0) FROM users WHERE id = ?", userID).Scan(&assignedPort, &portLastChanged)
	if err != nil {
		return 0, 0, err
	}

	if assignedPort.Valid && assignedPort.Int64 >= int64(portStart) && assignedPort.Int64 <= int64(portEnd) {
		return int(assignedPort.Int64), portLastChanged, nil
	}

	// Assign new port
	port, err := getAvailablePort()
	if err != nil {
		return 0, 0, err
	}

	_, err = db.Exec("UPDATE users SET assigned_port = ? WHERE id = ?", port, userID)
	if err != nil {
		return 0, 0, err
	}

	userIDVal := userID
	portToUserMu.Lock()
	portToUserID[port] = &userIDVal
	portToUserMu.Unlock()

	log.Printf("[Telemetry] Automatically provisioned port %d for user ID %d", port, userID)
	return port, 0, nil
}

// Start listeners for all 100 ports
func startUserTelemetryListeners(hub *Hub) {
	for port := portStart; port <= portEnd; port++ {
		go runUserUDPListener(port, hub)
	}
}

// Single UDP listener routine
func runUserUDPListener(port int, hub *Hub) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Printf("[UDP Listener] Port %d resolve failed: %v", port, err)
		return
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Printf("[UDP Listener] Port %d listen failed: %v", port, err)
		return
	}
	defer conn.Close()
	
	// Increase OS UDP read buffer to 8MB to prevent dropped packets under high load (100 players)
	if err := conn.SetReadBuffer(8 * 1024 * 1024); err != nil {
		log.Printf("[UDP Listener] Warning: failed to set read buffer for port %d: %v", port, err)
	}

	buf := make([]byte, 1500)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		portToUserMu.RLock()
		uIDPtr := portToUserID[port]
		var userID int64
		if uIDPtr != nil {
			userID = *uIDPtr
		}
		portToUserMu.RUnlock()

		if userID == 0 {
			continue // No user assigned to this port yet
		}

		dataCopy := make([]byte, n)
		copy(dataCopy, buf[:n])

		handleIncomingTelemetry(userID, port, remoteAddr.IP.String(), dataCopy, hub)
	}
}

// Process telemetry for a specific user
func handleIncomingTelemetry(userID int64, port int, clientIP string, data []byte, hub *Hub) {
	packet, err := parser.Parse(data)
	if err != nil {
		return
	}

	telemetryStatesMu.Lock()
	state, exists := telemetryStates[userID]
	if !exists {
		state = &UserTelemetryState{
			UserID:    userID,
			Confirmed: false,
			LastIP:    clientIP,
			LastSeen:  time.Now(),
		}
		telemetryStates[userID] = state
	}

	now := time.Now()
	// Ask for confirmation if IP changed or last packet was more than 30 mins ago
	ipChanged := state.LastIP != clientIP
	expired := now.Sub(state.LastSeen) > 30*time.Minute

	if ipChanged || expired {
		state.Confirmed = false
		state.LastIP = clientIP
	}
	state.LastSeen = now
	state.PendingCarOrdinal = packet.CarOrdinal
	state.PendingCarClass = packet.CarClass
	state.PendingCarPi = packet.CarPi

	confirmed := state.Confirmed
	telemetryStatesMu.Unlock()

	if !confirmed {
		telemetryStatesMu.Lock()
		shouldNotify := now.Sub(state.LastTelemetryEvent) > 5*time.Second
		if shouldNotify {
			state.LastTelemetryEvent = now
		}
		telemetryStatesMu.Unlock()

		if shouldNotify {
			type PendingPayload struct {
				Type       string `json:"type"`
				Port       int    `json:"port"`
				CarOrdinal int32  `json:"carOrdinal"`
				CarClass   int32  `json:"carClass"`
				CarPi      int32  `json:"carPi"`
				ClientIP   string `json:"clientIp"`
			}
			payloadBytes, err := json.Marshal(PendingPayload{
				Type:       "telemetry_pending",
				Port:       port,
				CarOrdinal: packet.CarOrdinal,
				CarClass:   packet.CarClass,
				CarPi:      packet.CarPi,
				ClientIP:   clientIP,
			})
			if err == nil {
				hub.broadcastToUser(userID, payloadBytes)
			}
		}
		return
	}

	// 1. Broadcast live telemetry packet directly to Svelte user client
	type BroadcastPayload struct {
		Type string                  `json:"type"`
		Data *parser.TelemetryPacket `json:"data"`
	}
	liveBytes, err := json.Marshal(BroadcastPayload{
		Type: "telemetry_live",
		Data: packet,
	})
	if err == nil {
		hub.broadcastToUser(userID, liveBytes)
	}

	// 2. Check if user is in a lobby room
	roomsMu.RLock()
	var activeLobby *LobbyRoom
	var slotNum int
	for _, room := range activeRooms {
		room.mu.Lock()
		for _, slot := range room.Slots {
			if slot.UserID == userID {
				activeLobby = room
				slotNum = slot.SlotNumber
				break
			}
		}
		room.mu.Unlock()
		if activeLobby != nil {
			break
		}
	}
	roomsMu.RUnlock()

	if activeLobby != nil {
		// Route to lobby room
		activeLobby.mu.Lock()
		activeLobby.Slots[slotNum-1].Connected = true
		activeLobby.Slots[slotNum-1].CarOrdinal = packet.CarOrdinal
		activeLobby.Slots[slotNum-1].CarClass = packet.CarClass
		activeLobby.Slots[slotNum-1].CarPi = packet.CarPi
		activeSessionID := activeLobby.ActiveSessionID
		isRecording := activeLobby.IsRecording
		username := activeLobby.Slots[slotNum-1].Username
		activeLobby.mu.Unlock()

		type LivePayload struct {
			Type       string                  `json:"type"`
			SlotNumber int                     `json:"slotNumber"`
			Username   string                  `json:"username"`
			Data       *parser.TelemetryPacket `json:"data"`
		}
		payloadBytes, err := json.Marshal(LivePayload{
			Type:       "telemetry",
			SlotNumber: slotNum,
			Username:   username,
			Data:       packet,
		})
		if err == nil {
			hub.broadcastToRoom(activeLobby.Code, payloadBytes)
		}

		if isRecording && activeSessionID > 0 && packet.CarOrdinal != 0 {
			// Fast RAM-first caching!
			activeLobby.mu.Lock()
			slot := activeLobby.Slots[slotNum-1]
			slot.PacketsRAM = append(slot.PacketsRAM, data)
			activeLobby.mu.Unlock()
		}
	} else {
		// Route to user's isolated solo session
		processSoloTelemetry(userID, packet, data)
	}
}

// isolated solo telemetry processor
func processSoloTelemetry(userID int64, packet *parser.TelemetryPacket, rawData []byte) {
	sm := getUserSessionManager(userID)
	nowMs := time.Now().UnixMilli()

	sm.Lock()
	wasRacing := sm.ActiveId != nil
	isRacing := packet.IsRaceOn
	action, carOrdinal, carClass, carPi := sm.OnRaceOnChange(wasRacing, isRacing, packet.CarOrdinal, packet.CarClass, packet.CarPi)

	var activeId *int64

	if action == "Open" {
		sm.Unlock()
		id := openSession(db, nowMs, carOrdinal, carClass, carPi, &userID)
		sm.Lock()
		if id >= 0 {
			sm.BeginNewSession()
			sm.ActiveId = &id
		}
	} else if action == "Close" {
		if sm.ActiveId != nil {
			id := *sm.ActiveId
			best := sm.BestForClose()
			sm.NoteClose(nowMs)
			sm.ActiveId = nil
			sm.Unlock()
			closeSession(db, id, nowMs, best)
			sm.Lock()
		}
	}

	if sm.ActiveId != nil && packet.CarOrdinal != 0 {
		updateSessionCarIfUnknown(db, *sm.ActiveId, packet.CarOrdinal, packet.CarClass, packet.CarPi)
	}

	if sm.ActiveId == nil {
		reopenId := sm.CheckReopen(float64(packet.CurrentRaceTime), nowMs)
		if reopenId != nil {
			sm.Unlock()
			reopenSession(db, *reopenId)
			sm.Lock()
			sm.ActiveId = reopenId
		}
	}

	if sm.ActiveId != nil {
		sm.UpdateRaceTime(float64(packet.CurrentRaceTime))
		completed, lapNum, lapTime := sm.NoteTick(packet.IsRaceOn, float64(packet.CurrentLap), float64(packet.CurrentRaceTime))
		activeId = sm.ActiveId
		sm.Unlock()
		if completed {
			insertLap(db, *activeId, lapNum, lapTime)
		}
		insertPacket(db, *activeId, packet.TimestampMs, rawData)
	} else {
		sm.Unlock()
	}
}

// REST API: Get Current User Port Status
func handleUserPortStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, ok := checkSession(r)
	if !ok {
		log.Printf("[Telemetry Debug] handleUserPortStatus unauthorized: cookie not found or invalid")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	port, portLastChanged, err := ensureUserHasPort(session.UserID)
	if err != nil {
		log.Printf("[Telemetry Debug] ensureUserHasPort failed for user ID %d: %v", session.UserID, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get port info"})
		return
	}
	log.Printf("[Telemetry Debug] handleUserPortStatus success for user ID %d: port=%d, portLastChanged=%d", session.UserID, port, portLastChanged)

	// Calculate if they can change port (48 hours cooldown)
	canChange := true
	var remainingTime string
	if portLastChanged > 0 {
		lastChangedTime := time.UnixMilli(portLastChanged)
		diff := time.Since(lastChangedTime)
		if diff < changeCooldown {
			canChange = false
			rem := changeCooldown - diff
			remainingTime = fmt.Sprintf("%d jam, %d menit", int(rem.Hours()), int(rem.Minutes())%60)
		}
	}

	// Get stream confirmation status
	confirmed := false
	var pendingInfo *UserTelemetryState
	telemetryStatesMu.Lock()
	if state, exists := telemetryStates[session.UserID]; exists {
		confirmed = state.Confirmed
		if !confirmed && time.Since(state.LastSeen) < 15*time.Second {
			pendingInfo = state
		}
	}
	telemetryStatesMu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"port":              port,
		"portLastChanged":   portLastChanged,
		"canChange":         canChange,
		"remainingCooldown": remainingTime,
		"confirmed":         confirmed,
		"pendingStream":     pendingInfo,
	})
}

// REST API: Change User Port (cooldown enforced)
func handleUserPortChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, ok := checkSession(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Check last changed cooldown
	var oldPort sql.NullInt64
	var portLastChanged int64
	err := db.QueryRow("SELECT assigned_port, COALESCE(port_last_changed, 0) FROM users WHERE id = ?", session.UserID).Scan(&oldPort, &portLastChanged)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database error"})
		return
	}

	if portLastChanged > 0 {
		lastChangedTime := time.UnixMilli(portLastChanged)
		if time.Since(lastChangedTime) < changeCooldown {
			rem := changeCooldown - time.Since(lastChangedTime)
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Anda hanya dapat mengubah port 1 kali setiap 2 hari. Sisa waktu: %d jam, %d menit.", int(rem.Hours()), int(rem.Minutes())%60),
			})
			return
		}
	}

	// Assign new port
	newPort, err := getAvailablePort()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	nowMs := time.Now().UnixMilli()
	_, err = db.Exec("UPDATE users SET assigned_port = ?, port_last_changed = ? WHERE id = ?", newPort, nowMs, session.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update port"})
		return
	}

	// Update local memory maps
	portToUserMu.Lock()
	if oldPort.Valid {
		delete(portToUserID, int(oldPort.Int64))
	}
	userIDVal := session.UserID
	portToUserID[newPort] = &userIDVal
	portToUserMu.Unlock()

	// Reset confirmation state
	telemetryStatesMu.Lock()
	delete(telemetryStates, session.UserID)
	telemetryStatesMu.Unlock()

	log.Printf("[Telemetry] User ID %d changed port to %d", session.UserID, newPort)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"port":            newPort,
		"portLastChanged": nowMs,
	})
}

// REST API: Confirm Telemetry Data Stream
func handleUserPortConfirm(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		session, ok := checkSession(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		telemetryStatesMu.Lock()
		state, exists := telemetryStates[session.UserID]
		if exists {
			state.Confirmed = true
			state.LastSeen = time.Now()
		} else {
			// Create dummy if confirm clicked before packets arrive
			telemetryStates[session.UserID] = &UserTelemetryState{
				UserID:    session.UserID,
				Confirmed: true,
				LastSeen:  time.Now(),
			}
		}
		telemetryStatesMu.Unlock()

		log.Printf("[Telemetry] User ID %d confirmed telemetry stream", session.UserID)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

// REST API: Reject/Reset Telemetry Data Stream
func handleUserPortReject(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		session, ok := checkSession(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		telemetryStatesMu.Lock()
		delete(telemetryStates, session.UserID)
		telemetryStatesMu.Unlock()

		log.Printf("[Telemetry] User ID %d rejected/reset telemetry stream", session.UserID)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}
