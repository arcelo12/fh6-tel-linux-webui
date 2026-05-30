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
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
    
    ws.onopen = () => {
      isConnected.set(true);
    };

    ws.onmessage = (event) => {
      try {
        const data: TelemetryPacket = JSON.parse(event.data);
        packet.set(data);
        lastPacketTime = Date.now();
        isConnected.set(true);
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
