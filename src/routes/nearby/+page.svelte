<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { currentUser, checkAuth } from '$lib/stores/auth';

  interface RoomSummary {
    code: string;
    hostUsername: string;
    playerCount: number;
    maxSlots: number;
    isRecording: boolean;
  }

  let rooms = $state<RoomSummary[]>([]);
  let loading = $state(true);
  let me = $state<any>(null);

  // Per-room request state
  let requestStates = $state<Record<string, 'idle' | 'pending' | 'approved' | 'rejected' | 'loading'>>({});
  let requestIds = $state<Record<string, string>>({});

  let ws = $state<WebSocket | null>(null);
  let refreshInterval: any;

  async function fetchRooms() {
    try {
      const res = await fetch('/api/lobby/list', { credentials: 'include' });
      if (res.ok) rooms = await res.json();
    } catch {}
    loading = false;
  }

  async function requestJoin(code: string) {
    requestStates[code] = 'loading';
    try {
      const res = await fetch('/api/lobby/request-join', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ roomCode: code })
      });
      const data = await res.json();
      if (data.error) {
        requestStates[code] = 'idle';
        alert(data.error);
      } else {
        requestIds[code] = data.requestId;
        requestStates[code] = 'pending';
      }
    } catch {
      requestStates[code] = 'idle';
    }
  }

  function setupWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws?room=`);

    ws.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload.type === 'join_response') {
          const { roomCode, approved, slotNumber } = payload;
          if (approved) {
            requestStates[roomCode] = 'approved';
            // Navigate to lobby after brief delay
            setTimeout(() => goto(`/lobby/${roomCode}`), 1200);
          } else {
            requestStates[roomCode] = 'rejected';
            setTimeout(() => { requestStates[roomCode] = 'idle'; }, 3000);
          }
        }
      } catch {}
    };

    ws.onclose = () => setTimeout(setupWebSocket, 3000);
  }

  onMount(async () => {
    me = await checkAuth();
    if (!me) { goto('/login'); return; }
    await fetchRooms();
    setupWebSocket();
    refreshInterval = setInterval(fetchRooms, 8000);
  });

  onDestroy(() => {
    ws?.close();
    clearInterval(refreshInterval);
  });
</script>

<svelte:head>
  <title>Nearby Rooms – FH6 Telemetry</title>
</svelte:head>

<div class="page">
  <div class="header">
    <a href="/" class="back-btn">← Back</a>
    <div class="title-row">
      <h1>🌐 Nearby Rooms</h1>
      <button class="refresh-btn" onclick={fetchRooms}>↻ Refresh</button>
    </div>
    <p class="subtitle">Browse active public lobbies. Send a join request — the host must approve.</p>
  </div>

  <div class="content">
    {#if loading}
      <div class="empty-state">
        <div class="spinner"></div>
        <span>Scanning for rooms...</span>
      </div>
    {:else if rooms.length === 0}
      <div class="empty-state">
        <div class="empty-icon">📡</div>
        <p>No active rooms found</p>
        <span>Create a lobby from the main page or check back later.</span>
      </div>
    {:else}
      <div class="rooms-grid">
        {#each rooms as room}
          {@const state = requestStates[room.code] ?? 'idle'}
          <div class="room-card" class:recording={room.isRecording}>
            <!-- Header -->
            <div class="room-header">
              <div class="room-code">{room.code}</div>
              {#if room.isRecording}
                <div class="rec-badge">⏺ REC</div>
              {/if}
            </div>

            <!-- Host Info -->
            <div class="room-host">
              <span class="host-icon">👑</span>
              <span class="host-name">{room.hostUsername}</span>
            </div>

            <!-- Player Count -->
            <div class="room-slots">
              <div class="slots-bar">
                {#each Array.from({ length: room.maxSlots }) as _, i}
                  <div class="slot-pip" class:filled={i < room.playerCount}></div>
                {/each}
              </div>
              <span class="slots-label">{room.playerCount}/{room.maxSlots} players</span>
            </div>

            <!-- Action Button -->
            <div class="room-action">
              {#if state === 'idle'}
                <button
                  class="join-btn"
                  onclick={() => requestJoin(room.code)}
                  disabled={room.playerCount >= room.maxSlots}
                >
                  {room.playerCount >= room.maxSlots ? '🔒 Full' : '📨 Request to Join'}
                </button>
              {:else if state === 'loading'}
                <button class="join-btn pending" disabled>Sending...</button>
              {:else if state === 'pending'}
                <div class="status-badge pending">
                  <div class="pulse-dot"></div>
                  Waiting for approval...
                </div>
              {:else if state === 'approved'}
                <div class="status-badge approved">
                  ✅ Approved! Joining...
                </div>
              {:else if state === 'rejected'}
                <div class="status-badge rejected">
                  ❌ Request declined
                </div>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .page {
    min-height: 100vh;
    background: var(--bg-body, #030712);
    color: #e5e7eb;
    font-family: 'Inter', system-ui, sans-serif;
    padding: 0 0 4rem;
  }

  .header {
    background: linear-gradient(135deg, rgba(59,130,246,0.08), rgba(139,92,246,0.05));
    border-bottom: 1px solid rgba(59,130,246,0.15);
    padding: 1.5rem 2rem 1.25rem;
  }

  .back-btn {
    font-size: 0.82rem;
    color: #6b7280;
    text-decoration: none;
    letter-spacing: 0.04em;
    transition: color 0.15s;
  }
  .back-btn:hover { color: #93c5fd; }

  .title-row {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-top: 0.5rem;
  }

  h1 {
    font-size: 1.6rem;
    font-weight: 800;
    color: #f9fafb;
    margin: 0;
    letter-spacing: -0.02em;
  }

  .refresh-btn {
    background: rgba(59,130,246,0.12);
    border: 1px solid rgba(59,130,246,0.25);
    color: #93c5fd;
    padding: 0.3rem 0.75rem;
    border-radius: 6px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s;
  }
  .refresh-btn:hover { background: rgba(59,130,246,0.22); }

  .subtitle {
    color: #6b7280;
    font-size: 0.85rem;
    margin: 0.4rem 0 0;
  }

  .content {
    max-width: 900px;
    margin: 2rem auto;
    padding: 0 1.5rem;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
    padding: 4rem 2rem;
    color: #4b5563;
    text-align: center;
  }
  .empty-icon { font-size: 3rem; }
  .empty-state p { font-size: 1.1rem; font-weight: 700; color: #6b7280; margin: 0; }
  .empty-state span { font-size: 0.85rem; }

  .spinner {
    width: 36px; height: 36px;
    border: 3px solid rgba(59,130,246,0.2);
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .rooms-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 1rem;
  }

  .room-card {
    background: rgba(13, 20, 32, 0.9);
    border: 1px solid rgba(255,255,255,0.07);
    border-radius: 12px;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    transition: border-color 0.2s, box-shadow 0.2s;
  }
  .room-card:hover {
    border-color: rgba(59,130,246,0.3);
    box-shadow: 0 4px 20px rgba(59,130,246,0.08);
  }
  .room-card.recording {
    border-color: rgba(239,68,68,0.3);
  }
  .room-card.recording:hover {
    border-color: rgba(239,68,68,0.5);
    box-shadow: 0 4px 20px rgba(239,68,68,0.08);
  }

  .room-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .room-code {
    font-family: 'JetBrains Mono', 'Courier New', monospace;
    font-size: 1.1rem;
    font-weight: 800;
    color: #93c5fd;
    letter-spacing: 0.12em;
  }
  .rec-badge {
    font-size: 0.65rem;
    font-weight: 800;
    color: #ef4444;
    background: rgba(239,68,68,0.12);
    border: 1px solid rgba(239,68,68,0.3);
    border-radius: 4px;
    padding: 0.15rem 0.45rem;
    letter-spacing: 0.08em;
    animation: blink 1.2s infinite;
  }
  @keyframes blink { 0%,100% { opacity: 1; } 50% { opacity: 0.5; } }

  .room-host {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .host-icon { font-size: 0.9rem; }
  .host-name {
    font-size: 0.95rem;
    font-weight: 700;
    color: #d1d5db;
  }

  .room-slots { display: flex; flex-direction: column; gap: 0.35rem; }
  .slots-bar {
    display: flex;
    gap: 3px;
    flex-wrap: wrap;
  }
  .slot-pip {
    width: 14px; height: 6px;
    border-radius: 3px;
    background: rgba(255,255,255,0.08);
    border: 1px solid rgba(255,255,255,0.05);
    transition: background 0.2s;
  }
  .slot-pip.filled { background: #3b82f6; border-color: #2563eb; }
  .slots-label { font-size: 0.75rem; color: #6b7280; font-weight: 600; }

  .room-action { margin-top: 0.25rem; }

  .join-btn {
    width: 100%;
    padding: 0.6rem;
    background: linear-gradient(135deg, #3b82f6, #6366f1);
    border: none;
    border-radius: 8px;
    color: #fff;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s, transform 0.1s;
    letter-spacing: 0.02em;
  }
  .join-btn:hover:not(:disabled) { opacity: 0.9; transform: translateY(-1px); }
  .join-btn:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }
  .join-btn.pending { background: rgba(59,130,246,0.3); }

  .status-badge {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 0.55rem;
    border-radius: 8px;
    font-size: 0.82rem;
    font-weight: 700;
    letter-spacing: 0.02em;
  }
  .status-badge.pending {
    background: rgba(251,191,36,0.1);
    border: 1px solid rgba(251,191,36,0.25);
    color: #fbbf24;
  }
  .status-badge.approved {
    background: rgba(34,197,94,0.12);
    border: 1px solid rgba(34,197,94,0.3);
    color: #4ade80;
  }
  .status-badge.rejected {
    background: rgba(239,68,68,0.1);
    border: 1px solid rgba(239,68,68,0.25);
    color: #f87171;
  }

  .pulse-dot {
    width: 8px; height: 8px;
    border-radius: 50%;
    background: #fbbf24;
    animation: pulse 1.2s infinite;
  }
  @keyframes pulse {
    0%,100% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.4); opacity: 0.6; }
  }
</style>
