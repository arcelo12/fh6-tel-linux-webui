import { writable, derived } from 'svelte/store';
import type { TelemetryPacket } from '$lib/types';

export const packet = writable<TelemetryPacket | null>(null);
export const isConnected = writable(false);

export interface ReplayState {
  active: boolean;
  packets: TelemetryPacket[];
  index: number;
  playing: boolean;
  speed: number;
  sessionId: number | null;
  label: string;
}

const emptyReplay: ReplayState = {
  active: false,
  packets: [],
  index: 0,
  playing: false,
  speed: 1,
  sessionId: null,
  label: '',
};

export const replay = writable<ReplayState>({ ...emptyReplay });

export function startReplay(
  sessionId: number,
  label: string,
  packets: TelemetryPacket[]
) {
  replay.set({
    active: true,
    packets,
    index: 0,
    playing: false,
    speed: 1,
    sessionId,
    label,
  });
}

export function exitReplay() {
  replay.set({ ...emptyReplay });
}

export interface PendingTelemetryState {
  type: 'telemetry_pending';
  port: number;
  carOrdinal: number;
  carClass: number;
  carPi: number;
  clientIp: string;
}

export const pendingTelemetry = writable<PendingTelemetryState | null>(null);

let _frozen: TelemetryPacket | null = null;
export const displayPacket = derived(
  [packet, replay],
  ([$p, $r]): TelemetryPacket | null => {
    if ($r.active && $r.packets.length > 0) {
      const i = Math.min(Math.max($r.index, 0), $r.packets.length - 1);
      return $r.packets[i];
    }
    if ($p !== null && $p.isRaceOn) {
      _frozen = $p;
      return $p;
    }
    return _frozen ?? $p;
  }
);

export const speedMph = derived(displayPacket, ($p) =>
  $p ? $p.speedMs * 2.23694 : 0
);

export const speedKph = derived(displayPacket, ($p) =>
  $p ? $p.speedMs * 3.6 : 0
);

export const rpmPercent = derived(displayPacket, ($p) => {
  if (!$p || $p.engineMaxRpm === 0) return 0;
  return ($p.currentEngineRpm / $p.engineMaxRpm) * 100;
});

let lastPacketTime = 0;
let connectionTimer: ReturnType<typeof setInterval> | null = null;
let ws: WebSocket | null = null;

export async function startTelemetryListener() {
  if (!ws || ws.readyState === WebSocket.CLOSED) {
    let wsUrl = '';
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('backend_node_url');
      if (saved && saved.trim()) {
        const clean = saved.trim().replace(/\/$/, '');
        wsUrl = clean.replace(/^http:/, 'ws:').replace(/^https:/, 'wss:') + '/ws';
      }
    }

    if (!wsUrl) {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      wsUrl = `${protocol}//${window.location.host}/ws`;
    }

    ws = new WebSocket(wsUrl);
    
    ws.onopen = () => {
      isConnected.set(true);
    };

    ws.onmessage = (event) => {
      try {
        const raw = JSON.parse(event.data);
        if (raw && raw.type === 'telemetry_live') {
          packet.set(raw.data);
          lastPacketTime = Date.now();
          isConnected.set(true);
        } else if (raw && raw.type === 'telemetry_pending') {
          pendingTelemetry.set(raw);
        } else if (raw && typeof raw === 'object' && 'speedMs' in raw) {
          // Fallback legacy raw packet format
          packet.set(raw);
          lastPacketTime = Date.now();
          isConnected.set(true);
        }
      } catch (err) {}
    };

    ws.onclose = () => {
      isConnected.set(false);
      ws = null;
    };
  }

  if (connectionTimer) clearInterval(connectionTimer);
  connectionTimer = setInterval(() => {
    if (Date.now() - lastPacketTime > 2000) {
      isConnected.set(false);
    }
  }, 1000);
}
