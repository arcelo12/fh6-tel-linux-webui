import express from 'express';
import { createServer } from 'http';
import { Server } from 'socket.io';
import cors from 'cors';
import dgram from 'dgram';
import fs from 'fs';
import path from 'path';
import { parse } from './parser.js';
import * as db from './db.js';
import { SessionManager } from './session.js';
import { loadSettings, saveSettings } from './settings.js';

const settings = loadSettings();
const database = db.open();
const sessionManager = new SessionManager(settings.autoRecord);

const app = express();
app.use(cors());
app.use(express.json());

// API Routes
app.get('/api/sessions', (req, res) => {
  res.json(db.listSessions(database));
});

app.get('/api/sessions/:id/packets', (req, res) => {
  const buffers = db.getSessionPackets(database, Number(req.params.id));
  const packets = buffers.map(b => {
    try { return parse(b); } catch (e) { return null; }
  }).filter(Boolean);
  res.json(packets);
});

app.get('/api/sessions/:id/laps', (req, res) => {
  res.json(db.getSessionLaps(database, Number(req.params.id)));
});

app.delete('/api/sessions/:id', (req, res) => {
  db.deleteSession(database, Number(req.params.id));
  res.json({ success: true });
});

app.delete('/api/sessions', (req, res) => {
  db.clearAllSessions(database);
  res.json({ success: true });
});

app.post('/api/sessions/:id/rename', (req, res) => {
  db.renameSession(database, Number(req.params.id), req.body.name);
  res.json({ success: true });
});

app.post('/api/sessions/:id/bookmark', (req, res) => {
  db.setSessionBookmark(database, Number(req.params.id), req.body.bookmarked);
  res.json({ success: true });
});

app.get('/api/settings', (req, res) => {
  res.json(loadSettings());
});

app.post('/api/settings', (req, res) => {
  saveSettings(req.body);
  sessionManager.setAutoRecord(req.body.autoRecord);
  res.json({ success: true });
});

// SvelteKit Handler (if built)
if (fs.existsSync(path.resolve('build/handler.js'))) {
  import(path.resolve('build/handler.js')).then(({ handler }) => {
    app.use(handler);
  }).catch(console.error);
}

const httpServer = createServer(app);
const io = new Server(httpServer, {
  cors: { origin: '*' }
});

io.on('connection', (socket) => {
  console.log('Client connected');
});

// UDP Server
const udpClient = dgram.createSocket('udp4');
udpClient.on('message', (msg) => {
  try {
    const packet = parse(msg);
    const wasRacing = sessionManager.activeSessionId() !== null;
    const isRacing = packet.isRaceOn;
    
    const action = sessionManager.onRaceOnChange(
      wasRacing, isRacing, packet.carOrdinal, packet.carClass, packet.carPi
    );
    
    if (action.type === 'Open') {
      const id = db.openSession(database, Date.now(), action.carOrdinal, action.carClass, action.carPi);
      sessionManager.beginNewSession();
      sessionManager.setActiveId(id);
    } else if (action.type === 'Close') {
      const id = sessionManager.activeSessionId();
      db.closeSession(database, id, Date.now(), sessionManager.bestForClose());
      sessionManager.noteClose(Date.now());
      sessionManager.setActiveId(null);
    }

    const currentId = sessionManager.activeSessionId();
    if (currentId !== null && packet.carOrdinal !== 0) {
      db.updateSessionCarIfUnknown(database, currentId, packet.carOrdinal, packet.carClass, packet.carPi);
    }

    if (currentId === null) {
      const reopenId = sessionManager.checkReopen(packet.currentRaceTime, Date.now());
      if (reopenId !== null) {
        db.reopenSession(database, reopenId);
        sessionManager.setActiveId(reopenId);
      }
    }

    const activeId = sessionManager.activeSessionId();
    if (activeId !== null) {
      sessionManager.updateRaceTime(packet.currentRaceTime);
      const completedLap = sessionManager.noteTick(packet.isRaceOn, packet.currentLap, packet.currentRaceTime);
      if (completedLap) {
        db.insertLap(database, activeId, completedLap.lapNumber, completedLap.lapTime);
      }
      db.insertPacket(database, activeId, packet.timestampMs, msg);
    }

    io.emit('telemetry_tick', packet);
  } catch (err) {
    // console.error('Error parsing packet', err);
  }
});

udpClient.on('listening', () => {
  const address = udpClient.address();
  console.log(`UDP Server listening on ${address.address}:${address.port}`);
});

udpClient.bind(settings.port);

const WEB_PORT = process.env.PORT || 5173;
httpServer.listen(WEB_PORT, () => {
  console.log(`Web server listening on port ${WEB_PORT}`);
});
