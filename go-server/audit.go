package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type AuditLog struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	IPAddress  string `json:"ipAddress"`
	TimestampMs int64  `json:"timestampMs"`
}

// logAudit records an action into the audit_logs table.
// If userID is 0, it logs as an anonymous/system action.
func logAudit(userID int64, username, action, target, ipAddress string) {
	go func() {
		_, err := db.Exec(
			"INSERT INTO audit_logs (user_id, username, action, target, ip_address, timestamp_ms) VALUES (?, ?, ?, ?, ?, ?)",
			userID, username, action, target, ipAddress, time.Now().UnixMilli(),
		)
		if err != nil {
			log.Printf("[Audit] Error recording audit log: %v", err)
		}
	}()
}

// handleAdminAuditLogs returns a list of audit logs for the admin dashboard.
func handleAdminAuditLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, ok := checkSession(r)
	if !ok || session.Role != "admin" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden: Admins only"})
		return
	}

	rows, err := db.Query("SELECT id, user_id, username, action, target, ip_address, timestamp_ms FROM audit_logs ORDER BY timestamp_ms DESC LIMIT 100")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.Action, &l.Target, &l.IPAddress, &l.TimestampMs); err != nil {
			continue
		}
		logs = append(logs, l)
	}

	if logs == nil {
		logs = []AuditLog{}
	}

	json.NewEncoder(w).Encode(logs)
}
