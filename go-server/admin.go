package main

import (
	"encoding/json"
	"net/http"
	"time"
	"log"
)

// checkAdmin validates if the current user is an admin
func checkAdmin(r *http.Request) (*SessionInfo, bool) {
	session, ok := checkSession(r)
	if !ok || session.Role != "admin" {
		return nil, false
	}
	return session, true
}

// handleAdminStats returns database and active server stats
func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if _, ok := checkAdmin(r); !ok {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden: Admin access required"})
		return
	}

	var stats struct {
		TotalUsers    int `json:"totalUsers"`
		ActiveRooms   int `json:"activeRooms"`
		TotalSessions int `json:"totalSessions"`
		TotalPackets  int `json:"totalPackets"`
	}

	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&stats.TotalSessions)
	db.QueryRow("SELECT SUM(packet_count) FROM sessions").Scan(&stats.TotalPackets)
	
	roomsMu.RLock()
	stats.ActiveRooms = len(activeRooms)
	roomsMu.RUnlock()

	json.NewEncoder(w).Encode(stats)
}

// handleAdminUsers lists all users
func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if _, ok := checkAdmin(r); !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT id, username, email, role, created_at FROM users ORDER BY id ASC")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type UserItem struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		Role      string `json:"role"`
		CreatedAt int64  `json:"createdAt"`
	}

	var users []UserItem
	for rows.Next() {
		var u UserItem
		rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt)
		users = append(users, u)
	}

	json.NewEncoder(w).Encode(users)
}

// handleAdminUpdateRole changes a user's role
func handleAdminUpdateRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, ok := checkAdmin(r)
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var req struct {
		UserID int64  `json:"userId"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid role"})
		return
	}

	_, err := db.Exec("UPDATE users SET role = ? WHERE id = ?", req.Role, req.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logAudit(session.UserID, session.Username, "UPDATE_USER_ROLE", "User ID "+fmt.Sprint(req.UserID)+" to "+req.Role, getIP(r))

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// handleAdminRooms lists all active rooms
func handleAdminRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if _, ok := checkAdmin(r); !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	type AdminRoomSummary struct {
		Code         string `json:"code"`
		HostUsername string `json:"hostUsername"`
		PlayerCount  int    `json:"playerCount"`
		IsPublic     bool   `json:"isPublic"`
		IsRecording  bool   `json:"isRecording"`
		CreatedAt    int64  `json:"createdAt"`
	}

	roomsMu.RLock()
	defer roomsMu.RUnlock()

	var list []AdminRoomSummary
	for _, room := range activeRooms {
		room.mu.Lock()
		count := 0
		for _, s := range room.Slots {
			if s.UserID > 0 {
				count++
			}
		}
		list = append(list, AdminRoomSummary{
			Code:         room.Code,
			HostUsername: room.HostUsername,
			PlayerCount:  count,
			IsPublic:     room.IsPublic,
			IsRecording:  room.IsRecording,
			CreatedAt:    room.CreatedAt,
		})
		room.mu.Unlock()
	}

	if list == nil {
		list = []AdminRoomSummary{}
	}
	json.NewEncoder(w).Encode(list)
}

// handleAdminDeleteRoom force closes a room
func handleAdminDeleteRoom(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		session, ok := checkAdmin(r)
		if !ok {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		var req struct {
			RoomCode string `json:"roomCode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		roomsMu.Lock()
		room, exists := activeRooms[req.RoomCode]
		if exists {
			delete(activeRooms, req.RoomCode)
		}
		roomsMu.Unlock()

		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Room not found"})
			return
		}

		// Update DB status
		db.Exec("UPDATE lobbies SET status = 'closed' WHERE room_code = ?", req.RoomCode)
		
		// End recording if active
		room.mu.Lock()
		if room.IsRecording && room.ActiveSessionID > 0 {
			nowMs := time.Now().UnixMilli()
			db.Exec("UPDATE multiplayer_sessions SET ended_at = ?, status = 'completed' WHERE id = ?", nowMs, room.ActiveSessionID)
		}
		room.mu.Unlock()

		// Broadcast kill message
		type KillMsg struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		}
		msg, _ := json.Marshal(KillMsg{Type: "room_closed", Reason: "Closed by Administrator"})
		hub.broadcastToRoom(req.RoomCode, msg)

		log.Printf("[Admin] Room %s forcefully closed", req.RoomCode)
		logAudit(session.UserID, session.Username, "DELETE_ROOM", "Room "+req.RoomCode, getIP(r))

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}
