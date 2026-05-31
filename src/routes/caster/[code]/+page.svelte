<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { settings, loadSettings } from '$lib/stores/sessions';
  import type { TelemetryPacket } from '$lib/types';
  import MapPanel from '$lib/components/MapPanel.svelte';
  import { carName } from '$lib/car-name';

  const code = $page.params.code;

  let lobby = $state<any>(null);
  let ws = $state<WebSocket | null>(null);
  let telemetryData = $state<Record<number, TelemetryPacket>>({});
  let activeDrivers = $state<Record<number, boolean>>({});
  let timers: Record<number, any> = {};

  // Customizer state (for OBS caster config)
  let showLeaderboard = $state(true);
  let showFocusedHud = $state(true);
  let showMap = $state(false);
  let mapMode = $state<'all' | 'focused'>('all');
  let focusedSlot = $state<number>(1);
  let bgMode = $state<'transparent' | 'green' | 'blue'>('transparent');
  let showControls = $state(true); // Tiny config button triggers this

  const SLOT_COLORS = [
    '#3b82f6', '#ef4444', '#10b981', '#f59e0b',
    '#8b5cf6', '#06b6d4', '#ec4899', '#14b8a6',
    '#f43f5e', '#eab308', '#84cc16', '#a855f7'
  ];

  async function fetchLobby() {
    try {
      const res = await fetch(`/api/lobby/status?code=${code}`);
      if (!res.ok) throw new Error(await res.text());
      lobby = await res.json();
    } catch (err) {}
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
    // Read optional URL search parameters
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.get('leaderboard') === '0') showLeaderboard = false;
    if (urlParams.get('hud') === '0') showFocusedHud = false;
    if (urlParams.get('map') === '1') showMap = true;
    if (urlParams.get('mapmode') === 'focused') mapMode = 'focused';
    
    const focusParam = urlParams.get('focus');
    if (focusParam) focusedSlot = parseInt(focusParam);
    
    const bgParam = urlParams.get('bg');
    if (bgParam === 'green') bgMode = 'green';
    if (bgParam === 'blue') bgMode = 'blue';
    if (urlParams.get('control') === '0') showControls = false;

    await loadSettings();
    await fetchLobby();
    setupWebSocket();
  });

  onDestroy(() => {
    if (ws) ws.close();
    Object.values(timers).forEach(clearTimeout);
  });

  let s = $derived($settings);

  // Leaderboard Sorting (Race position or Best Lap)
  let sortedDrivers = $derived.by(() => {
    if (!lobby) return [];
    const list = lobby.slots.filter((s: any) => s.userId > 0);
    return list.sort((a: any, b: any) => {
      const pktA = telemetryData[a.slotNumber];
      const pktB = telemetryData[b.slotNumber];
      if (pktA && pktB && pktA.isRaceOn && pktB.isRaceOn) {
        if (pktA.racePosition !== pktB.racePosition) {
          return pktA.racePosition - pktB.racePosition;
        }
      }
      const bestA = pktA ? pktA.bestLap : 999999;
      const bestB = pktB ? pktB.bestLap : 999999;
      return (bestA > 0 ? bestA : 999999) - (bestB > 0 ? bestB : 999999);
    });
  });

  let focusedPkt = $derived(telemetryData[focusedSlot]);
  let focusedDriver = $derived(lobby ? lobby.slots.find((s: any) => s.slotNumber === focusedSlot) : null);

  // Map players: all or just the focused driver
  let mapPlayers = $derived.by(() => {
    if (!lobby) return [];
    const allSlots = lobby.slots.filter((slot: any) => slot.userId > 0 && telemetryData[slot.slotNumber]);
    const source = mapMode === 'focused'
      ? allSlots.filter((slot: any) => slot.slotNumber === focusedSlot)
      : allSlots;
    return source.map((slot: any) => ({
      slotNumber: slot.slotNumber,
      name: slot.username,
      color: SLOT_COLORS[slot.slotNumber - 1],
      packet: telemetryData[slot.slotNumber]
    }));
  });
</script>

<div class="overlay-container bg-{bgMode}">
  
  <!-- Floating Control Tool (Hidden when showControls is false) -->
  {#if showControls}
    <div class="floating-controls">
      <div class="ctrl-header">
        <span>OBS Caster HUD Overlay Config</span>
        <button class="hide-ctrl-btn" onclick={() => (showControls = false)} title="Hide overlay controller">✕</button>
      </div>

      <div class="ctrl-options">
        <label class="opt-label">
          <input type="checkbox" bind:checked={showLeaderboard} />
          Show Leaderboard
        </label>

        <label class="opt-label">
          <input type="checkbox" bind:checked={showFocusedHud} />
          Show Telemetry HUD
        </label>

        <div class="opt-select">
          <span>Focus Driver:</span>
          <select bind:value={focusedSlot}>
            {#if lobby}
              {#each lobby.slots as slot}
                {#if slot.userId > 0}
                  <option value={slot.slotNumber}>{slot.username} (Slot {slot.slotNumber})</option>
                {:else}
                  <option value={slot.slotNumber}>Slot {slot.slotNumber} (Empty)</option>
                {/if}
              {/each}
            {:else}
              <option value={1}>Slot 1</option>
            {/if}
          </select>
        </div>

        <div class="opt-select">
          <span>Chroma Key BG:</span>
          <select bind:value={bgMode}>
            <option value="transparent">Transparent (OBS Native)</option>
            <option value="green">Green Screen (#00FF00)</option>
            <option value="blue">Blue Screen (#0000FF)</option>
          </select>
        </div>

        <div class="ctrl-divider"></div>
        <label class="opt-label">
          <input type="checkbox" bind:checked={showMap} />
          Show Map Overlay
        </label>

        {#if showMap}
          <div class="opt-select">
            <span>Map Players:</span>
            <select bind:value={mapMode}>
              <option value="all">All Players</option>
              <option value="focused">Focused Only</option>
            </select>
          </div>
        {/if}
      </div>
      <p class="ctrl-tip">Tip: Copy this URL into OBS as a Browser Source. Hide this control bar by clicking the ✕.</p>
    </div>
  {:else}
    <!-- Small floating gear button to bring back controls if needed during preview -->
    <button class="open-ctrl-btn" onclick={() => (showControls = true)} title="Configure overlay layout">⚙</button>
  {/if}

  {#if lobby}
    <!-- Leaderboard Overlay View -->
    {#if showLeaderboard}
      <div class="leaderboard-overlay">
        <div class="obs-hdr">STANDINGS</div>
        {#each sortedDrivers as driver, idx}
          {@const pkt = telemetryData[driver.slotNumber]}
          <div class="obs-row" style="--slot-color: {SLOT_COLORS[driver.slotNumber - 1]}">
            <span class="obs-pos">{pkt && pkt.isRaceOn && pkt.racePosition > 0 ? pkt.racePosition : idx + 1}</span>
            <span class="obs-color-bar"></span>
            <div class="obs-driver-info">
              <span class="obs-driver-name">{driver.username}</span>
              {#if pkt && pkt.carOrdinal > 0}
                <span class="obs-car-name">{carName(pkt.carOrdinal)} (PI {pkt.carPi})</span>
              {/if}
            </div>
            
            <div class="obs-stats">
              {#if pkt && activeDrivers[driver.slotNumber]}
                <span class="obs-stat-speed">
                  {Math.round(pkt.speedMs * (s?.useMph ? 2.23694 : 3.6))}
                  <span class="small-u">{s?.useMph ? 'MPH' : 'KMH'}</span>
                </span>
                              {#if pkt.throttle > 0 || pkt.brake > 0 || pkt.handbrake > 0}
                  <div class="obs-mini-bars">
                    <div class="bar gas" style="width: {pkt.throttle / 2.55}%"></div>
                    <div class="bar brake" style="width: {pkt.brake / 2.55}%"></div>
                    {#if pkt.handbrake > 10}
                      <div class="bar hbk" style="width: {pkt.handbrake / 2.55}%"></div>
                    {/if}
                  </div>
                {/if}
              {:else}
                <span class="obs-offline">OFFLINE</span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}

    <!-- Focused Player Telemetry Overlay (Bottom Right) -->
    {#if showFocusedHud && focusedDriver}
      {#if focusedPkt && activeDrivers[focusedSlot]}
        <div class="telemetry-hud-overlay" style="border-left-color: {SLOT_COLORS[focusedSlot - 1]}">
          <div class="hud-driver-name" style="color: {SLOT_COLORS[focusedSlot - 1]}">
            {focusedDriver.username}
          </div>
          
          <div class="hud-main">
            <!-- Speed, gear, and unit -->
            <div class="hud-speedometer">
              <span class="val-speed">{Math.round(focusedPkt.speedMs * (s?.useMph ? 2.23694 : 3.6))}</span>
              <div class="sub-row">
                <span class="unit">{s?.useMph ? 'MPH' : 'KM/H'}</span>
                <span class="gear">{focusedPkt.gear === 0 ? 'R' : focusedPkt.gear === 11 ? 'N' : focusedPkt.gear}</span>
              </div>
            </div>

            <!-- Pedals bars -->
            <div class="hud-pedals">
              <div class="hud-pedal-container">
                <div class="bar-lbl">THR</div>
                <div class="bar-track"><div class="bar-fill throttle" style="height: {focusedPkt.throttle / 2.55}%"></div></div>
              </div>
              <div class="hud-pedal-container">
                <div class="bar-lbl">BRK</div>
                <div class="bar-track"><div class="bar-fill brake" style="height: {focusedPkt.brake / 2.55}%"></div></div>
              </div>
              <div class="hud-pedal-container">
                <div class="bar-lbl">HBK</div>
                <div class="bar-track"><div class="bar-fill handbrake" style="height: {focusedPkt.handbrake / 2.55}%"></div></div>
              </div>
            </div>

            <!-- RPM curve -->
            <div class="hud-rpm">
              <div class="rpm-val">{Math.round(focusedPkt.currentEngineRpm)} RPM</div>
              <div class="rpm-track">
                <div class="rpm-fill" style="width: {(focusedPkt.currentEngineRpm / (focusedPkt.engineMaxRpm || 8000)) * 100}%"></div>
              </div>
            </div>
          </div>
        </div>
      {/if}
    {/if}

    <!-- Map Overlay (Bottom Center) -->
    {#if showMap && s}
      <div class="map-overlay">
        <div class="map-overlay-hdr">
          <span class="map-overlay-title">🗺 TRACK MAP</span>
          <span class="map-overlay-mode">{mapMode === 'all' ? 'ALL PLAYERS' : `FOCUS: ${focusedDriver?.username ?? '—'}`}</span>
        </div>
        <div class="map-overlay-canvas">
          <MapPanel
            points={[]}
            drawLine={false}
            settings={s}
            multiplayers={mapPlayers}
            compact={true}
          />
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .overlay-container {
    position: relative;
    width: 100vw;
    height: 100vh;
    height: 100dvh;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
    color: #ffffff;
    overflow: hidden;
    pointer-events: none; /* Let clicks pass through to game screens unless hover controls */
  }

  /* Chroma Key Background Toggles */
  .bg-transparent {
    background: transparent !important;
  }
  .bg-green {
    background: #00ff00 !important;
  }
  .bg-blue {
    background: #0000ff !important;
  }

  /* Floating Controller Settings (for caster configure) */
  .floating-controls {
    position: absolute;
    top: 15px;
    right: 15px;
    z-index: 999;
    width: 320px;
    background: rgba(10, 15, 30, 0.95);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    padding: 1rem;
    box-shadow: 0 10px 30px rgba(0,0,0,0.6);
    pointer-events: auto; /* Re-enable clicks for controls */
  }
  .ctrl-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.85rem;
    font-weight: 700;
    color: #3b82f6;
    margin-bottom: 0.75rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    padding-bottom: 0.4rem;
  }
  .hide-ctrl-btn {
    background: none;
    border: none;
    color: #6b7280;
    font-weight: bold;
    cursor: pointer;
  }
  .hide-ctrl-btn:hover { color: #ffffff; }

  .ctrl-options {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .opt-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.82rem;
    cursor: pointer;
  }
  .opt-select {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.82rem;
  }
  .opt-select select {
    background: #1f2937;
    border: 1px solid #4b5563;
    border-radius: 4px;
    color: #ffffff;
    padding: 0.2rem 0.4rem;
    font-size: 0.8rem;
    outline: none;
  }
  .ctrl-tip {
    font-size: 0.65rem;
    color: #9ca3af;
    line-height: 1.3;
    margin-top: 0.8rem;
    border-top: 1px solid rgba(255, 255, 255, 0.05);
    padding-top: 0.5rem;
  }
  .ctrl-divider {
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    margin: 0.15rem 0;
  }

  /* Map Overlay */
  .map-overlay {
    position: absolute;
    bottom: 25px;
    left: 50%;
    transform: translateX(-50%);
    width: 340px;
    background: rgba(8, 12, 22, 0.88);
    border: 1px solid rgba(59, 130, 246, 0.35);
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.7), 0 0 0 1px rgba(59,130,246,0.1);
    backdrop-filter: blur(8px);
    pointer-events: auto;
  }
  .map-overlay-hdr {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.35rem 0.65rem;
    background: rgba(59, 130, 246, 0.12);
    border-bottom: 1px solid rgba(59, 130, 246, 0.2);
  }
  .map-overlay-title {
    font-size: 0.65rem;
    font-weight: 800;
    color: #93c5fd;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }
  .map-overlay-mode {
    font-size: 0.58rem;
    font-weight: 700;
    color: #6b7280;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .map-overlay-canvas {
    height: 280px;
    width: 100%;
  }

  .open-ctrl-btn {
    position: absolute;
    top: 10px;
    right: 10px;
    z-index: 998;
    background: rgba(10, 15, 30, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 4px;
    color: #9ca3af;
    width: 28px;
    height: 28px;
    cursor: pointer;
    font-size: 0.95rem;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: auto;
  }
  .open-ctrl-btn:hover {
    color: #ffffff;
    background: #1f2937;
  }

  /* Standings Overlay (F1 TV Style) */
  .leaderboard-overlay {
    position: absolute;
    top: 25px;
    left: 25px;
    width: 250px;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    filter: drop-shadow(0 4px 6px rgba(0, 0, 0, 0.6));
  }
  .obs-hdr {
    background: #ef4444;
    color: #ffffff;
    font-size: 0.72rem;
    font-weight: 900;
    letter-spacing: 0.1em;
    padding: 0.35rem 0.5rem;
    border-radius: 4px 4px 0 0;
    text-align: center;
  }
  .obs-row {
    display: flex;
    align-items: center;
    background: rgba(12, 17, 29, 0.92);
    border-radius: 0 4px 4px 0;
    height: 38px;
    padding: 0 0.5rem;
    position: relative;
  }
  .obs-pos {
    font-size: 0.78rem;
    font-weight: 800;
    color: #9ca3af;
    width: 15px;
    text-align: center;
  }
  .obs-color-bar {
    width: 3px;
    height: 20px;
    background: var(--slot-color);
    border-radius: 2px;
    margin: 0 0.4rem;
  }
  .obs-driver-info {
    display: flex;
    flex-direction: column;
    justify-content: center;
    flex: 1;
    min-width: 0;
  }
  .obs-driver-name {
    font-size: 0.8rem;
    font-weight: 700;
    color: #ffffff;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .obs-car-name {
    font-size: 0.55rem;
    font-weight: 600;
    color: #9ca3af;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .obs-stats {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .obs-stat-speed {
    font-size: 0.75rem;
    font-weight: 700;
  }
  .small-u {
    font-size: 0.55rem;
    color: #9ca3af;
    font-weight: 500;
  }
  .obs-offline {
    font-size: 0.65rem;
    color: #4b5563;
    font-weight: bold;
  }
  .obs-mini-bars {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 2px;
    display: flex;
  }
  .bar { height: 100%; }
  .bar.gas { background: #22c55e; }
  .bar.brake { background: #ef4444; }
  .bar.hbk { background: #f97316; }

  /* Telemetry Focused HUD Overlay (Bottom Right) */
  .telemetry-hud-overlay {
    position: absolute;
    bottom: 25px;
    right: 25px;
    width: 280px;
    background: rgba(12, 17, 29, 0.92);
    border-left: 5px solid #3b82f6;
    border-radius: 0 8px 8px 0;
    padding: 0.75rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.6));
  }
  .hud-driver-name {
    font-size: 0.95rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .hud-main {
    display: grid;
    grid-template-columns: 100px 50px 1fr;
    gap: 0.75rem;
    align-items: center;
  }

  .hud-speedometer {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
  }
  .val-speed {
    font-size: 2rem;
    font-weight: 900;
    line-height: 1;
  }
  .sub-row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .unit {
    font-size: 0.6rem;
    font-weight: 600;
    color: #9ca3af;
  }
  .gear {
    font-size: 0.9rem;
    font-weight: 900;
    color: #f59e0b;
  }

  .hud-pedals {
    display: flex;
    gap: 0.4rem;
    justify-content: center;
  }
  .hud-pedal-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.15rem;
  }
  .bar-lbl {
    font-size: 0.52rem;
    font-weight: 700;
    color: #9ca3af;
  }
  .bar-track {
    width: 8px;
    height: 36px;
    background: rgba(255, 255, 255, 0.06);
    border-radius: 4px;
    display: flex;
    flex-direction: column-reverse;
    overflow: hidden;
  }
  .bar-fill { width: 100%; border-radius: 4px; }
  .bar-fill.throttle { background: #22c55e; }
  .bar-fill.brake { background: #ef4444; }
  .bar-fill.handbrake { background: #f97316; }

  .hud-rpm {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .rpm-val {
    font-size: 0.68rem;
    font-weight: 700;
    color: #d1d5db;
    text-align: right;
  }
  .rpm-track {
    height: 4px;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 2px;
    overflow: hidden;
  }
  .rpm-fill {
    height: 100%;
    background: linear-gradient(90deg, #3b82f6, #a855f7);
    border-radius: 2px;
  }
</style>
