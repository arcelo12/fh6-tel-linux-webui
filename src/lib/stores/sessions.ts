import { writable } from 'svelte/store';
import type { SessionRow, TelemetryPacket, AppSettings, SessionLap } from '$lib/types';

export const sessions = writable<SessionRow[]>([]);
export const settings = writable<AppSettings | null>(null);

function getBackendUrl(): string {
  if (typeof window !== 'undefined') {
    const saved = localStorage.getItem('backend_node_url');
    if (saved && saved.trim()) {
      return saved.trim().replace(/\/$/, '');
    }
  }
  return '';
}

async function apiRequest<T>(path: string, method: string = 'GET', body?: any): Promise<T> {
  const opts: RequestInit = { method, headers: {} };
  if (body) {
    opts.headers = { 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }
  opts.credentials = 'include';
  const base = getBackendUrl();
  const url = base ? `${base}/api${path}` : `/api${path}`;
  const res = await fetch(url, opts);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function loadSessions() {
  const rows = await apiRequest<SessionRow[]>('/sessions');
  sessions.set(rows);
}

export async function loadSessionPackets(sessionId: number): Promise<TelemetryPacket[]> {
  return apiRequest<TelemetryPacket[]>(`/sessions/${sessionId}/packets`);
}

export async function loadSessionLaps(sessionId: number): Promise<SessionLap[]> {
  return apiRequest<SessionLap[]>(`/sessions/${sessionId}/laps`);
}

export async function deleteSession(sessionId: number) {
  await apiRequest(`/sessions/${sessionId}`, 'DELETE');
  await loadSessions();
}

export async function clearAllSessions() {
  await apiRequest('/sessions', 'DELETE');
  await loadSessions();
}

export async function renameSession(sessionId: number, name: string | null) {
  await apiRequest(`/sessions/${sessionId}/rename`, 'POST', { name });
  await loadSessions();
}

export async function setSessionBookmark(sessionId: number, bookmarked: boolean) {
  await apiRequest(`/sessions/${sessionId}/bookmark`, 'POST', { bookmarked });
  await loadSessions();
}

export async function loadSettings(): Promise<AppSettings> {
  const s = await apiRequest<AppSettings>('/settings');
  settings.set(s);
  return s;
}

export async function saveSettings(s: AppSettings) {
  await apiRequest('/settings', 'POST', s);
  settings.set(s);
}
