package main

import (
	"database/sql"
	"encoding/json"
	"fh6-telemetry/parser"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	IsMinimal      = "false" // Can be overridden at build time using -ldflags="-X main.IsMinimal=true"
	db             *sql.DB
	sessionManager *SessionManager
)

func main() {
	loadConfig()
	hub := newHub()
	
	// Settings Default (Fallback/Global)
	settingsData := LoadSettings(0)
	autoRecord := true
	if val, ok := settingsData["autoRecord"].(bool); ok {
		autoRecord = val
	}
	sessionManager = NewSessionManager(autoRecord)

	// 0. Database Setup
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, ".local", "share", "fh6-tel", "sessions_multi.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	var err error
	db, err = sql.Open("sqlite", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		log.Printf("Cannot open DB at %s: %v", dbPath, err)
	} else {
		// SQLite performs best with a single writer connection
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		// Initialize tables if they do not exist
		_, err := db.Exec(`
			PRAGMA journal_mode=WAL;
			PRAGMA busy_timeout=5000;
			PRAGMA synchronous=NORMAL;
			CREATE TABLE IF NOT EXISTS sessions (
				id INTEGER PRIMARY KEY,
				started_at INTEGER NOT NULL,
				ended_at INTEGER,
				car_ordinal INTEGER NOT NULL DEFAULT 0,
				car_class INTEGER NOT NULL DEFAULT 0,
				car_pi INTEGER NOT NULL DEFAULT 0,
				best_lap REAL,
				packet_count INTEGER NOT NULL DEFAULT 0,
				name TEXT,
				bookmarked INTEGER NOT NULL DEFAULT 0,
				user_id INTEGER REFERENCES users(id) ON DELETE CASCADE
			);
		`)
		if err != nil {
			log.Printf("Error creating sessions table: %v", err)
		}

		// Run migration for sessions table
		_, _ = db.Exec("ALTER TABLE sessions ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE")

		// Cleanup junk micro-sessions (less than 180 packets = ~3 seconds of data)
		// This keeps user lists consistent and clean from accidental pause/unpause spam.
		_, _ = db.Exec("DELETE FROM sessions WHERE packet_count < 180 AND ended_at IS NOT NULL")

		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS user_settings (
				user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
				settings_json TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS session_packets (
				id INTEGER PRIMARY KEY,
				session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
				timestamp_ms INTEGER NOT NULL,
				data BLOB NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_packets_session ON session_packets(session_id);
			CREATE TABLE IF NOT EXISTS session_laps (
				id INTEGER PRIMARY KEY,
				session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
				lap_number INTEGER NOT NULL,
				lap_time REAL NOT NULL,
				UNIQUE(session_id, lap_number)
			);
			CREATE INDEX IF NOT EXISTS idx_laps_session ON session_laps(session_id);

			-- Multiplayer Tables
			CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT UNIQUE NOT NULL,
				email TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				assigned_port INTEGER UNIQUE,
				port_last_changed INTEGER DEFAULT 0
			);
		`)
		if err != nil {
			log.Printf("Error creating users table: %v", err)
		}

		// Run migrations for existing DB instances to add columns if they don't exist
		// NOTE: SQLite does NOT support UNIQUE in ALTER TABLE ADD COLUMN.
		// We add the column first, then create a unique index separately.
		_, _ = db.Exec("ALTER TABLE users ADD COLUMN assigned_port INTEGER")
		_, _ = db.Exec("ALTER TABLE users ADD COLUMN port_last_changed INTEGER DEFAULT 0")
		_, _ = db.Exec("ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'")
		_, _ = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_assigned_port ON users(assigned_port) WHERE assigned_port IS NOT NULL")
		_, _ = db.Exec("ALTER TABLE multiplayer_sessions ADD COLUMN session_type TEXT DEFAULT 'race'")
		_, _ = db.Exec("ALTER TABLE users ADD COLUMN dashboard_layout TEXT")
		_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_verified INTEGER DEFAULT 0")
		_, _ = db.Exec("ALTER TABLE users ADD COLUMN verification_token TEXT")

		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS lobbies (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				room_code TEXT UNIQUE NOT NULL,
				host_id INTEGER REFERENCES users(id),
				created_at INTEGER NOT NULL,
				status TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS auth_sessions (
				session_id TEXT PRIMARY KEY,
				user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				username TEXT NOT NULL,
				expires_at INTEGER NOT NULL
			);
			CREATE TABLE IF NOT EXISTS multiplayer_sessions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				lobby_id INTEGER REFERENCES lobbies(id) ON DELETE CASCADE,
				name TEXT,
				started_at INTEGER NOT NULL,
				ended_at INTEGER,
				session_type TEXT DEFAULT 'race',
				track_id TEXT,
				best_lap_overall REAL
			);
			CREATE TABLE IF NOT EXISTS session_players (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				multi_session_id INTEGER REFERENCES multiplayer_sessions(id) ON DELETE CASCADE,
				user_id INTEGER REFERENCES users(id),
				slot_number INTEGER NOT NULL,
				car_ordinal INTEGER,
				car_class INTEGER,
				car_pi INTEGER,
				best_lap REAL,
				driver_name TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS multiplayer_session_packets (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				multi_session_id INTEGER REFERENCES multiplayer_sessions(id) ON DELETE CASCADE,
				slot_number INTEGER NOT NULL,
				timestamp_ms INTEGER NOT NULL,
				data BLOB NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_packets_multi_session ON multiplayer_session_packets(multi_session_id, slot_number);
			CREATE TABLE IF NOT EXISTS multiplayer_session_laps (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				multi_session_id INTEGER REFERENCES multiplayer_sessions(id) ON DELETE CASCADE,
				slot_number INTEGER NOT NULL,
				lap_number INTEGER NOT NULL,
				lap_time REAL NOT NULL,
				UNIQUE(multi_session_id, slot_number, lap_number)
			);
			CREATE INDEX IF NOT EXISTS idx_laps_multi_session ON multiplayer_session_laps(multi_session_id, slot_number);
		`)
		if err != nil {
			log.Printf("Error initializing tables: %v", err)
		} else {
			fmt.Printf("Connected to SQLite (WAL mode): %s\n", dbPath)
			// Clean up expired auth sessions
			_, _ = db.Exec("DELETE FROM auth_sessions WHERE expires_at < ?", time.Now().UnixMilli())
			
			if IsMinimal != "true" {
				// Initialize Multiplayer Lobby System
				initLobbySubsystem(db)
				// Initialize User Telemetry System
				initUserTelemetry(db, hub)
			}
			
			// Initialize Solo Database Worker
			go StartSoloDBWorker(db)
		}
	}

	// 1. HTTP Server & API Routes
	http.HandleFunc("/ws", hub.handleWS)

	// Expose Config for Frontend
	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"multiplayer": IsMinimal != "true",
		})
	})

	if IsMinimal != "true" {
		http.HandleFunc("/api/auth/register", handleRegister)
		http.HandleFunc("/api/auth/verify", handleVerify)
		http.HandleFunc("/api/auth/login", handleLogin)
		http.HandleFunc("/api/auth/logout", handleLogout)
		http.HandleFunc("/api/auth/me", handleMe)

		// User Telemetry Port APIs
		http.HandleFunc("/api/user/port", handleUserPortStatus)
		http.HandleFunc("/api/user/port/change", handleUserPortChange)
		http.HandleFunc("/api/user/port/confirm", handleUserPortConfirm(hub))
		http.HandleFunc("/api/user/port/reject", handleUserPortReject(hub))
		http.HandleFunc("/api/user/layout", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				handleGetLayout(w, r)
			} else {
				handleSaveLayout(w, r)
			}
		})

		// Lobby Rooms API
		http.HandleFunc("/api/lobby/create", handleCreateLobby(hub))
		http.HandleFunc("/api/lobby/join", handleJoinLobby(hub))
		http.HandleFunc("/api/lobby/leave", handleLeaveLobby(hub))
		http.HandleFunc("/api/lobby/status", handleLobbyStatus)
		http.HandleFunc("/api/lobby/start-record", handleStartRecord(hub))
		http.HandleFunc("/api/lobby/stop-record", handleStopRecord(hub))
		http.HandleFunc("/api/lobby/list", handleListLobbies)
		http.HandleFunc("/api/lobby/history", handleLobbyHistory)
		http.HandleFunc("/api/lobby/request-join", handleRequestJoin(hub))
		http.HandleFunc("/api/lobby/respond-join", handleRespondJoin(hub))

		// Admin APIs
		http.HandleFunc("/api/admin/stats", handleAdminStats)
		http.HandleFunc("/api/admin/users", handleAdminUsers)
		http.HandleFunc("/api/admin/users/role", handleAdminUpdateRole)
		http.HandleFunc("/api/admin/rooms", handleAdminRooms)
		http.HandleFunc("/api/admin/rooms/delete", handleAdminDeleteRoom(hub))
	}

	// Export APIs
	http.HandleFunc("/api/export/csv", handleExportCSV)
	http.HandleFunc("/api/export/json", handleExportJSON)
	
	http.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db == nil {
			w.Write([]byte(`[]`))
			return
		}

		session, loggedIn := checkSession(r)

		if r.Method == "DELETE" {
			if loggedIn {
				db.Exec("DELETE FROM sessions WHERE user_id = ?", session.UserID)
			} else {
				db.Exec("DELETE FROM sessions WHERE user_id IS NULL")
			}
			w.Write([]byte(`{"success":true}`))
			return
		}

		var query string
		var args []interface{}
		if loggedIn {
			query = `
				SELECT id, started_at, ended_at, car_ordinal, car_class, car_pi, best_lap, packet_count, name, bookmarked
				FROM sessions 
				WHERE user_id = ? AND (packet_count >= 180 OR ended_at IS NULL)
				ORDER BY bookmarked DESC, started_at DESC LIMIT 100
			`
			args = append(args, session.UserID)
		} else {
			query = `
				SELECT id, started_at, ended_at, car_ordinal, car_class, car_pi, best_lap, packet_count, name, bookmarked
				FROM sessions 
				WHERE user_id IS NULL AND (packet_count >= 180 OR ended_at IS NULL)
				ORDER BY bookmarked DESC, started_at DESC LIMIT 100
			`
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			w.Write([]byte(`[]`))
			return
		}
		defer rows.Close()

		var sessions []map[string]interface{}
		for rows.Next() {
			var id, startedAt, carOrdinal, carClass, carPi, packetCount int
			var endedAt sql.NullInt64
			var bestLap sql.NullFloat64
			var name sql.NullString
			var bookmarked int

			rows.Scan(&id, &startedAt, &endedAt, &carOrdinal, &carClass, &carPi, &bestLap, &packetCount, &name, &bookmarked)
			
			s := map[string]interface{}{
				"id": id, "startedAt": startedAt, "carOrdinal": carOrdinal,
				"carClass": carClass, "carPi": carPi, "packetCount": packetCount,
				"bookmarked": bookmarked != 0,
			}
			if endedAt.Valid { s["endedAt"] = endedAt.Int64 } else { s["endedAt"] = nil }
			if bestLap.Valid { s["bestLap"] = bestLap.Float64 } else { s["bestLap"] = nil }
			if name.Valid { s["name"] = name.String } else { s["name"] = nil }
			
			sessions = append(sessions, s)
		}
		
		if sessions == nil {
			w.Write([]byte(`[]`))
			return
		}
		json.NewEncoder(w).Encode(sessions)
	})

	http.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db == nil {
			w.Write([]byte(`[]`))
			return
		}

		parts := strings.Split(r.URL.Path, "/")
		
		if r.Method == "DELETE" && len(parts) == 4 {
			idStr := parts[3]
			id, _ := strconv.Atoi(idStr)
			
			// Enforce session ownership
			session, loggedIn := checkSession(r)
			var dbUserId sql.NullInt64
			err := db.QueryRow("SELECT user_id FROM sessions WHERE id = ?", id).Scan(&dbUserId)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"success":false}`))
				return
			}
			if loggedIn {
				if !dbUserId.Valid || dbUserId.Int64 != session.UserID {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"success":false}`))
					return
				}
			} else {
				if dbUserId.Valid {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"success":false}`))
					return
				}
			}
			
			db.Exec("DELETE FROM sessions WHERE id=?", id)
			w.Write([]byte(`{"success":true}`))
			return
		}

		if len(parts) >= 5 && parts[4] == "packets" {
			idStr := parts[3]
			id, _ := strconv.Atoi(idStr)

			// Enforce session ownership
			session, loggedIn := checkSession(r)
			var dbUserId sql.NullInt64
			err := db.QueryRow("SELECT user_id FROM sessions WHERE id = ?", id).Scan(&dbUserId)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`[]`))
				return
			}
			if loggedIn {
				if !dbUserId.Valid || dbUserId.Int64 != session.UserID {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`[]`))
					return
				}
			} else {
				if dbUserId.Valid {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`[]`))
					return
				}
			}

			rows, err := db.Query("SELECT data FROM session_packets WHERE session_id=? ORDER BY timestamp_ms ASC", id)
			if err != nil {
				w.Write([]byte(`[]`))
				return
			}
			defer rows.Close()

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
				w.Write([]byte(`[]`))
				return
			}
			json.NewEncoder(w).Encode(packets)
		} else {
			w.Write([]byte(`[]`))
		}
	})

	// Manual session control
	http.HandleFunc("/api/session/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if db == nil {
			w.Write([]byte(`{"error":"no db"}`))
			return
		}

		session, loggedIn := checkSession(r)
		var user_id *int64
		var sm *SessionManager
		if loggedIn {
			user_id = &session.UserID
			sm = getUserSessionManager(session.UserID)
		} else {
			sm = sessionManager
		}

		// Create session in DB first (no lock needed)
		nowMs := time.Now().UnixMilli()
		id := openSession(db, nowMs, 0, 0, 0, user_id)
		if id < 0 {
			w.Write([]byte(`{"error":"failed to create session"}`))
			return
		}

		// Now lock to update in-memory state
		sm.Lock()
		if sm.ActiveId != nil {
			sm.Unlock()
			// Close the orphan session we just created
			closeSession(db, id, nowMs, -1)
			w.Write([]byte(`{"error":"session already active"}`))
			return
		}
		sm.BeginNewSession()
		sm.IsManual = true
		sm.ActiveId = &id
		sm.Unlock()

		fmt.Printf("[Manual] Session %d started\n", id)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "startedAt": nowMs})
	})

	http.HandleFunc("/api/session/stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}

		session, loggedIn := checkSession(r)
		var sm *SessionManager
		if loggedIn {
			sm = getUserSessionManager(session.UserID)
		} else {
			sm = sessionManager
		}

		sm.Lock()
		if sm.ActiveId == nil {
			sm.Unlock()
			w.Write([]byte(`{"error":"no active session"}`))
			return
		}
		nowMs := time.Now().UnixMilli()
		id := *sm.ActiveId
		best := sm.BestForClose()
		sm.NoteClose(nowMs)
		sm.ActiveId = nil
		sm.Unlock()

		closeSession(db, id, nowMs, best)
		fmt.Printf("[Manual] Session %d stopped\n", id)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "endedAt": nowMs})
	})

	http.HandleFunc("/api/session/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		session, loggedIn := checkSession(r)
		var sm *SessionManager
		if loggedIn {
			sm = getUserSessionManager(session.UserID)
		} else {
			sm = sessionManager
		}

		sm.Lock()
		active := sm.ActiveId
		var sid int64
		if active != nil {
			sid = *active
		}
		sm.Unlock()
		if active != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"recording": true, "sessionId": sid})
		} else {
			w.Write([]byte(`{"recording":false,"sessionId":null}`))
		}
	})

	http.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		session, loggedIn := checkSession(r)
		if !loggedIn {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			return
		}

		if r.Method == "POST" {
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			
			// Save to DB
			SaveSettings(session.UserID, req)
			
			// Update in-memory session manager if autoRecord changed
			if val, ok := req["autoRecord"].(bool); ok {
				sm := getUserSessionManager(session.UserID)
				sm.Lock()
				sm.AutoRecord = val
				sm.Unlock()
			}
			
			w.Write([]byte(`{"success":true}`))
			return
		}
		
		// GET request: Load from DB and return
		settingsData := LoadSettings(session.UserID)
		json.NewEncoder(w).Encode(settingsData)
	})
	
	// Setup static files handler (dev or embedded release based on build tags)
	setupStaticHandlers()

	// 2. Start HTTP Server in background
	go func() {
		fmt.Println("Web server listening on port 5173 (Golang)...")
		if err := http.ListenAndServe(":5173", nil); err != nil {
			log.Fatal(err)
		}
	}()

	// 3. Start UDP Telemetry Server
	port := "0.0.0.0:20440"
	addr, err := net.ResolveUDPAddr("udp4", port)
	if err != nil {
		log.Fatalf("Gagal resolve port: %v\n", err)
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		fmt.Printf("Gagal listen UDP: %v\n", err)
		return
	}
	defer conn.Close()

	if err := conn.SetReadBuffer(8 * 1024 * 1024); err != nil {
		log.Printf("Warning: failed to set read buffer: %v", err)
	}

	fmt.Printf("Mendengarkan telemetri Forza di UDP port %s...\n", port)

	buf := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error membaca UDP: %v\n", err)
			continue
		}

		packet, err := parser.Parse(buf[:n])
		if err != nil {
			continue // Paket tidak valid
		}
		savePacket := true

		if db != nil {
			nowMs := time.Now().UnixMilli()

			// CREATE A COPY OF THE BUFFER TO PREVENT REFERENCE CAPTURE BUGS!
			dataCopy := make([]byte, n)
			copy(dataCopy, buf[:n])

			sessionManager.Lock()
			wasRacing := sessionManager.ActiveId != nil
			isRacing := packet.IsRaceOn
			action, carOrdinal, carClass, carPi := sessionManager.OnRaceOnChange(wasRacing, isRacing, packet.CarOrdinal, packet.CarClass, packet.CarPi)



			if action == "Open" {
				sessionManager.Unlock()
				id := openSession(db, nowMs, carOrdinal, carClass, carPi, nil)
				sessionManager.Lock()
				if id >= 0 {
					sessionManager.BeginNewSession()
					sessionManager.ActiveId = &id
				}
			} else if action == "Close" {
				if sessionManager.ActiveId != nil {
					id := *sessionManager.ActiveId
					best := sessionManager.BestForClose()
					sessionManager.NoteClose(nowMs)
					sessionManager.ActiveId = nil
					sessionManager.Unlock()
					closeSession(db, id, nowMs, best)
					sessionManager.Lock()
				}
			}

			if sessionManager.ActiveId != nil && packet.CarOrdinal != 0 {
				updateSessionCarIfUnknown(db, *sessionManager.ActiveId, packet.CarOrdinal, packet.CarClass, packet.CarPi)
			}

			if sessionManager.ActiveId == nil {
				reopenId := sessionManager.CheckReopen(float64(packet.CurrentRaceTime), nowMs)
				if reopenId != nil {
					sessionManager.Unlock()
					reopenSession(db, *reopenId)
					sessionManager.Lock()
					sessionManager.ActiveId = reopenId
				}
			}

			if sessionManager.ActiveId != nil {
				sessionManager.UpdateRaceTime(float64(packet.CurrentRaceTime))
				completed, lapNum, lapTime := sessionManager.NoteTick(packet.IsRaceOn, float64(packet.CurrentLap), float64(packet.CurrentRaceTime))
				activeId := sessionManager.ActiveId
				sessionManager.Unlock()
				if completed {
					insertLap(db, *activeId, lapNum, lapTime)
				}
				if savePacket {
					insertPacket(db, *activeId, packet.TimestampMs, dataCopy)
				}
			} else {
				sessionManager.Unlock()
			}
		}

		// Broadcast ke semua koneksi UI
		jsonData, err := json.Marshal(packet)
		if err == nil {
			hub.broadcast(jsonData)
		}
	}
}
