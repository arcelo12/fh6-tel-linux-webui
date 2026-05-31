package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

const rewindWindowMs = 30000
const rewindMinRaceTime = 5.0
const minLapSecs = 20.0
const rewindGuardTicks = 60

type SessionManager struct {
	mu                  sync.Mutex
	AutoRecord          bool
	ActiveId            *int64
	bestLap             float64
	lastRaceTime        float64
	peakRaceTime        float64
	prevCurrentLap      float64
	curLapPeak          float64
	prevRaceTime        float64
	rewindGuard         int
	lapsRecorded        int
	ticks               int
	closedId            *int64
	closedWallMs        int64
	lastRaceTimeAtClose float64
}

func NewSessionManager(autoRecord bool) *SessionManager {
	return &SessionManager{
		AutoRecord: autoRecord,
		bestLap:    math.Inf(1),
	}
}

func (s *SessionManager) Lock()   { s.mu.Lock() }
func (s *SessionManager) Unlock() { s.mu.Unlock() }

func (s *SessionManager) BeginNewSession() {
	s.bestLap = math.Inf(1)
	s.prevCurrentLap = 0.0
	s.curLapPeak = 0.0
	s.prevRaceTime = 0.0
	s.rewindGuard = 0
	s.lapsRecorded = 0
	s.ticks = 0
	s.peakRaceTime = 0.0
}

func (s *SessionManager) updateBestLap(lap float64) {
	if lap > 0.0 && lap < s.bestLap {
		s.bestLap = lap
	}
}

func (s *SessionManager) NoteTick(isRaceOn bool, currentLap float64, raceTime float64) (bool, int, float64) {
	s.ticks++
	if currentLap > s.curLapPeak {
		s.curLapPeak = currentLap
	}

	if !isRaceOn || (raceTime > 0.0 && raceTime+0.25 < s.prevRaceTime) {
		s.rewindGuard = rewindGuardTicks
	}
	if raceTime > 0.0 {
		s.prevRaceTime = raceTime
	}

	completed := false
	var lapNum int
	var lapTime float64

	if isRaceOn && s.rewindGuard == 0 && s.prevCurrentLap > minLapSecs && currentLap < 1.0 {
		t := s.curLapPeak
		s.curLapPeak = currentLap
		idx := s.lapsRecorded
		s.lapsRecorded++
		s.updateBestLap(t)
		completed = true
		lapNum = idx
		lapTime = t
	}

	if s.rewindGuard > 0 && isRaceOn {
		s.rewindGuard--
	}
	s.prevCurrentLap = currentLap
	return completed, lapNum, lapTime
}

func (s *SessionManager) UpdateRaceTime(t float64) {
	s.lastRaceTime = t
	if t > s.peakRaceTime {
		s.peakRaceTime = t
	}
}

func (s *SessionManager) NoteClose(wallMs int64) {
	s.closedId = s.ActiveId
	s.closedWallMs = wallMs
	s.lastRaceTimeAtClose = s.peakRaceTime
}

func (s *SessionManager) CheckReopen(newRaceTime float64, nowWallMs int64) *int64 {
	if s.closedId == nil {
		return nil
	}
	gapMs := nowWallMs - s.closedWallMs
	if gapMs < 0 {
		gapMs = 0
	}
	if gapMs < rewindWindowMs && newRaceTime > rewindMinRaceTime && newRaceTime < s.lastRaceTimeAtClose {
		id := s.closedId
		s.closedId = nil
		return id
	}
	return nil
}

func (s *SessionManager) OnRaceOnChange(wasRacing bool, isRacing bool, carOrdinal int32, carClass int32, carPi int32) (string, int32, int32, int32) {
	if !wasRacing && isRacing && s.AutoRecord {
		return "Open", carOrdinal, carClass, carPi
	}
	if wasRacing && !isRacing && s.ActiveId != nil {
		return "Close", 0, 0, 0
	}
	return "None", 0, 0, 0
}

func (s *SessionManager) BestForClose() float64 {
	if math.IsInf(s.bestLap, 1) {
		return -1.0
	}
	return s.bestLap
}

// Database Helpers
func openSession(db *sql.DB, startedAt int64, carOrdinal int32, carClass int32, carPi int32, userID *int64) int64 {
	var res sql.Result
	var err error
	if userID != nil {
		res, err = db.Exec("INSERT INTO sessions (started_at, car_ordinal, car_class, car_pi, user_id) VALUES (?, ?, ?, ?, ?)", startedAt, carOrdinal, carClass, carPi, *userID)
	} else {
		res, err = db.Exec("INSERT INTO sessions (started_at, car_ordinal, car_class, car_pi) VALUES (?, ?, ?, ?)", startedAt, carOrdinal, carClass, carPi)
	}
	if err != nil {
		fmt.Printf("Error inserting session: %v\n", err)
		return -1
	}
	id, _ := res.LastInsertId()
	return id
}

func closeSession(db *sql.DB, id int64, endedAt int64, bestLap float64) {
	db.Exec("UPDATE sessions SET ended_at=?, best_lap = CASE WHEN ? > 0.0 THEN ? ELSE best_lap END WHERE id=?", endedAt, bestLap, bestLap, id)
}

func updateSessionCarIfUnknown(db *sql.DB, id int64, carOrdinal int32, carClass int32, carPi int32) {
	db.Exec("UPDATE sessions SET car_ordinal=?, car_class=?, car_pi=? WHERE id=? AND car_ordinal=0", carOrdinal, carClass, carPi, id)
}

func reopenSession(db *sql.DB, id int64) {
	db.Exec("UPDATE sessions SET ended_at = NULL WHERE id=?", id)
}

func insertLap(db *sql.DB, sessionId int64, lapNumber int, lapTime float64) {
	db.Exec("INSERT INTO session_laps (session_id, lap_number, lap_time) VALUES (?, ?, ?) ON CONFLICT(session_id, lap_number) DO UPDATE SET lap_time=excluded.lap_time", sessionId, lapNumber, lapTime)
}

type SoloQueuedPacket struct {
	SessionID   int64
	TimestampMs uint32
	Data        []byte
}

var soloPacketQueue = make(chan *SoloQueuedPacket, 500000) // ~150MB RAM buffer

func insertPacket(db *sql.DB, sessionId int64, timestampMs uint32, data []byte) {
	select {
	case soloPacketQueue <- &SoloQueuedPacket{
		SessionID:   sessionId,
		TimestampMs: timestampMs,
		Data:        data,
	}:
	default:
		// Queue full, drop packet to avoid blocking UDP thread
	}
}

// Background Worker: High-Performance Database Queue Writer for Solo Sessions
func StartSoloDBWorker(dbConn *sql.DB) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var batch []*SoloQueuedPacket

	flush := func(records []*SoloQueuedPacket) {
		if len(records) == 0 {
			return
		}

		tx, err := dbConn.Begin()
		if err != nil {
			log.Printf("[Solo DB Batch] Transaction start failed: %v", err)
			return
		}

		stmt, err := tx.Prepare("INSERT INTO session_packets (session_id, timestamp_ms, data) VALUES (?, ?, ?)")
		if err != nil {
			log.Printf("[Solo DB Batch] Statement prepare failed: %v", err)
			tx.Rollback()
			return
		}
		defer stmt.Close()

		packetCounts := make(map[int64]int)
		for _, p := range records {
			_, err = stmt.Exec(p.SessionID, p.TimestampMs, p.Data)
			if err != nil {
				log.Printf("[Solo DB Batch] Insert execute failed: %v", err)
			} else {
				packetCounts[p.SessionID]++
			}
		}

		// Update counts efficiently
		countStmt, err := tx.Prepare("UPDATE sessions SET packet_count = packet_count + ? WHERE id=?")
		if err == nil {
			for sId, count := range packetCounts {
				countStmt.Exec(count, sId)
			}
			countStmt.Close()
		}

		if err := tx.Commit(); err != nil {
			log.Printf("[Solo DB Batch] Transaction commit failed: %v", err)
			tx.Rollback()
		}
	}

	for {
		select {
		case p := <-soloPacketQueue:
			batch = append(batch, p)
			if len(batch) >= 50000 { // Flush at 50k packets to utilize bulk I/O efficiently
				flush(batch)
				batch = make([]*SoloQueuedPacket, 0, 50000)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flush(batch)
				batch = make([]*SoloQueuedPacket, 0, 50000)
			}
		}
	}
}
