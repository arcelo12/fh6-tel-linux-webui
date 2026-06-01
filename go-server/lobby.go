package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"fh6-telemetry/parser"
)

type JoinRequest struct {
	RequestID  string `json:"requestId"`
	UserID     int64  `json:"userId"`
	Username   string `json:"username"`
	RequestedAt int64 `json:"requestedAt"`
	Status     string `json:"status"` // "pending", "approved", "rejected"
}

type LobbySlot struct {
	SlotNumber int      `json:"slotNumber"`
	UserID     int64    `json:"userId"`
	Username   string   `json:"username"`
	CarOrdinal int32    `json:"carOrdinal"`
	CarClass   int32    `json:"carClass"`
	CarPi      int32    `json:"carPi"`
	Port       int      `json:"port"`
	Connected  bool     `json:"connected"`
	PacketsRAM [][]byte `json:"-"`
}

type LobbyRoom struct {
	Code            string                  `json:"code"`
	HostID          int64                   `json:"hostId"`
	HostUsername    string                  `json:"hostUsername"`
	ActiveSessionID int64                   `json:"activeSessionId"`
	IsRecording     bool                    `json:"isRecording"`
	Slots           [12]*LobbySlot          `json:"slots"`
	IsPublic        bool                    `json:"isPublic"`
	CreatedAt       int64                   `json:"createdAt"`
	JoinRequests    map[string]*JoinRequest  `json:"joinRequests,omitempty"`

	mu       sync.Mutex
}

type QueuedPacket struct {
	MultiSessionID int64
	SlotNumber     int
	TimestampMs    uint32
	Data           []byte
}

var (
	roomsMu       sync.RWMutex
	activeRooms   = make(map[string]*LobbyRoom)
)

// Initialize the Dynamic Lobby Subsystem
func initLobbySubsystem(dbConn *sql.DB) {
	// Replaced DB Worker with RAM-Buffer + Multicore flush approach!
}

// Generate Room Code (6 Alphanumeric characters)
func generateRoomCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Exclude confusing chars like I, O, 0, 1
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := 0; i < 6; i++ {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// Port allocation and dynamic UDP listener routines removed.
// Users use their statically assigned ports managed in telemetry.go.

// Broadcast room status changes
func (r *LobbyRoom) broadcastStatus(hub *Hub) {
	r.mu.Lock()
	defer r.mu.Unlock()

	type StatusPayload struct {
		Type  string     `json:"type"`
		Lobby *LobbyRoom `json:"lobby"`
	}
	payloadBytes, err := json.Marshal(StatusPayload{
		Type:  "lobby_update",
		Lobby: r,
	})
	if err == nil {
		hub.broadcastToRoom(r.Code, payloadBytes)
	}
}

// REST API: Create Lobby
func handleCreateLobby(hub *Hub) http.HandlerFunc {
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

		roomCode := generateRoomCode()
		room := &LobbyRoom{
			Code:         roomCode,
			HostID:       session.UserID,
			HostUsername: session.Username,
			IsPublic:     true,
			CreatedAt:    time.Now().UnixMilli(),
			JoinRequests: make(map[string]*JoinRequest),
		}

		// Initialize slot numbers with 0 port (will be populated when players join)
		for i := 0; i < 12; i++ {
			room.Slots[i] = &LobbySlot{
				SlotNumber: i + 1,
				Port:       0,
			}
		}

		roomsMu.Lock()
		activeRooms[roomCode] = room
		roomsMu.Unlock()

		log.Printf("[Lobby] Created room %s by %s", roomCode, session.Username)
		json.NewEncoder(w).Encode(room)
	}
}

// REST API: Join Lobby Slot
func handleJoinLobby(hub *Hub) http.HandlerFunc {
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

		var req struct {
			RoomCode   string `json:"roomCode"`
			SlotNumber int    `json:"slotNumber"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		roomsMu.RLock()
		room, ok := activeRooms[req.RoomCode]
		roomsMu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Lobby not found"})
			return
		}

		if req.SlotNumber < 1 || req.SlotNumber > 12 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid slot number"})
			return
		}

		// Ensure user has their personal port provisioned
		userPort, _, err := ensureUserHasPort(session.UserID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Gagal mengalokasikan port telemetri Anda"})
			return
		}

		room.mu.Lock()
		slotIndex := req.SlotNumber - 1

		// Verify if user is already in another slot
		for idx, s := range room.Slots {
			if s.UserID == session.UserID {
				// Empty previous slot
				room.Slots[idx].UserID = 0
				room.Slots[idx].Username = ""
				room.Slots[idx].Port = 0
				room.Slots[idx].Connected = false
			}
		}

		// Set slot
		room.Slots[slotIndex].UserID = session.UserID
		room.Slots[slotIndex].Username = session.Username
		room.Slots[slotIndex].Port = userPort
		room.Slots[slotIndex].Connected = false
		room.mu.Unlock()

		room.broadcastStatus(hub)
		json.NewEncoder(w).Encode(room)
	}
}

// REST API: Leave Lobby Slot
func handleLeaveLobby(hub *Hub) http.HandlerFunc {
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

		var req struct {
			RoomCode string `json:"roomCode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		roomsMu.RLock()
		room, ok := activeRooms[req.RoomCode]
		roomsMu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Lobby not found"})
			return
		}

		room.mu.Lock()
		for idx, s := range room.Slots {
			if s.UserID == session.UserID {
				room.Slots[idx].UserID = 0
				room.Slots[idx].Username = ""
				room.Slots[idx].Port = 0
				room.Slots[idx].Connected = false
			}
		}
		room.mu.Unlock()

		room.broadcastStatus(hub)
		json.NewEncoder(w).Encode(room)
	}
}

// REST API: Get Lobby Status
func handleLobbyStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	code := r.URL.Query().Get("code")
	roomsMu.RLock()
	room, ok := activeRooms[code]
	roomsMu.RUnlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Lobby not found"})
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()
	json.NewEncoder(w).Encode(room)
}

// REST API: Start Recording (Host only)
func handleStartRecord(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		session, ok := checkSession(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req struct {
			RoomCode    string `json:"roomCode"`
			SessionName string `json:"sessionName"`
			SessionType string `json:"sessionType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.SessionType == "" {
			req.SessionType = "race"
		}

		roomsMu.RLock()
		room, ok := activeRooms[req.RoomCode]
		roomsMu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if room.HostID != session.UserID {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Only host can control recording"})
			return
		}

		room.mu.Lock()
		if room.IsRecording {
			room.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "sessionID": room.ActiveSessionID})
			return
		}

		// Insert multiplayer session into database
		nowMs := time.Now().UnixMilli()
		res, err := db.Exec(
			"INSERT INTO multiplayer_sessions (lobby_id, name, session_type, started_at, status) VALUES ((SELECT id FROM lobbies WHERE room_code = ?), ?, ?, ?, 'recording')",
			room.Code, req.SessionName, req.SessionType, nowMs,
		)
		var sessionID int64
		if err != nil {
			// Fallback: create mock lobby entry if not exist in DB yet
			db.Exec("INSERT OR IGNORE INTO lobbies (room_code, created_at, status) VALUES (?, ?, 'active')", room.Code, nowMs)
			res, err = db.Exec(
				"INSERT INTO multiplayer_sessions (lobby_id, name, session_type, started_at, status) VALUES ((SELECT id FROM lobbies WHERE room_code = ?), ?, ?, ?, 'recording')",
				room.Code, req.SessionName, req.SessionType, nowMs,
			)
		}
		if err == nil {
			sessionID, _ = res.LastInsertId()
		}

		// Insert slot bindings for active slot players
		for _, s := range room.Slots {
			if s.UserID > 0 {
				var uid sql.NullInt64
				uid.Int64 = s.UserID
				uid.Valid = true
				db.Exec(
					"INSERT INTO session_players (multi_session_id, user_id, slot_number, driver_name) VALUES (?, ?, ?, ?)",
					sessionID, uid, s.SlotNumber, s.Username,
				)
			}
		}

		room.IsRecording = true
		room.ActiveSessionID = sessionID
		room.mu.Unlock()

		room.broadcastStatus(hub)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "sessionID": sessionID})
	}
}

// REST API: Stop Recording (Host only)
func handleStopRecord(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		session, ok := checkSession(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req struct {
			RoomCode string `json:"roomCode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		roomsMu.RLock()
		room, ok := activeRooms[req.RoomCode]
		roomsMu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if room.HostID != session.UserID {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Only host can control recording"})
			return
		}

		room.mu.Lock()
		if !room.IsRecording {
			room.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		nowMs := time.Now().UnixMilli()
		db.Exec("UPDATE multiplayer_sessions SET ended_at = ?, status = 'completed' WHERE id = ?", nowMs, room.ActiveSessionID)

		sessionIDToFlush := room.ActiveSessionID
		room.IsRecording = false
		room.ActiveSessionID = 0

		// Flush all PacketsRAM for all slots using MULTICORE!
		for i := 0; i < 12; i++ {
			packets := room.Slots[i].PacketsRAM
			slotNum := room.Slots[i].SlotNumber
			room.Slots[i].PacketsRAM = nil // clear RAM
			
			if len(packets) > 0 {
				go func(sid int64, sNum int, pkts [][]byte) {
					tx, err := db.Begin()
					if err != nil { return }
					stmt, err := tx.Prepare("INSERT INTO multiplayer_session_packets (multi_session_id, slot_number, timestamp_ms, data) VALUES (?, ?, ?, ?)")
					if err != nil { tx.Rollback(); return }
					
					for _, pData := range pkts {
						if len(pData) > 12 { // basic length check
							// Hacky fast way to extract timestamp_ms from byte buffer without full struct parse:
							// Wait, just use parser.Parse to be safe
							if pkt, err := parser.Parse(pData); err == nil {
								stmt.Exec(sid, sNum, pkt.TimestampMs, pData)
							}
						}
					}
					stmt.Close()
					tx.Commit()
				}(sessionIDToFlush, slotNum, packets)
			}
		}
		
		room.mu.Unlock()

		room.broadcastStatus(hub)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

// ─── Nearby Rooms ──────────────────────────────────────────────────────────

// REST API: List all active public lobbies
func handleListLobbies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type RoomSummary struct {
		Code         string `json:"code"`
		HostUsername string `json:"hostUsername"`
		PlayerCount  int    `json:"playerCount"`
		MaxSlots     int    `json:"maxSlots"`
		IsRecording  bool   `json:"isRecording"`
	}

	roomsMu.RLock()
	defer roomsMu.RUnlock()

	var list []RoomSummary
	for _, room := range activeRooms {
		if !room.IsPublic {
			continue
		}
		room.mu.Lock()
		count := 0
		for _, s := range room.Slots {
			if s.UserID > 0 {
				count++
			}
		}
		list = append(list, RoomSummary{
			Code:         room.Code,
			HostUsername: room.HostUsername,
			PlayerCount:  count,
			MaxSlots:     12,
			IsRecording:  room.IsRecording,
		})
		room.mu.Unlock()
	}

	if list == nil {
		list = []RoomSummary{}
	}
	json.NewEncoder(w).Encode(list)
}

// REST API: Request to join a lobby (requires host approval)
func handleRequestJoin(hub *Hub) http.HandlerFunc {
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

		var req struct {
			RoomCode string `json:"roomCode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		roomsMu.RLock()
		room, ok := activeRooms[req.RoomCode]
		roomsMu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Room not found"})
			return
		}

		// Generate unique request ID
		b := make([]byte, 6)
		rand.Read(b)
		reqID := fmt.Sprintf("%x", b)

		joinReq := &JoinRequest{
			RequestID:   reqID,
			UserID:      session.UserID,
			Username:    session.Username,
			RequestedAt: time.Now().UnixMilli(),
			Status:      "pending",
		}

		room.mu.Lock()
		// Check if user is already in room
		for _, s := range room.Slots {
			if s.UserID == session.UserID {
				room.mu.Unlock()
				json.NewEncoder(w).Encode(map[string]string{"error": "Already in this room"})
				return
			}
		}
		// Check if already has pending request
		for _, existing := range room.JoinRequests {
			if existing.UserID == session.UserID && existing.Status == "pending" {
				room.mu.Unlock()
				json.NewEncoder(w).Encode(map[string]interface{}{
					"requestId": existing.RequestID,
					"status":    "pending",
				})
				return
			}
		}
		room.JoinRequests[reqID] = joinReq
		room.mu.Unlock()

		log.Printf("[Lobby] Join request %s from %s to room %s", reqID, session.Username, req.RoomCode)

		// Notify host via WebSocket
		type JoinRequestNotif struct {
			Type    string      `json:"type"`
			RoomCode string     `json:"roomCode"`
			Request *JoinRequest `json:"request"`
		}
		notif, _ := json.Marshal(JoinRequestNotif{
			Type:     "join_request",
			RoomCode: req.RoomCode,
			Request:  joinReq,
		})
		hub.sendToUser(room.HostID, notif)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"requestId": reqID,
			"status":    "pending",
		})
	}
}

// REST API: Host approves or rejects a join request
func handleRespondJoin(hub *Hub) http.HandlerFunc {
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

		var req struct {
			RoomCode  string `json:"roomCode"`
			RequestID string `json:"requestId"`
			Approve   bool   `json:"approve"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		roomsMu.RLock()
		room, ok := activeRooms[req.RoomCode]
		roomsMu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Room not found"})
			return
		}

		if room.HostID != session.UserID {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Only host can respond to requests"})
			return
		}

		room.mu.Lock()
		joinReq, exists := room.JoinRequests[req.RequestID]
		if !exists || joinReq.Status != "pending" {
			room.mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Request not found or already handled"})
			return
		}

		var assignedSlot int
		if req.Approve {
			joinReq.Status = "approved"
			// Auto-assign to first free slot
			for idx, s := range room.Slots {
				if s.UserID == 0 {
					assignedSlot = idx + 1
					break
				}
			}
		} else {
			joinReq.Status = "rejected"
		}
		room.mu.Unlock()

		// If approved, provision their port and assign slot
		if req.Approve && assignedSlot > 0 {
			userPort, _, err := ensureUserHasPort(joinReq.UserID)
			if err == nil {
				room.mu.Lock()
				slotIdx := assignedSlot - 1
				room.Slots[slotIdx].UserID = joinReq.UserID
				room.Slots[slotIdx].Username = joinReq.Username
				room.Slots[slotIdx].Port = userPort
				room.Slots[slotIdx].Connected = false
				room.mu.Unlock()
				room.broadcastStatus(hub)
			}
		}

		log.Printf("[Lobby] Request %s %s by host %s", req.RequestID,
			map[bool]string{true: "approved", false: "rejected"}[req.Approve],
			session.Username)

		// Notify requester via WebSocket
		type JoinResponseNotif struct {
			Type      string `json:"type"`
			RoomCode  string `json:"roomCode"`
			RequestID string `json:"requestId"`
			Approved  bool   `json:"approved"`
			SlotNumber int   `json:"slotNumber,omitempty"`
		}
		notif, _ := json.Marshal(JoinResponseNotif{
			Type:       "join_response",
			RoomCode:   req.RoomCode,
			RequestID:  req.RequestID,
			Approved:   req.Approve,
			SlotNumber: assignedSlot,
		})
		hub.sendToUser(joinReq.UserID, notif)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"approved":   req.Approve,
			"slotNumber": assignedSlot,
		})
	}
}

// REST API: Get Multiplayer History
func handleLobbyHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, ok := checkSession(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Get all multiplayer session slots for any session where the user participated
	rows, err := db.Query(`
		SELECT m.id, l.room_code, m.name, m.session_type, m.started_at, m.ended_at, sp.slot_number, sp.car_ordinal, sp.driver_name,
			(SELECT COUNT(*) FROM multiplayer_session_packets WHERE multi_session_id = m.id AND slot_number = sp.slot_number) as packet_count
		FROM multiplayer_sessions m
		JOIN lobbies l ON m.lobby_id = l.id
		JOIN session_players sp ON m.id = sp.multi_session_id
		WHERE m.id IN (SELECT multi_session_id FROM session_players WHERE user_id = ?)
		ORDER BY m.started_at DESC, sp.slot_number ASC
	`, session.UserID)

	if err != nil {
		log.Printf("Error fetching lobby history: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var id int64
		var roomCode, name, sessionType, driverName string
		var startedAt int64
		var endedAt sql.NullInt64
		var slotNumber, carOrdinal, packetCount int
		
		err := rows.Scan(&id, &roomCode, &name, &sessionType, &startedAt, &endedAt, &slotNumber, &carOrdinal, &driverName, &packetCount)
		if err == nil {
			history = append(history, map[string]interface{}{
				"id": id,
				"roomCode": roomCode,
				"name": name,
				"sessionType": sessionType,
				"startedAt": startedAt,
				"endedAt": endedAt.Int64,
				"slotNumber": slotNumber,
				"carOrdinal": carOrdinal,
				"driverName": driverName,
				"packetCount": packetCount,
			})
		}
	}

	if history == nil {
		history = make([]map[string]interface{}, 0)
	}

	json.NewEncoder(w).Encode(history)
}
