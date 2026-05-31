<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { checkAuth } from '$lib/stores/auth';
  import { settings, loadSettings } from '$lib/stores/sessions';
  import MapPanel from '$lib/components/MapPanel.svelte';
  import { carName } from '$lib/car-name';
  import type { TelemetryPacket } from '$lib/types';

  const code = $page.params.code;

  let lobby = $state<any>(null);
  let ws = $state<WebSocket | null>(null);
  let errorMsg = $state('');
  
  // Latest packet for each slot (1-indexed)
  let telemetryData = $state<Record<number, TelemetryPacket>>({});
  let activeDrivers = $state<Record<number, boolean>>({});
  let timers: Record<number, any> = {};
  
  let focusedSlot = $state<number | null>(null);

  const SLOT_COLORS = [
    '#3b82f6', // Slot 1: Blue
    '#ef4444', // Slot 2: Red
    '#10b981', // Slot 3: Green
    '#f59e0b', // Slot 4: Amber/Orange
    '#8b5cf6', // Slot 5: Violet
    '#06b6d4', // Slot 6: Cyan
    '#ec4899', // Slot 7: Pink
    '#14b8a6', // Slot 8: Teal
    '#f43f5e', // Slot 9: Rose
    '#eab308', // Slot 10: Yellow
    '#84cc16', // Slot 11: Lime
    '#a855f7'  // Slot 12: Purple
  ];

  async function fetchLobby() {
    try {
      const res = await fetch(`/api/lobby/status?code=${code}`);
      if (!res.ok) throw new Error(await res.text());
      lobby = await res.json();
      
      // Auto-focus on first connected player
      if (focusedSlot === null) {
        const firstActive = lobby.slots.find((s: any) => s.userId > 0);
        if (firstActive) focusedSlot = firstActive.slotNumber;
      }
    } catch (err: any) {
      errorMsg = err.message || 'Lobby not found';
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
          telemetryData[slot] = payload.data;
          activeDrivers[slot] = true;

          if (timers[slot]) clearTimeout(timers[slot]);
          timers[slot] = setTimeout(() => {
            activeDrivers[slot] = false;
          }, 1500);
        }
      } catch (err) {}
    };

    ws.onclose = () => {
      setTimeout(setupWebSocket, 3000);
    };
  }

  onMount(async () => {
    try {
      await checkAuth(); // Try to load user session if exists
    } catch {}
    await loadSettings();
    await fetchLobby();
    setupWebSocket();
  });

  onDestroy(() => {
    if (ws) ws.close();
    Object.values(timers).forEach(clearTimeout);
  });

  // Calculate live values
  let s = $derived($settings);
  
  // Format speed based on preference (MPH vs KM/H)
  function formatSpeed(speedMs: number): string {
    const factor = s?.useMph ? 2.23694 : 3.6;
    const unit = s?.useMph ? 'mph' : 'km/h';
    return `${Math.round(speedMs * factor)} ${unit}`;
  }

  // Map active players to format needed by MapPanel
  let mapPlayers = $derived.by(() => {
    if (!lobby) return [];
    return lobby.slots
      .filter((slot: any) => slot.userId > 0 && telemetryData[slot.slotNumber])
      .map((slot: any) => ({
        slotNumber: slot.slotNumber,
        name: slot.username,
        color: SLOT_COLORS[slot.slotNumber - 1],
        packet: telemetryData[slot.slotNumber]
      }));
  });

  // Sort drivers for the leaderboard
  let sortedSlots = $derived.by(() => {
    if (!lobby) return [];
    
    // Create copy of player slots
    const list = lobby.slots.filter((s: any) => s.userId > 0);
    
    return list.sort((a: any, b: any) => {
      const pktA = telemetryData[a.slotNumber];
      const pktB = telemetryData[b.slotNumber];
      
      // If race is active, sort by racePosition
      if (pktA && pktB && pktA.isRaceOn && pktB.isRaceOn) {
        if (pktA.racePosition !== pktB.racePosition) {
          return pktA.racePosition - pktB.racePosition;
        }
      }
      
      // Fallback: sort by best lap
      const bestA = pktA ? pktA.bestLap : 999999;
      const bestB = pktB ? pktB.bestLap : 999999;
      return (bestA > 0 ? bestA : 999999) - (bestB > 0 ? bestB : 999999);
    });
  });

  // Focused driver details
  let focusedPkt = $derived(focusedSlot ? telemetryData[focusedSlot] : null);
  let focusedDriver = $derived(lobby ? lobby.slots.find((s: any) => s.slotNumber === focusedSlot) : null);
</script>

<div class="spectator-container">
  <div class="glow-sphere sphere-1"></div>
  <div class="glow-sphere sphere-2"></div>

  {#if lobby}
    <!-- Top Nav -->
    <header class="navbar">
      <div class="nav-left">
        <a href="/lobby/{code}" class="back-link">← Lobby</a>
        <span class="nav-title">SPECTATING: <strong class="room-code">{code}</strong></span>
      </div>
      <div class="nav-right">
        {#if lobby.isRecording}
          <div class="rec-badge">
            <span class="rec-dot"></span>
            <span>RECORDING ON</span>
          </div>
        {:else}
          <div class="rec-badge off">PRACTICE SESSION</div>
        {/if}
      </div>
    </header>

    <main class="spectator-layout">
      <!-- Leaderboard Column -->
      <section class="leaderboard-panel">
        <h3>Live Leaderboard</h3>
        <div class="leaderboard-list">
          {#if sortedSlots.length === 0}
            <div class="empty-state">No players in this room yet.</div>
          {:else}
            {#each sortedSlots as player, idx}
              {@const pkt = telemetryData[player.slotNumber]}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                class="leaderboard-row"
                class:focused={focusedSlot === player.slotNumber}
                onclick={() => (focusedSlot = player.slotNumber)}
                style="--slot-color: ${SLOT_COLORS[player.slotNumber - 1]}"
              >
                <div class="pos-num">
                  {pkt && pkt.isRaceOn && pkt.racePosition > 0 ? `#${pkt.racePosition}` : `${idx + 1}`}
                </div>
                
                <div class="color-indicator"></div>

                <div class="driver-details">
                  <span class="driver-name">{player.username}</span>
                  <span class="car-info-lbl">
                    {#if pkt && pkt.carOrdinal > 0}
                      <span class="car-pill">{carName(pkt.carOrdinal)}</span> • Class {pkt.carClass} (PI {pkt.carPi})
                    {:else}
                      Waiting for telemetry...
                    {/if}
                  </span>
                </div>

                <div class="live-stats">
                  {#if pkt && activeDrivers[player.slotNumber]}
                    <span class="speed-txt">{formatSpeed(pkt.speedMs)}</span>
                    <span class="best-lap-txt">Best: {pkt.bestLap > 0 ? pkt.bestLap.toFixed(3) + 's' : '—'}</span>
                  {:else}
                    <span class="offline-txt">OFFLINE</span>
                  {/if}
                </div>

                <!-- Input Mini Bars -->
                {#if pkt && activeDrivers[player.slotNumber]}
                  <div class="mini-inputs">
                    <div class="mini-bar gas" style="width: {pkt.throttle / 2.55}%"></div>
                    <div class="mini-bar brake" style="width: {pkt.brake / 2.55}%"></div>
                    {#if pkt.handbrake > 10}
                      <div class="mini-bar hbk" style="width: {pkt.handbrake / 2.55}%"></div>
                    {/if}
                  </div>
                {/if}
              </div>
            {/each}
          {/if}
        </div>
      </section>

      <!-- Center & Map Column -->
      <section class="main-panel">
        <div class="map-container">
          {#if s}
            <MapPanel
              points={[]}
              drawLine={false}
              settings={s}
              multiplayers={mapPlayers}
            />
          {/if}
        </div>

        <!-- Focused Driver Telemetry HUD -->
        <div class="focused-hud-panel">
          {#if focusedDriver}
            <div class="hud-header">
              <span class="focused-name" style="color: {SLOT_COLORS[focusedDriver.slotNumber - 1]}">
                👤 {focusedDriver.username}
              </span>
              <span class="focused-slot-lbl">Slot {focusedDriver.slotNumber}</span>
            </div>

            {#if focusedPkt && activeDrivers[focusedDriver.slotNumber]}
              <div class="hud-dashboard">
                <!-- Speedometer -->
                <div class="hud-item speed-hud">
                  <div class="speed-num">{Math.round(focusedPkt.speedMs * (s?.useMph ? 2.23694 : 3.6))}</div>
                  <div class="speed-unit">{s?.useMph ? 'MPH' : 'KM/H'}</div>
                  <div class="gear-num">{focusedPkt.gear === 0 ? 'R' : focusedPkt.gear === 11 ? 'N' : focusedPkt.gear}</div>
                </div>

                <!-- Inputs and RPM -->
                <div class="hud-item bar-hud">
                  <div class="hud-label">Inputs & RPM</div>
                                  <div class="input-bar-container">
                    <span class="bar-lbl">Throttle</span>
                    <div class="bar-bg"><div class="bar-fill throttle" style="width: {focusedPkt.throttle / 2.55}%"></div></div>
                  </div>
                  <div class="input-bar-container">
                    <span class="bar-lbl">Brake</span>
                    <div class="bar-bg"><div class="bar-fill brake" style="width: {focusedPkt.brake / 2.55}%"></div></div>
                  </div>
                  <div class="input-bar-container">
                    <span class="bar-lbl">Handbrake</span>
                    <div class="bar-bg"><div class="bar-fill handbrake" style="width: {focusedPkt.handbrake / 2.55}%"></div></div>
                  </div>
                  <div class="input-bar-container">
                    <span class="bar-lbl">RPM: {Math.round(focusedPkt.currentEngineRpm)}</span>
                    <div class="bar-bg">
                      <div class="bar-fill rpm" style="width: {(focusedPkt.currentEngineRpm / (focusedPkt.engineMaxRpm || 8000)) * 100}%"></div>
                    </div>
                  </div>
                </div>

                <!-- Tires Temperatures -->
                <div class="hud-item tire-hud">
                  <div class="hud-label">Tire Temps (°C)</div>
                  <div class="tire-grid">
                    <div class="tire-box">
                      <span class="t-pos">FL</span>
                      <span class="t-val">{Math.round(focusedPkt.tireTempFl)}°</span>
                    </div>
                    <div class="tire-box">
                      <span class="t-pos">FR</span>
                      <span class="t-val">{Math.round(focusedPkt.tireTempFr)}°</span>
                    </div>
                    <div class="tire-box">
                      <span class="t-pos">RL</span>
                      <span class="t-val">{Math.round(focusedPkt.tireTempRl)}°</span>
                    </div>
                    <div class="tire-box">
                      <span class="t-pos">RR</span>
                      <span class="t-val">{Math.round(focusedPkt.tireTempRr)}°</span>
                    </div>
                  </div>
                </div>
              </div>
            {:else}
              <div class="hud-offline">Focused driver is offline or not sending telemetry data...</div>
            {/if}
          {:else}
            <div class="hud-offline">Select a driver from the leaderboard to inspect their live telemetry dashboard.</div>
          {/if}
        </div>
      </section>
    </main>
  {:else}
    <div class="loading-state">
      <div class="spinner"></div>
      <p>Connecting to Room Spectator Feed...</p>
    </div>
  {/if}
</div>

<style>
  .spectator-container {
    position: relative;
    width: 100vw;
    height: 100vh;
    height: 100dvh;
    background: #02050b;
    color: #f9fafb;
    overflow: hidden;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
    display: flex;
    flex-direction: column;
  }

  .glow-sphere {
    position: absolute;
    border-radius: 50%;
    filter: blur(140px);
    opacity: 0.1;
    z-index: 0;
    pointer-events: none;
  }
  .sphere-1 {
    top: -5%;
    left: 20%;
    width: 500px;
    height: 500px;
    background: #3b82f6;
  }
  .sphere-2 {
    bottom: -5%;
    right: 20%;
    width: 500px;
    height: 500px;
    background: #8b5cf6;
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
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

  .navbar {
    position: relative;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 1.5rem;
    height: 3rem;
    background: rgba(10, 15, 30, 0.8);
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    backdrop-filter: blur(10px);
    flex-shrink: 0;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 1.25rem;
  }
  .back-link {
    color: #9ca3af;
    text-decoration: none;
    font-size: 0.88rem;
    border: 1px solid rgba(255, 255, 255, 0.15);
    padding: 0.25rem 0.65rem;
    border-radius: 4px;
    transition: all 0.2s;
  }
  .back-link:hover {
    color: #ffffff;
    background: rgba(255, 255, 255, 0.05);
  }
  .nav-title {
    font-size: 0.95rem;
    color: #9ca3af;
    letter-spacing: 0.05em;
  }
  .room-code {
    color: #3b82f6;
    text-shadow: 0 0 10px rgba(59, 130, 246, 0.3);
  }

  .rec-badge {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 4px;
    padding: 0.25rem 0.6rem;
    font-size: 0.72rem;
    font-weight: 700;
    color: #fca5a5;
  }
  .rec-badge.off {
    background: rgba(255, 255, 255, 0.05);
    border-color: rgba(255, 255, 255, 0.1);
    color: #d1d5db;
  }
  .rec-dot {
    width: 6px;
    height: 6px;
    background: #ef4444;
    border-radius: 50%;
    box-shadow: 0 0 6px #ef4444;
    animation: flash 1s infinite alternate;
  }
  @keyframes flash { from { opacity: 0.6; } to { opacity: 1; } }

  .spectator-layout {
    position: relative;
    z-index: 10;
    flex: 1;
    display: grid;
    grid-template-columns: 320px 1fr;
    min-height: 0;
    overflow: hidden;
  }

  /* Leaderboard Panel */
  .leaderboard-panel {
    border-right: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(4, 8, 16, 0.4);
    display: flex;
    flex-direction: column;
    padding: 1.25rem;
    min-height: 0;
  }
  .leaderboard-panel h3 {
    font-size: 1.15rem;
    font-weight: 700;
    margin-bottom: 1rem;
    color: #ffffff;
  }
  .leaderboard-list {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    overflow-y: auto;
    padding-right: 0.25rem;
  }
  .empty-state {
    font-size: 0.85rem;
    color: #6b7280;
    text-align: center;
    margin-top: 2rem;
    font-style: italic;
  }

  .leaderboard-row {
    position: relative;
    display: flex;
    align-items: center;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 0.65rem;
    cursor: pointer;
    overflow: hidden;
    transition: all 0.2s ease;
  }
  .leaderboard-row:hover {
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.1);
  }
  .leaderboard-row.focused {
    background: rgba(59, 130, 246, 0.06);
    border-color: rgba(59, 130, 246, 0.3);
  }

  .pos-num {
    font-size: 0.9rem;
    font-weight: 700;
    color: #9ca3af;
    width: 25px;
    text-align: center;
  }
  .color-indicator {
    width: 4px;
    height: 24px;
    background: var(--slot-color);
    border-radius: 2px;
    margin: 0 0.5rem;
  }
  .driver-details {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .driver-name {
    font-size: 0.9rem;
    font-weight: 700;
    color: #ffffff;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .car-info-lbl {
    font-size: 0.7rem;
    color: #6b7280;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .live-stats {
    text-align: right;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }
  .speed-txt {
    font-size: 0.82rem;
    font-weight: 700;
    color: #f3f4f6;
  }
  .best-lap-txt {
    font-size: 0.68rem;
    color: #f59e0b;
  }
  .offline-txt {
    font-size: 0.72rem;
    font-weight: 700;
    color: #4b5563;
  }

  .mini-inputs {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 2px;
    display: flex;
    background: rgba(255, 255, 255, 0.05);
  }
  .mini-bar { height: 100%; }
  .mini-bar.gas { background: #22c55e; }
  .mini-bar.brake { background: #ef4444; }
  .mini-bar.hbk { background: #f97316; }

  /* Main Column */
  .main-panel {
    display: grid;
    grid-template-rows: 1fr 200px;
    min-height: 0;
    overflow: hidden;
  }

  .map-container {
    padding: 1rem;
    min-height: 0;
  }

  /* Focused Driver HUD */
  .focused-hud-panel {
    background: rgba(6, 12, 24, 0.9);
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    padding: 1rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .hud-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .focused-name {
    font-size: 1.15rem;
    font-weight: 800;
  }
  .focused-slot-lbl {
    font-size: 0.75rem;
    font-weight: 600;
    color: #9ca3af;
    background: rgba(255, 255, 255, 0.05);
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
  }

  .hud-offline {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100px;
    font-size: 0.85rem;
    color: #4b5563;
    font-style: italic;
  }

  .hud-dashboard {
    display: grid;
    grid-template-columns: 140px 1fr 220px;
    gap: 1.5rem;
    align-items: center;
  }

  /* Speedometer */
  .speed-hud {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 10px;
    padding: 0.5rem;
    position: relative;
  }
  .speed-num {
    font-size: 2.2rem;
    font-weight: 800;
    color: #ffffff;
    line-height: 1;
  }
  .speed-unit {
    font-size: 0.65rem;
    font-weight: 700;
    color: #9ca3af;
    letter-spacing: 0.05em;
  }
  .gear-num {
    position: absolute;
    bottom: 5px;
    right: 10px;
    font-size: 1.1rem;
    font-weight: 800;
    color: #f59e0b;
  }

  /* Bars */
  .bar-hud {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .hud-label {
    font-size: 0.72rem;
    font-weight: 700;
    color: #9ca3af;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.15rem;
  }
  .input-bar-container {
    display: grid;
    grid-template-columns: 85px 1fr;
    align-items: center;
    gap: 0.5rem;
  }
  .bar-lbl {
    font-size: 0.72rem;
    color: #d1d5db;
    font-weight: 600;
  }
  .bar-bg {
    height: 6px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 3px;
    overflow: hidden;
  }
  .bar-fill { height: 100%; border-radius: 3px; }
  .bar-fill.throttle { background: #22c55e; }
  .bar-fill.brake { background: #ef4444; }
  .bar-fill.handbrake { background: #f97316; }
  .bar-fill.rpm { background: linear-gradient(90deg, #3b82f6, #a855f7); }

  /* Tires */
  .tire-hud {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .tire-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.4rem;
  }
  .tire-box {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 6px;
    padding: 0.35rem 0.6rem;
  }
  .t-pos { font-size: 0.68rem; font-weight: 700; color: #6b7280; }
  .t-val { font-size: 0.78rem; font-weight: 700; color: #ffffff; }
</style>
