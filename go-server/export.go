package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fh6-telemetry/parser"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// Export CSV Handler
func handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sessionIDStr := r.URL.Query().Get("session_id")
	slotNumberStr := r.URL.Query().Get("slot_number")

	sessionID, _ := strconv.ParseInt(sessionIDStr, 10, 64)
	if sessionID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid session_id"))
		return
	}

	var rows *sql.Rows
	var err error
	var filename string

	if slotNumberStr == "" {
		// Single Player Export
		rows, err = db.Query("SELECT data FROM session_packets WHERE session_id = ? ORDER BY timestamp_ms ASC", sessionID)
		var carOrdinal int32
		var sessionName sql.NullString
		db.QueryRow("SELECT car_ordinal, name FROM sessions WHERE id = ?", sessionID).Scan(&carOrdinal, &sessionName)
		
		namePart := "solo"
		if sessionName.Valid && sessionName.String != "" {
			namePart = sessionName.String
		}
		filename = fmt.Sprintf("fh6_telemetry_solo_session_%d_%s.csv", sessionID, namePart)
	} else {
		// Multiplayer Slot Export
		slotNumber, _ := strconv.Atoi(slotNumberStr)
		if slotNumber < 1 || slotNumber > 12 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid slot_number"))
			return
		}

		rows, err = db.Query("SELECT data FROM multiplayer_session_packets WHERE multi_session_id = ? AND slot_number = ? ORDER BY timestamp_ms ASC", sessionID, slotNumber)
		var driverName string
		db.QueryRow("SELECT driver_name FROM session_players WHERE multi_session_id = ? AND slot_number = ?", sessionID, slotNumber).Scan(&driverName)
		if driverName == "" {
			driverName = fmt.Sprintf("slot_%d", slotNumber)
		}
		filename = fmt.Sprintf("fh6_telemetry_multi_session_%d_%s.csv", sessionID, driverName)
	}

	if err != nil {
		log.Printf("Export CSV query error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "text/csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write CSV Header
	header := []string{
		"Time_s", "Distance_m", "Speed_kmh", "RPM", "Gear", 
		"Throttle_pct", "Brake_pct", "Handbrake_pct", "Steer_pct",
		"AccelX_G", "AccelY_G", "AccelZ_G",
		"PositionX", "PositionY", "PositionZ",
		"TireTempFL", "TireTempFR", "TireTempRL", "TireTempRR",
	}
	writer.Write(header)

	var startTimeMs uint32
	isFirst := true

	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err == nil {
			if pkt, err := parser.Parse(data); err == nil {
				if isFirst {
					startTimeMs = pkt.TimestampMs
					isFirst = false
				}

				timeSec := float64(pkt.TimestampMs-startTimeMs) / 1000.0
				speedKmh := pkt.SpeedMs * 3.6
				throttlePct := (float64(pkt.Throttle) / 255.0) * 100.0
				brakePct := (float64(pkt.Brake) / 255.0) * 100.0
				handbrakePct := (float64(pkt.Handbrake) / 255.0) * 100.0
				steerPct := (float64(pkt.Steer) / 127.0) * 100.0

				accelXG := pkt.AccelX / 9.80665
				accelYG := pkt.AccelY / 9.80665
				accelZG := pkt.AccelZ / 9.80665

				row := []string{
					fmt.Sprintf("%.3f", timeSec),
					fmt.Sprintf("%.1f", pkt.DistanceTraveled),
					fmt.Sprintf("%.2f", speedKmh),
					fmt.Sprintf("%.0f", pkt.CurrentEngineRpm),
					fmt.Sprintf("%d", pkt.Gear),
					fmt.Sprintf("%.1f", throttlePct),
					fmt.Sprintf("%.1f", brakePct),
					fmt.Sprintf("%.1f", handbrakePct),
					fmt.Sprintf("%.1f", steerPct),
					fmt.Sprintf("%.3f", accelXG),
					fmt.Sprintf("%.3f", accelYG),
					fmt.Sprintf("%.3f", accelZG),
					fmt.Sprintf("%.3f", pkt.PositionX),
					fmt.Sprintf("%.3f", pkt.PositionY),
					fmt.Sprintf("%.3f", pkt.PositionZ),
					fmt.Sprintf("%.1f", pkt.TireTempFl),
					fmt.Sprintf("%.1f", pkt.TireTempFr),
					fmt.Sprintf("%.1f", pkt.TireTempRl),
					fmt.Sprintf("%.1f", pkt.TireTempRr),
				}
				writer.Write(row)
			}
		}
	}
}

// Export JSON Handler
func handleExportJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sessionIDStr := r.URL.Query().Get("session_id")
	slotNumberStr := r.URL.Query().Get("slot_number")

	sessionID, _ := strconv.ParseInt(sessionIDStr, 10, 64)
	if sessionID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid session_id"))
		return
	}

	var rows *sql.Rows
	var err error
	var filename string

	if slotNumberStr == "" {
		rows, err = db.Query("SELECT data FROM session_packets WHERE session_id = ? ORDER BY timestamp_ms ASC", sessionID)
		var sessionName sql.NullString
		db.QueryRow("SELECT name FROM sessions WHERE id = ?", sessionID).Scan(&sessionName)
		namePart := "solo"
		if sessionName.Valid && sessionName.String != "" {
			namePart = sessionName.String
		}
		filename = fmt.Sprintf("fh6_telemetry_solo_session_%d_%s.json", sessionID, namePart)
	} else {
		slotNumber, _ := strconv.Atoi(slotNumberStr)
		if slotNumber < 1 || slotNumber > 12 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid slot_number"))
			return
		}
		rows, err = db.Query("SELECT data FROM multiplayer_session_packets WHERE multi_session_id = ? AND slot_number = ? ORDER BY timestamp_ms ASC", sessionID, slotNumber)
		var driverName string
		db.QueryRow("SELECT driver_name FROM session_players WHERE multi_session_id = ? AND slot_number = ?", sessionID, slotNumber).Scan(&driverName)
		if driverName == "" {
			driverName = fmt.Sprintf("slot_%d", slotNumber)
		}
		filename = fmt.Sprintf("fh6_telemetry_multi_session_%d_%s.json", sessionID, driverName)
	}

	if err != nil {
		log.Printf("Export JSON query error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/json")

	var packets []*parser.TelemetryPacket
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err == nil {
			if pkt, err := parser.Parse(data); err == nil {
				packets = append(packets, pkt)
			}
		}
	}

	if packets == nil {
		w.Write([]byte("[]"))
		return
	}

	json.NewEncoder(w).Encode(packets)
}
