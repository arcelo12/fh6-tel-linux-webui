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
	db             *sql.DB
	sessionManager *SessionManager
)

func main() {
	hub := newHub()
	
	// Settings Default
	settingsData := LoadSettings()
	autoRecord := true
	if val, ok := settingsData["autoRecord"].(bool); ok {
		autoRecord = val
	}
	sessionManager = NewSessionManager(autoRecord)

	// 0. Database Setup
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, ".local", "share", "fh6-tel", "sessions.db")
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
				bookmarked INTEGER NOT NULL DEFAULT 0
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
		`)
		if err != nil {
			log.Printf("Error initializing tables: %v", err)
		} else {
			fmt.Printf("Connected to SQLite (WAL mode): %s\n", dbPath)
		}
	}

	// 1. HTTP Server & API Routes
	http.HandleFunc("/ws", hub.handleWS)
	
	http.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db == nil {
			w.Write([]byte(`[]`))
			return
		}

		rows, err := db.Query(`
			SELECT id, started_at, ended_at, car_ordinal, car_class, car_pi, best_lap, packet_count, name, bookmarked
			FROM sessions ORDER BY bookmarked DESC, started_at DESC LIMIT 100
		`)
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
		if len(parts) >= 5 && parts[4] == "packets" {
			idStr := parts[3]
			id, _ := strconv.Atoi(idStr)

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

		// Create session in DB first (no lock needed)
		nowMs := time.Now().UnixMilli()
		id := openSession(db, nowMs, 0, 0, 0)
		if id < 0 {
			w.Write([]byte(`{"error":"failed to create session"}`))
			return
		}

		// Now lock to update in-memory state
		sessionManager.Lock()
		if sessionManager.ActiveId != nil {
			sessionManager.Unlock()
			// Close the orphan session we just created
			closeSession(db, id, nowMs, -1)
			w.Write([]byte(`{"error":"session already active"}`))
			return
		}
		sessionManager.BeginNewSession()
		sessionManager.ActiveId = &id
		sessionManager.Unlock()

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

		sessionManager.Lock()
		if sessionManager.ActiveId == nil {
			sessionManager.Unlock()
			w.Write([]byte(`{"error":"no active session"}`))
			return
		}
		nowMs := time.Now().UnixMilli()
		id := *sessionManager.ActiveId
		best := sessionManager.BestForClose()
		sessionManager.NoteClose(nowMs)
		sessionManager.ActiveId = nil
		sessionManager.Unlock()

		closeSession(db, id, nowMs, best)
		fmt.Printf("[Manual] Session %d stopped\n", id)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "endedAt": nowMs})
	})

	http.HandleFunc("/api/session/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		sessionManager.Lock()
		active := sessionManager.ActiveId
		var sid int64
		if active != nil {
			sid = *active
		}
		sessionManager.Unlock()
		if active != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"recording": true, "sessionId": sid})
		} else {
			w.Write([]byte(`{"recording":false,"sessionId":null}`))
		}
	})

	http.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			
			// Save to file
			SaveSettings(req)
			
			// Update in-memory session manager if autoRecord changed
			if val, ok := req["autoRecord"].(bool); ok {
				sessionManager.Lock()
				sessionManager.AutoRecord = val
				sessionManager.Unlock()
			}
			
			w.Write([]byte(`{"success":true}`))
			return
		}
		
		// GET request: Load from file and return
		settingsData := LoadSettings()
		json.NewEncoder(w).Encode(settingsData)
	})
	
	// Serve the raw UI folder directly (No Svelte/Build needed)
	uiFs := http.FileServer(http.Dir("../ui"))
	http.Handle("/ui/", http.StripPrefix("/ui/", uiFs))

	// Serve SvelteKit build folder with SPA fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join("../build", filepath.Clean(r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// If file doesn't exist, serve SPA fallback (index.html)
			http.ServeFile(w, r, "../build/index.html")
			return
		}
		// Otherwise serve the static file
		http.FileServer(http.Dir("../build")).ServeHTTP(w, r)
	})

	// 2. Start HTTP Server in background
	go func() {
		fmt.Println("Web server listening on port 5173 (Golang)...")
		if err := http.ListenAndServe(":5173", nil); err != nil {
			log.Fatal(err)
		}
	}()

	// 3. Start UDP Telemetry Server
	port := ":20440"
	addr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		log.Fatalf("Gagal resolve port: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Gagal mendengarkan di UDP %s: %v\n", port, err)
	}
	defer conn.Close()

	fmt.Printf("Mendengarkan telemetri Forza di UDP port %s...\n", port)

	buf := make([]byte, 1024)
	udpTick := 0
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
		udpTick++
		savePacket := (udpTick % 5 == 0)

		if db != nil {
			nowMs := time.Now().UnixMilli()

			sessionManager.Lock()
			wasRacing := sessionManager.ActiveId != nil
			isRacing := packet.IsRaceOn
			action, carOrdinal, carClass, carPi := sessionManager.OnRaceOnChange(wasRacing, isRacing, packet.CarOrdinal, packet.CarClass, packet.CarPi)

			var activeId *int64

			if action == "Open" {
				sessionManager.Unlock()
				id := openSession(db, nowMs, carOrdinal, carClass, carPi)
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
				activeId = sessionManager.ActiveId
				sessionManager.Unlock()
				if completed {
					insertLap(db, *activeId, lapNum, lapTime)
				}
				if savePacket {
					insertPacket(db, *activeId, packet.TimestampMs, buf[:n])
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
