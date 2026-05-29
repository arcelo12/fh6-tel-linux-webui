import Database from 'better-sqlite3';
import fs from 'fs';
import path from 'path';
import os from 'os';

export function dbPath() {
  const localAppdata = process.env.LOCALAPPDATA || path.join(os.homedir(), '.local', 'share');
  return path.join(localAppdata, 'fh6-tel', 'sessions.db');
}

export function open() {
  const p = dbPath();
  fs.mkdirSync(path.dirname(p), { recursive: true });
  const db = new Database(p);
  db.pragma('journal_mode = WAL');
  db.pragma('synchronous = NORMAL');
  init(db);
  return db;
}

export function init(db) {
  db.exec(`
    CREATE TABLE IF NOT EXISTS sessions (
      id INTEGER PRIMARY KEY,
      started_at INTEGER NOT NULL,
      ended_at INTEGER,
      car_ordinal INTEGER NOT NULL DEFAULT 0,
      car_class INTEGER NOT NULL DEFAULT 0,
      car_pi INTEGER NOT NULL DEFAULT 0,
      best_lap REAL,
      packet_count INTEGER NOT NULL DEFAULT 0
    );
    CREATE TABLE IF NOT EXISTS session_packets (
      id INTEGER PRIMARY KEY,
      session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
      timestamp_ms INTEGER NOT NULL,
      data BLOB NOT NULL
    );
    CREATE INDEX IF NOT EXISTS idx_packets_session ON session_packets(session_id);
  `);
  migrate(db);
}

function migrate(db) {
  try { db.exec("ALTER TABLE sessions ADD COLUMN name TEXT"); } catch (e) {}
  try { db.exec("ALTER TABLE sessions ADD COLUMN bookmarked INTEGER NOT NULL DEFAULT 0"); } catch (e) {}
  
  db.exec(`
    CREATE TABLE IF NOT EXISTS session_laps (
      id INTEGER PRIMARY KEY,
      session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
      lap_number INTEGER NOT NULL,
      lap_time REAL NOT NULL,
      UNIQUE(session_id, lap_number)
    );
    CREATE INDEX IF NOT EXISTS idx_laps_session ON session_laps(session_id);
  `);
}

export function insertLap(db, sessionId, lapNumber, lapTime) {
  const stmt = db.prepare(`
    INSERT INTO session_laps (session_id, lap_number, lap_time) VALUES (?, ?, ?)
    ON CONFLICT(session_id, lap_number) DO UPDATE SET lap_time=excluded.lap_time
  `);
  stmt.run(sessionId, lapNumber, lapTime);
}

export function minLapTime(db, sessionId) {
  const row = db.prepare("SELECT MIN(lap_time) as min FROM session_laps WHERE session_id=?").get(sessionId);
  return row ? row.min : null;
}

export function getSessionLaps(db, sessionId) {
  const rows = db.prepare("SELECT lap_number, lap_time FROM session_laps WHERE session_id=? ORDER BY lap_number ASC").all(sessionId);
  return rows.map(r => ({ lapNumber: r.lap_number, lapTime: r.lap_time }));
}

export function openSession(db, startedAt, carOrdinal, carClass, carPi) {
  const stmt = db.prepare("INSERT INTO sessions (started_at, car_ordinal, car_class, car_pi) VALUES (?, ?, ?, ?)");
  const info = stmt.run(startedAt, carOrdinal, carClass, carPi);
  return info.lastInsertRowid;
}

export function updateSessionCarIfUnknown(db, id, carOrdinal, carClass, carPi) {
  const stmt = db.prepare("UPDATE sessions SET car_ordinal=?, car_class=?, car_pi=? WHERE id=? AND car_ordinal=0");
  stmt.run(carOrdinal, carClass, carPi, id);
}

export function reopenSession(db, id) {
  const stmt = db.prepare("UPDATE sessions SET ended_at = NULL WHERE id=?");
  stmt.run(id);
}

export function closeSession(db, id, endedAt, bestLap) {
  const stmt = db.prepare(`
    UPDATE sessions SET ended_at=?,
    best_lap = CASE WHEN ? > 0.0 THEN ? ELSE best_lap END
    WHERE id=?
  `);
  stmt.run(endedAt, bestLap, bestLap, id);
}

export function insertPacket(db, sessionId, timestampMs, data) {
  db.prepare("INSERT INTO session_packets (session_id, timestamp_ms, data) VALUES (?, ?, ?)").run(sessionId, timestampMs, data);
  db.prepare("UPDATE sessions SET packet_count = packet_count + 1 WHERE id=?").run(sessionId);
}

export function listSessions(db) {
  const rows = db.prepare(`
    SELECT id, started_at, ended_at, car_ordinal, car_class, car_pi, best_lap, packet_count, name, bookmarked
    FROM sessions ORDER BY bookmarked DESC, started_at DESC LIMIT 100
  `).all();
  return rows.map(r => ({
    id: r.id,
    startedAt: r.started_at,
    endedAt: r.ended_at,
    carOrdinal: r.car_ordinal,
    carClass: r.car_class,
    carPi: r.car_pi,
    bestLap: r.best_lap,
    packetCount: r.packet_count,
    name: r.name,
    bookmarked: r.bookmarked !== 0,
  }));
}

export function renameSession(db, id, name) {
  const trimmed = name ? name.trim() : null;
  const val = trimmed && trimmed.length > 0 ? trimmed : null;
  db.prepare("UPDATE sessions SET name=? WHERE id=?").run(val, id);
}

export function setSessionBookmark(db, id, bookmarked) {
  db.prepare("UPDATE sessions SET bookmarked=? WHERE id=?").run(bookmarked ? 1 : 0, id);
}

export function getSessionPackets(db, sessionId) {
  const rows = db.prepare("SELECT data FROM session_packets WHERE session_id=? ORDER BY timestamp_ms ASC").all(sessionId);
  return rows.map(r => r.data);
}

export function deleteSession(db, id) {
  db.prepare("DELETE FROM session_laps WHERE session_id=?").run(id);
  db.prepare("DELETE FROM session_packets WHERE session_id=?").run(id);
  db.prepare("DELETE FROM sessions WHERE id=?").run(id);
}

export function clearAllSessions(db) {
  db.prepare("DELETE FROM session_laps").run();
  db.prepare("DELETE FROM session_packets").run();
  db.prepare("DELETE FROM sessions").run();
}
