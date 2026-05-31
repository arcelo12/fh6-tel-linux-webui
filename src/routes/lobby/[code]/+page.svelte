<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { currentUser, checkAuth } from '$lib/stores/auth';
  import { carName } from '$lib/car-name';

  const code = $page.params.code;

  let lobby = $state<any>(null);
  let sessionName = $state('');
  let ws = $state<WebSocket | null>(null);
  let loading = $state(false);
  let errorMsg = $state('');
  
  // Join request notifications (host sees these)
  let pendingRequests = $state<Array<{requestId: string; userId: number; username: string; requestedAt: number}>>([]);
  let respondingTo = $state<string | null>(null);
  
  // Track telemetry packet counts locally to show flashing "data incoming" lights
  let activeTelemetry = $state<Record<number, boolean>>({});
  let telemetryTimers: Record<number, any> = {};

  async function fetchLobbyStatus() {
    try {
      const res = await fetch(`/api/lobby/status?code=${code}`);
      if (!res.ok) throw new Error(await res.text());
      lobby = await res.json();
    } catch (err: any) {
      errorMsg = err.message || 'Failed to fetch lobby details';
    }
  }

  function setupWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const socketUrl = `${protocol}//${window.location.host}/ws?room=${code}`;
    ws = new WebSocket(socketUrl);

    ws.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload.type === 'lobby_update') {
          lobby = payload.lobby;
        } else if (payload.type === 'telemetry') {
          const slot = payload.slotNumber;
          activeTelemetry[slot] = true;
          
          if (telemetryTimers[slot]) clearTimeout(telemetryTimers[slot]);
          telemetryTimers[slot] = setTimeout(() => {
            activeTelemetry[slot] = false;
          }, 1000);
        } else if (payload.type === 'join_request') {
          // Host receives join requests
          pendingRequests = [...pendingRequests, payload.request];
        }
      } catch (err) {}
    };

    ws.onclose = () => {
      // Reconnect after 3 seconds
      setTimeout(setupWebSocket, 3000);
    };
  }

  onMount(async () => {
    const user = await checkAuth();
    if (!user) {
      goto('/login');
      return;
    }
    await fetchLobbyStatus();
    setupWebSocket();
  });

  onDestroy(() => {
    if (ws) ws.close();
    Object.values(telemetryTimers).forEach(clearTimeout);
  });

  async function respondToRequest(requestId: string, approve: boolean) {
    respondingTo = requestId;
    try {
      await fetch('/api/lobby/respond-join', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ roomCode: code, requestId, approve })
      });
      // Remove from list
      pendingRequests = pendingRequests.filter(r => r.requestId !== requestId);
    } catch {}
    respondingTo = null;
  }

  async function joinSlot(slotNumber: number) {
    loading = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/lobby/join', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ roomCode: code, slotNumber })
      });
      if (!res.ok) throw new Error(await res.text());
      lobby = await res.json();
    } catch (err: any) {
      errorMsg = err.message || 'Failed to join slot';
    } finally {
      loading = false;
    }
  }

  async function leaveLobby() {
    loading = true;
    try {
      await fetch('/api/lobby/leave', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ roomCode: code })
      });
      goto('/');
    } catch (err) {}
  }

  async function startRecording() {
    if (!sessionName.trim()) {
      errorMsg = 'Please enter a name for the race session';
      return;
    }
    errorMsg = '';
    try {
      const res = await fetch('/api/lobby/start-record', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ roomCode: code, sessionName })
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      if (data.success) {
        lobby.isRecording = true;
        lobby.activeSessionId = data.sessionID;
      }
    } catch (err: any) {
      errorMsg = err.message || 'Failed to start recording';
    }
  }

  async function stopRecording() {
    try {
      const res = await fetch('/api/lobby/stop-record', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ roomCode: code })
      });
      if (!res.ok) throw new Error(await res.text());
      lobby.isRecording = false;
      lobby.activeSessionId = 0;
      sessionName = '';
    } catch (err: any) {
      errorMsg = err.message || 'Failed to stop recording';
    }
  }

  function copyPort(port: number) {
    navigator.clipboard.writeText(String(port));
    // Add micro feedback in future, standard copy is fine for now
  }
</script>

<svelte:head>
  <title>Lobby {code} - FH6 Telemetry</title>
</svelte:head>

<div class="lobby-container">
  <div class="glow-sphere sphere-1"></div>
  <div class="glow-sphere sphere-2"></div>

  {#if lobby}
    <div class="lobby-header">
      <div class="header-left">
        <button class="back-btn" onclick={leaveLobby}>← Exit Lobby</button>
        <span class="room-tag">Room Code: <strong class="code-txt">{code}</strong></span>
      </div>

      <div class="header-right">
        <button class="action-btn" onclick={() => {
          navigator.clipboard.writeText(`${window.location.origin}/spectator/${code}`);
          const btn = document.getElementById('copy-spec-btn');
          if (btn) {
            btn.innerText = '✓ Copied!';
            setTimeout(() => btn.innerText = '📋 Copy Public Link', 2000);
          }
        }} id="copy-spec-btn">📋 Copy Public Link</button>
        <a href="/spectator/{code}" class="action-btn spectator-btn">📺 Spectator View</a>
        <a href="/caster/{code}" class="action-btn caster-btn">🎙 Caster Overlay</a>
      </div>
    </div>

    <div class="lobby-grid">
      <!-- Left side: Slots Grid -->
      <div class="slots-section">
        <h3>Lobby Slots (Up to 12 Players)</h3>
        <div class="slots-grid">
          {#each lobby.slots as slot}
            <div class="slot-card" class:occupied={slot.userId > 0}>
              <div class="slot-hdr">
                <span class="slot-num">Slot {slot.slotNumber}</span>
                {#if slot.userId > 0}
                  <span class="telemetry-status" class:active={activeTelemetry[slot.slotNumber]}>
                    {activeTelemetry[slot.slotNumber] ? 'LIVE DATA' : 'CONNECTED'}
                  </span>
                {:else}
                  <span class="telemetry-status open">OPEN</span>
                {/if}
              </div>

              {#if slot.userId > 0}
                <div class="driver-info">
                  <div class="driver-name">👤 {slot.username}</div>
                  <div class="car-badge-row">
                    {#if slot.carOrdinal > 0}
                      <span class="car-pill" title="Car #{slot.carOrdinal}">{carName(slot.carOrdinal)}</span>
                      <span class="car-pill pi">PI {slot.carPi}</span>
                    {:else}
                      <span class="car-pill pending">Waiting for car telemetry...</span>
                    {/if}
                  </div>
                  <div class="port-info" onclick={() => copyPort(slot.port)} role="button" tabindex="0" onkeydown={(e) => e.key === 'Enter' && copyPort(slot.port)} title="Click to copy telemetry port">
                    <span>Telemetry Port: <code>{slot.port}</code></span>
                    <span class="copy-icon">📋</span>
                  </div>
                </div>
              {:else}
                <div class="empty-slot">
                  <button class="join-slot-btn" onclick={() => joinSlot(slot.slotNumber)} disabled={loading}>
                    Claim Slot {slot.slotNumber}
                  </button>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>

      <!-- Right side: Host Controls & Lobby Stats -->
      <div class="controls-section">
        <div class="panel-card info-card">
          <h4>Room Details</h4>
          <div class="info-row">
            <span class="info-lbl">Host:</span>
            <span class="info-val">{lobby.hostUsername}</span>
          </div>
        </div>

        <div class="panel-card record-card">
          <h4>Race Session Recording</h4>
          <p class="desc">Recording aggregates telemetry from all slots into a unified multiplayer log for side-by-side post-race analysis.</p>

          {#if errorMsg}
            <div class="error-msg">{errorMsg}</div>
          {/if}

          {#if lobby.hostId === $currentUser?.id}
            {#if lobby.isRecording}
              <div class="recording-indicator">
                <span class="rec-dot"></span>
                <span>Recording Session #{lobby.activeSessionId}...</span>
              </div>
              <button class="record-btn stop" onclick={stopRecording}>Stop Recording</button>
            {:else}
              <div class="record-input-group">
                <label for="session-name">Session Name</label>
                <input
                  type="text"
                  id="session-name"
                  bind:value={sessionName}
                  placeholder="e.g. Suzuka GP - Race 1"
                />
              </div>
              <button class="record-btn start" onclick={startRecording}>Start Session Recording</button>
            {/if}
          {:else}
            <!-- Player/Spectator recording state only -->
            {#if lobby.isRecording}
              <div class="recording-indicator">
                <span class="rec-dot"></span>
                <span>Session is being recorded by Host...</span>
              </div>
            {:else}
              <div class="waiting-recording">Waiting for Host to start recording...</div>
            {/if}
          {/if}
        </div>

        <!-- Join Request Notifications (Host only) -->
        {#if lobby.hostId === $currentUser?.id && pendingRequests.length > 0}
          <div class="panel-card join-requests-card">
            <h4>🔔 Join Requests <span class="req-count">{pendingRequests.length}</span></h4>
            <div class="requests-list">
              {#each pendingRequests as req (req.requestId)}
                <div class="request-row">
                  <div class="req-info">
                    <span class="req-username">👤 {req.username}</span>
                    <span class="req-time">wants to join</span>
                  </div>
                  <div class="req-actions">
                    <button
                      class="req-btn approve"
                      onclick={() => respondToRequest(req.requestId, true)}
                      disabled={respondingTo === req.requestId}
                    >✓ Approve</button>
                    <button
                      class="req-btn reject"
                      onclick={() => respondToRequest(req.requestId, false)}
                      disabled={respondingTo === req.requestId}
                    >✕ Reject</button>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <div class="panel-card setup-instructions">
          <h4>Forza In-Game Setup Instructions</h4>
          <ol>
            <li>Go to <strong>Settings → HUD and Gameplay</strong> in Forza.</li>
            <li>Scroll down to the <strong>DATA OUT</strong> section.</li>
            <li>Turn <strong>Data Out</strong> to <strong>ON</strong>.</li>
            <li>Set <strong>Data Out IP Address</strong> to your server's IP address.</li>
            <li>Set <strong>Data Out IP Port</strong> to your assigned slot's <strong>Telemetry Port</strong> shown on the left.</li>
            <li>Set <strong>Data Out Format</strong> to <strong>Car Dash</strong>.</li>
          </ol>
        </div>
      </div>
    </div>
  {:else}
    <div class="loading-state">
      <div class="spinner"></div>
      <p>Loading lobby details...</p>
    </div>
  {/if}
</div>

<style>
  .lobby-container {
    position: relative;
    width: 100vw;
    min-height: 100vh;
    min-height: 100dvh;
    background: #030712;
    color: #f9fafb;
    overflow-x: hidden;
    overflow-y: auto;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
    padding: 1.5rem 2.5rem;
  }

  .glow-sphere {
    position: absolute;
    border-radius: 50%;
    filter: blur(140px);
    opacity: 0.12;
    z-index: 0;
    pointer-events: none;
  }
  .sphere-1 {
    top: 5%;
    left: 10%;
    width: 450px;
    height: 450px;
    background: #3b82f6;
  }
  .sphere-2 {
    bottom: 10%;
    right: 15%;
    width: 500px;
    height: 500px;
    background: #8b5cf6;
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-height: 80vh;
    z-index: 10;
  }
  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid rgba(255, 255, 255, 0.1);
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-bottom: 1rem;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .lobby-header {
    position: relative;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 2rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    padding-bottom: 1.25rem;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  .back-btn {
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 6px;
    color: #d1d5db;
    padding: 0.4rem 0.8rem;
    font-size: 0.85rem;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .back-btn:hover {
    background: rgba(255, 255, 255, 0.05);
    color: #ffffff;
  }

  .room-tag {
    font-size: 1.15rem;
    color: #9ca3af;
  }
  .code-txt {
    color: #3b82f6;
    font-size: 1.35rem;
    letter-spacing: 0.05em;
    text-shadow: 0 0 10px rgba(59, 130, 246, 0.3);
  }

  .header-right {
    display: flex;
    gap: 0.75rem;
  }

  .action-btn {
    display: inline-block;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.88rem;
    font-weight: 600;
    text-decoration: none;
    transition: all 0.2s ease;
  }
  .spectator-btn {
    background: rgba(59, 130, 246, 0.1);
    border: 1px solid rgba(59, 130, 246, 0.3);
    color: #60a5fa;
  }
  .spectator-btn:hover {
    background: #3b82f6;
    color: #ffffff;
  }
  .caster-btn {
    background: rgba(139, 92, 246, 0.1);
    border: 1px solid rgba(139, 92, 246, 0.3);
    color: #a78bfa;
  }
  .caster-btn:hover {
    background: #8b5cf6;
    color: #ffffff;
  }

  .lobby-grid {
    position: relative;
    z-index: 10;
    display: grid;
    grid-template-columns: 1fr 340px;
    gap: 2rem;
  }

  @media (max-width: 1024px) {
    .lobby-grid {
      grid-template-columns: 1fr;
    }
  }

  h3, h4 {
    font-size: 1.2rem;
    font-weight: 700;
    margin-bottom: 1.25rem;
    color: #f3f4f6;
  }

  .slots-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 1.25rem;
  }

  .slot-card {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 12px;
    padding: 1.25rem;
    transition: all 0.25s ease;
    backdrop-filter: blur(10px);
  }
  .slot-card.occupied {
    background: rgba(59, 130, 246, 0.03);
    border-color: rgba(59, 130, 246, 0.2);
  }

  .slot-hdr {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }
  .slot-num {
    font-size: 0.85rem;
    font-weight: 600;
    color: #9ca3af;
  }
  .telemetry-status {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    background: #22c55e;
    color: #ffffff;
  }
  .telemetry-status.active {
    background: #22c55e;
    animation: flash 1s infinite alternate;
  }
  @keyframes flash {
    0% { opacity: 0.7; }
    100% { opacity: 1; box-shadow: 0 0 10px #22c55e; }
  }
  .telemetry-status.open {
    background: rgba(255, 255, 255, 0.1);
    color: #d1d5db;
  }

  .driver-info {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .driver-name {
    font-size: 1.1rem;
    font-weight: 700;
    color: #ffffff;
  }
  .car-badge-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }
  .car-pill {
    font-size: 0.72rem;
    background: rgba(255, 255, 255, 0.05);
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    color: #d1d5db;
  }
  .car-pill.pi {
    background: rgba(59, 130, 246, 0.2);
    color: #60a5fa;
  }
  .car-pill.pending {
    background: transparent;
    color: #6b7280;
    font-style: italic;
    padding: 0;
  }

  .port-info {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.75rem;
    background: rgba(0, 0, 0, 0.2);
    border: 1px dashed rgba(255, 255, 255, 0.1);
    border-radius: 6px;
    padding: 0.4rem 0.65rem;
    cursor: pointer;
    margin-top: 0.5rem;
    transition: all 0.2s ease;
  }
  .port-info:hover {
    background: rgba(59, 130, 246, 0.05);
    border-color: #3b82f6;
  }
  .port-info code {
    color: #f59e0b;
    font-weight: bold;
  }
  .copy-icon {
    font-size: 0.7rem;
    opacity: 0.6;
  }

  .empty-slot {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 80px;
  }
  .join-slot-btn {
    background: transparent;
    border: 1px dashed rgba(255, 255, 255, 0.2);
    border-radius: 8px;
    color: #9ca3af;
    padding: 0.5rem 1.25rem;
    font-size: 0.85rem;
    cursor: pointer;
    width: 100%;
    transition: all 0.2s ease;
  }
  .join-slot-btn:hover:not(:disabled) {
    background: rgba(59, 130, 246, 0.05);
    border-color: #3b82f6;
    color: #60a5fa;
  }
  .join-slot-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Right Side Panel Cards */
  .panel-card {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 12px;
    padding: 1.25rem;
    margin-bottom: 1.5rem;
    backdrop-filter: blur(10px);
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    margin-bottom: 0.5rem;
    font-size: 0.88rem;
  }
  .info-lbl { color: #9ca3af; }
  .info-val { font-weight: 600; color: #ffffff; }

  .desc {
    font-size: 0.82rem;
    color: #9ca3af;
    line-height: 1.4;
    margin-bottom: 1.25rem;
  }

  .error-msg {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 6px;
    color: #fca5a5;
    font-size: 0.8rem;
    padding: 0.5rem;
    margin-bottom: 1rem;
  }

  .record-input-group {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin-bottom: 1.25rem;
  }
  .record-input-group label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #9ca3af;
    text-transform: uppercase;
  }
  .record-input-group input {
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 6px;
    padding: 0.5rem 0.75rem;
    color: #ffffff;
    font-size: 0.9rem;
  }
  .record-input-group input:focus {
    outline: none;
    border-color: #3b82f6;
  }

  .recording-indicator {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: 6px;
    padding: 0.6rem 0.75rem;
    color: #ef4444;
    font-size: 0.85rem;
    font-weight: 600;
    margin-bottom: 1.25rem;
  }
  .rec-dot {
    width: 8px;
    height: 8px;
    background: #ef4444;
    border-radius: 50%;
    box-shadow: 0 0 8px #ef4444;
    animation: flash 1s infinite alternate;
  }

  .waiting-recording, .waiting-recording {
    font-size: 0.85rem;
    color: #6b7280;
    font-style: italic;
    text-align: center;
    padding: 0.5rem;
  }

  .record-btn {
    width: 100%;
    padding: 0.65rem;
    border: none;
    border-radius: 6px;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .record-btn.start {
    background: #ef4444;
    color: #ffffff;
    box-shadow: 0 4px 10px rgba(239, 68, 68, 0.2);
  }
  .record-btn.start:hover {
    filter: brightness(1.1);
    transform: translateY(-1px);
  }
  .record-btn.stop {
    background: #4b5563;
    color: #ffffff;
  }
  .record-btn.stop:hover {
    background: #374151;
  }

  .setup-instructions ol {
    margin-left: 1rem;
    font-size: 0.82rem;
    color: #9ca3af;
    line-height: 1.6;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .setup-instructions ol strong {
    color: #e5e7eb;
  }

  /* Join Request Notifications */
  .join-requests-card {
    border-color: rgba(251, 191, 36, 0.3) !important;
    background: rgba(251, 191, 36, 0.03) !important;
    animation: glow-amber 2s infinite alternate;
  }
  @keyframes glow-amber {
    from { box-shadow: 0 0 0 rgba(251,191,36,0); }
    to { box-shadow: 0 0 12px rgba(251,191,36,0.15); }
  }
  .join-requests-card h4 { color: #fbbf24 !important; }
  .req-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: #f59e0b;
    color: #000;
    font-size: 0.7rem;
    font-weight: 800;
    border-radius: 99px;
    min-width: 18px;
    height: 18px;
    padding: 0 5px;
    margin-left: 6px;
    vertical-align: middle;
  }
  .requests-list { display: flex; flex-direction: column; gap: 0.5rem; margin-top: 0.5rem; }
  .request-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.5rem 0.6rem;
    background: rgba(0,0,0,0.2);
    border-radius: 6px;
    border: 1px solid rgba(255,255,255,0.06);
  }
  .req-info { display: flex; flex-direction: column; gap: 0.1rem; }
  .req-username { font-size: 0.88rem; font-weight: 700; color: #e5e7eb; }
  .req-time { font-size: 0.72rem; color: #6b7280; }
  .req-actions { display: flex; gap: 0.4rem; flex-shrink: 0; }
  .req-btn {
    padding: 0.3rem 0.65rem;
    border: none;
    border-radius: 5px;
    font-size: 0.78rem;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s, transform 0.1s;
  }
  .req-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .req-btn:not(:disabled):hover { opacity: 0.85; transform: translateY(-1px); }
  .req-btn.approve { background: #16a34a; color: #fff; }
  .req-btn.reject { background: #374151; color: #9ca3af; border: 1px solid #4b5563; }
  .req-btn.reject:not(:disabled):hover { background: #dc2626; color: #fff; border-color: #dc2626; }
</style>
