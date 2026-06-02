<script lang="ts">
  import { onMount } from 'svelte';
  import { currentUser } from '$lib/stores/auth';
  import { fetchConfig } from '$lib/stores/config';
  import { goto } from '$app/navigation';

  let configLoaded = $state(false);
  let showNodeModal = $state(false);
  let showLobbyInput = $state(false);
  let showSpectatorInput = $state(false);
  
  // Custom Node IP state (initialized to default for clean SSR hydration)
  let backendIpInput = $state('');
  let currentBackendNode = $state('Default Host');
  let connectionTestMsg = $state('');
  let connectionTestSuccess = $state<boolean | null>(null);

  // Input bindings
  let lobbyCodeInput = $state('');
  let spectatorCodeInput = $state('');
  let selectCaster = $state(false); 

  // Helper for node-aware API fetching
  function nodeFetch(path: string, init?: RequestInit): Promise<Response> {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('backend_node_url');
      const base = saved && saved.trim() ? saved.trim().replace(/\/$/, '') : '';
      const url = base ? `${base}${path}` : path;
      const finalInit = { ...init };
      finalInit.credentials = 'include';
      return fetch(url, finalInit);
    }
    return fetch(path, init);
  }

  onMount(async () => {
    // Read from localStorage ONLY on client mount to prevent SSR hydration mismatch
    const saved = localStorage.getItem('backend_node_url');
    currentBackendNode = saved && saved.trim() ? saved.trim() : 'Default Host';
    backendIpInput = currentBackendNode === 'Default Host' ? '' : currentBackendNode;

    try {
      const configRes = await nodeFetch('/api/config');
      if (configRes.ok) {
        const config = await configRes.json();
        if (config.multiplayer) {
          const authRes = await nodeFetch('/api/auth/me');
          if (!authRes.ok) {
            goto('/login');
            return;
          }
        }
      }
    } catch (e) {
      console.warn("Backend node unreachable on initialization", e);
    }
    configLoaded = true;
  });

  async function handleLogout() {
    try {
      await nodeFetch('/api/auth/logout', { method: 'POST' });
      goto('/login');
    } catch (e) {}
  }

  // Node Switcher handlers
  async function testAndSaveNode() {
    connectionTestMsg = 'Testing connection...';
    connectionTestSuccess = null;
    let url = backendIpInput.trim();
    
    if (!url) {
      localStorage.removeItem('backend_node_url');
      currentBackendNode = 'Default Host';
      connectionTestMsg = 'Reset to default node successfully!';
      connectionTestSuccess = true;
      setTimeout(() => {
        showNodeModal = false;
        window.location.reload();
      }, 1000);
      return;
    }

    if (!/^https?:\/\//i.test(url)) {
      url = 'http://' + url;
    }

    try {
      const res = await fetch(`${url}/api/settings`, { credentials: 'include' });
      if (res.ok) {
        localStorage.setItem('backend_node_url', url);
        currentBackendNode = url;
        connectionTestMsg = 'Connected successfully! Saving node settings...';
        connectionTestSuccess = true;
        setTimeout(() => {
          showNodeModal = false;
          window.location.reload();
        }, 1000);
      } else {
        connectionTestMsg = `Failed: Server returned status ${res.status}`;
        connectionTestSuccess = false;
      }
    } catch (err: any) {
      connectionTestMsg = 'Connection refused. Ensure backend is active & CORS is supported.';
      connectionTestSuccess = false;
    }
  }

  function resetNodeDefault() {
    backendIpInput = '';
    testAndSaveNode();
  }

  // Lobby actions
  async function handleCreateLobby() {
    try {
      const res = await nodeFetch('/api/lobby/create', { method: 'POST' });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      goto(`/lobby/${data.code}`);
    } catch (err: any) {
      alert(err.message || 'Failed to create lobby');
    }
  }

  function handleJoinLobby(e: SubmitEvent) {
    e.preventDefault();
    const code = lobbyCodeInput.trim().toUpperCase();
    if (code.length !== 6) {
      alert('Room code must be exactly 6 characters');
      return;
    }
    goto(`/lobby/${code}`);
  }

  function handleJoinSpectatorCaster(e: SubmitEvent) {
    e.preventDefault();
    const code = spectatorCodeInput.trim().toUpperCase();
    if (code.length !== 6) {
      alert('Lobby code must be exactly 6 characters');
      return;
    }
    if (selectCaster) {
      goto(`/caster/${code}`);
    } else {
      goto(`/spectator/${code}`);
    }
  }
</script>

<svelte:head>
  <title>Dashboard Hub - FH6 Telemetry</title>
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@400;600;700;800&family=JetBrains+Mono:wght@500;700&family=Space+Grotesk:wght@500;700&display=swap" rel="stylesheet" />
  <link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap" rel="stylesheet" />
</svelte:head>

<div class="hub-container">
  <!-- Glowing decorative depth blobs -->
  <div class="glow-blob blob-1"></div>
  <div class="glow-blob blob-2"></div>
  <div class="glow-blob blob-3"></div>

  <!-- Header Nav (Optimized height & text) -->
  <header class="hub-header">
    <div class="logo">
      <span class="logo-accent">FH6</span> TELEMETRY
    </div>
    
    <div class="header-right">
      <button class="node-switcher-trigger" onclick={() => showNodeModal = true}>
        <span class="pulse-dot {currentBackendNode === 'Default Host' ? 'bg-cyan' : 'bg-purple'}"></span>
        <span class="node-text">Node: {currentBackendNode === 'Default Host' ? 'Local Host' : currentBackendNode.replace(/^https?:\/\//i, '')}</span>
        <span class="material-symbols-outlined text-[13px]">settings_ethernet</span>
      </button>

      {#if $currentUser}
        <div class="user-menu">
          <span class="username">👤 {$currentUser.username}</span>
          {#if $currentUser.role === 'admin'}
            <a href="/admin" class="admin-badge">Admin</a>
          {/if}
          <button class="logout-btn" onclick={handleLogout}>Logout</button>
        </div>
      {/if}
    </div>
  </header>

  {#if configLoaded}
    <main class="hub-main">
      <!-- Hero Section (More compact typography) -->
      <div class="hero-section">
        <h1>CHOOSE YOUR INTERFACE</h1>
        <p class="subtitle">Select a telemetry dashboard, join convoys, or spectate races in real-time</p>
      </div>

      <!-- Dashboard Choices Grid (Optimized columns & smaller cards) -->
      <div class="choices-grid">
        <!-- Card 1: Pro Dashboard -->
        <a href="/pro" class="choice-card pro-card">
          <div class="card-glow glow-cyan-radial"></div>
          <div class="card-content">
            <div class="badge cyan-bg">PREMIUM</div>
            <div class="icon-wrap">
              <span class="material-symbols-outlined text-cyan">dashboard_customize</span>
            </div>
            <h2>Pro Dashboard</h2>
            <p>High-performance workspace featuring a fully customizable drag-and-drop widget engine. Save layouts and record/replay race sessions.</p>
            <ul class="features-list">
              <li>🛠️ Fully Customizable Grid System</li>
              <li>🔴 Live Session Recording (Manual Start/Stop)</li>
              <li>📊 Full Session Replays & Timelines</li>
              <li>🏎️ Premium 3-Mode HUD & Pedal Inputs</li>
            </ul>
            <div class="card-footer">
              <span class="launch-label">LAUNCH PRO</span>
              <span class="material-symbols-outlined arrow-icon">arrow_forward</span>
            </div>
          </div>
        </a>

        <!-- Card 2: Standard Dashboard -->
        <a href="/standard" class="choice-card standard-card">
          <div class="card-glow glow-blue-radial"></div>
          <div class="card-content">
            <div class="icon-wrap">
              <span class="material-symbols-outlined text-blue">dashboard</span>
            </div>
            <h2>Standard Dashboard</h2>
            <p>Minimalist layout featuring a precision compass bar, live RPM dial, tire temperature telemetry, and standard lap timeline tracker.</p>
            <ul class="features-list">
              <li>⚡ Precision Compass & Speedometer</li>
              <li>🔥 Live Tire Temps (FL, FR, RL, RR)</li>
              <li>🗺️ Live Vector Map Trace</li>
              <li>🏁 Quick timing & lap bar</li>
            </ul>
            <div class="card-footer">
              <span class="launch-label">LAUNCH STANDARD</span>
              <span class="material-symbols-outlined arrow-icon">arrow_forward</span>
            </div>
          </div>
        </a>

        <!-- Card 3: Multiplayer Convoy Hub -->
        <div class="choice-card interactive-card lobby-card">
          <div class="card-glow glow-pink-radial"></div>
          <div class="card-content">
            <div class="icon-wrap">
              <span class="material-symbols-outlined text-pink">groups</span>
            </div>
            <h2>Multiplayer Convoy</h2>
            <p>Host or join telemetry rooms to stream and log multiple players' data concurrently. Perfect for convoys, team racing, or league comparisons.</p>
            
            {#if showLobbyInput}
              <form onsubmit={handleJoinLobby} class="interactive-form">
                <input
                  type="text"
                  maxlength="6"
                  placeholder="ENTER 6-CHAR CODE"
                  bind:value={lobbyCodeInput}
                  class="code-input"
                  required
                />
                <div class="form-actions">
                  <button type="submit" class="submit-btn text-pink-btn">JOIN ROOM</button>
                  <button type="button" class="cancel-btn" onclick={() => showLobbyInput = false}>CANCEL</button>
                </div>
              </form>
            {:else}
              <div class="card-actions">
                <button class="action-btn pink-outline-btn" onclick={handleCreateLobby}>HOST ROOM</button>
                <button class="action-btn pink-filled-btn" onclick={() => showLobbyInput = true}>JOIN CONVOY</button>
              </div>
            {/if}

            <ul class="features-list mt-auto">
              <li>👥 Support up to 12 slots concurrently</li>
              <li>🏁 Live leaderboards & convoy telemetry</li>
              <li>🔴 Sessions synced directly to server</li>
            </ul>
          </div>
        </div>

        <!-- Card 4: Spectator & Caster -->
        <div class="choice-card interactive-card spec-card">
          <div class="card-glow glow-purple-radial"></div>
          <div class="card-content">
            <div class="icon-wrap">
              <span class="material-symbols-outlined text-purple">podium</span>
            </div>
            <h2>Spectator & Caster Hub</h2>
            <p>Watch multiplayer telemetry sessions in real-time or cast telemetry overlay statistics. View maps, relative gaps, and timing logs.</p>
            
            {#if showSpectatorInput}
              <form onsubmit={handleJoinSpectatorCaster} class="interactive-form">
                <input
                  type="text"
                  maxlength="6"
                  placeholder="ENTER LOBBY CODE"
                  bind:value={spectatorCodeInput}
                  class="code-input"
                  required
                />
                <div class="role-selector">
                  <label class="radio-label">
                    <input type="radio" name="role" checked={!selectCaster} onclick={() => selectCaster = false} />
                    <span>Spectator Mode</span>
                  </label>
                  <label class="radio-label">
                    <input type="radio" name="role" checked={selectCaster} onclick={() => selectCaster = true} />
                    <span>Caster overlay</span>
                  </label>
                </div>
                <div class="form-actions">
                  <button type="submit" class="submit-btn text-purple-btn">LAUNCH OVERLAY</button>
                  <button type="button" class="cancel-btn" onclick={() => showSpectatorInput = false}>CANCEL</button>
                </div>
              </form>
            {:else}
              <div class="card-actions">
                <button class="action-btn purple-filled-btn" onclick={() => showSpectatorInput = true}>ENTER LOBBY OVERLAY</button>
              </div>
            {/if}

            <ul class="features-list mt-auto">
              <li>🏁 Real-time relative gaps & driver timing</li>
              <li>🎙️ Dedicated caster analytics & charts</li>
              <li>🗺️ Live map displaying all convoy drivers</li>
            </ul>
          </div>
        </div>

        <!-- Card 5: Nearby Radar -->
        <a href="/nearby" class="choice-card nearby-card">
          <div class="card-glow glow-yellow-radial"></div>
          <div class="card-content">
            <div class="icon-wrap">
              <span class="material-symbols-outlined text-yellow">radar</span>
            </div>
            <h2>Live Nearby Radar</h2>
            <p>Open a live real-time radar mapping utility showing other players' coordinates, speed angles, and distance tracking in a clean cyber grid map.</p>
            <ul class="features-list">
              <li>🗺️ Full screen real-time track map</li>
              <li>📍 Grid locator for nearby cars</li>
              <li>📊 Dynamic vector tracing of lines</li>
            </ul>
            <div class="card-footer">
              <span class="launch-label">LAUNCH RADAR</span>
              <span class="material-symbols-outlined arrow-icon">arrow_forward</span>
            </div>
          </div>
        </a>
      </div>

      <!-- Quick Guide Section (Sleeker & more compact) -->
      <section class="guide-section">
        <div class="guide-header">
          <span class="material-symbols-outlined text-cyan">sports_esports</span>
          <h2>Quick Telemetry Calibration Guide</h2>
        </div>
        <div class="guide-grid">
          <div class="guide-step">
            <div class="step-num">1</div>
            <h3>Configure Port</h3>
            <p>Ensure the receiver port in telemetry settings is configured. Default receiver port is <code class="port-code">20440</code>.</p>
          </div>
          <div class="guide-step">
            <div class="step-num">2</div>
            <h3>Forza Settings</h3>
            <p>In Forza Horizon 5 / Motorsport settings, set **Data Out** to **ON**, **IP** to your PC's IP, and **Port** to <code class="port-code">20440</code>.</p>
          </div>
          <div class="guide-step">
            <div class="step-num">3</div>
            <h3>Map Auto-Resizing</h3>
            <p>If the track map is out-of-scale, toggle map **ON/OFF** in settings, or click **Refresh** (🔄) button in the widget header to auto-rescale.</p>
          </div>
        </div>
      </section>
    </main>
  {/if}

  <!-- Node Switcher Modal (Overlay) -->
  {#if showNodeModal}
    <div class="modal-overlay" onclick={() => showNodeModal = false}>
      <div class="modal-card" onclick={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div class="modal-title">
            <span class="material-symbols-outlined">settings_ethernet</span>
            <h2>Backend Node Configuration</h2>
          </div>
          <button class="close-btn" onclick={() => showNodeModal = false}>✕</button>
        </div>
        
        <div class="modal-body">
          <p class="modal-desc">
            Connect this frontend client to a custom backend telemetry server node. This enables multinode/IP routing, so a single UI can manage and log telemetry to different servers.
          </p>
          
          <div class="input-group">
            <label for="nodeUrl">Backend Node IP / URL</label>
            <input
              type="text"
              id="nodeUrl"
              placeholder="e.g. 192.168.1.100:5173"
              bind:value={backendIpInput}
            />
            <span class="input-tip">Leave empty to reset to Default Host server node.</span>
          </div>

          {#if connectionTestMsg}
            <div class="test-banner {connectionTestSuccess === true ? 'success-banner' : connectionTestSuccess === false ? 'error-banner' : 'pending-banner'}">
              <span>{connectionTestMsg}</span>
            </div>
          {/if}
        </div>

        <div class="modal-actions">
          <button class="action-btn secondary-btn" onclick={resetNodeDefault}>RESET TO DEFAULT</button>
          <button class="action-btn primary-btn" onclick={testAndSaveNode}>TEST & CONNECT</button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .hub-container {
    position: relative;
    width: 100vw;
    min-height: 100vh;
    min-height: 100dvh;
    background: #04060c;
    color: #f3f4f6;
    overflow-x: hidden;
    overflow-y: auto;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
    display: flex;
    flex-direction: column;
  }

  /* Animated Glowing blobs (Reduced blur and opacity for better performance) */
  .glow-blob {
    position: absolute;
    border-radius: 50%;
    filter: blur(100px);
    opacity: 0.08;
    pointer-events: none;
    z-index: 1;
  }
  .blob-1 { top: -10%; left: 10%; width: 350px; height: 350px; background: #00dbe9; }
  .blob-2 { bottom: 10%; right: 5%; width: 400px; height: 400px; background: #8b5cf6; }
  .blob-3 { top: 40%; left: 45%; width: 300px; height: 300px; background: #3b82f6; }

  /* Navigation Header (More compact) */
  .hub-header {
    position: relative;
    z-index: 10;
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.85rem 1.5rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    background: rgba(8, 10, 15, 0.7);
    backdrop-filter: blur(12px);
  }
  .logo { font-size: 1.15rem; font-weight: 800; letter-spacing: 0.08em; }
  .logo-accent { color: #00dbe9; text-shadow: 0 0 10px rgba(0, 219, 233, 0.35); }
  
  .header-right { display: flex; align-items: center; gap: 1rem; }
  .user-menu { display: flex; align-items: center; gap: 0.5rem; }
  .username { font-size: 0.8rem; color: #9ca3af; }
  
  .admin-badge {
    background: rgba(59, 130, 246, 0.12);
    border: 1px solid rgba(59, 130, 246, 0.25);
    color: #93c5fd;
    font-size: 0.68rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    text-decoration: none;
    transition: all 0.2s ease;
  }
  .admin-badge:hover { background: #3b82f6; color: #fff; }

  .logout-btn {
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.25);
    color: #fca5a5;
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.25rem 0.6rem;
    border-radius: 5px;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .logout-btn:hover { background: #ef4444; color: #fff; border-color: #ef4444; }

  /* Node Switcher Button */
  .node-switcher-trigger {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.06);
    color: #d1d5db;
    font-size: 0.72rem;
    font-weight: 700;
    padding: 0.3rem 0.65rem;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .node-switcher-trigger:hover {
    background: rgba(255, 255, 255, 0.05);
    border-color: rgba(255, 255, 255, 0.15);
  }
  .pulse-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    animation: blink 2s infinite;
  }
  .bg-cyan { background-color: #00dbe9; box-shadow: 0 0 6px #00dbe9; }
  .bg-purple { background-color: #a855f7; box-shadow: 0 0 6px #a855f7; }
  @keyframes blink { 0%, 100% { opacity: 0.4; } 50% { opacity: 1; } }

  /* Main Area */
  .hub-main {
    position: relative;
    z-index: 5;
    max-width: 1100px;
    width: 100%;
    margin: 0 auto;
    padding: 2rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .hero-section { text-align: center; }
  .hero-section h1 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: clamp(1.4rem, 4vw, 2.2rem);
    font-weight: 700;
    letter-spacing: -0.02em;
    background: linear-gradient(135deg, #fff 50%, #00dbe9);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    margin-bottom: 0.5rem;
  }
  .subtitle { font-size: 0.85rem; color: #9ca3af; max-width: 500px; margin: 0 auto; line-height: 1.4; }

  /* Choices Grid (Sleeker and smaller min-width) */
  .choices-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(290px, 1fr));
    gap: 1.5rem;
  }

  .choice-card {
    position: relative;
    display: flex;
    flex-direction: column;
    text-decoration: none;
    color: inherit;
    background: rgba(13, 15, 22, 0.7);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 12px;
    padding: 1.5rem;
    overflow: hidden;
    backdrop-filter: blur(12px);
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }
  .choice-card:hover {
    transform: translateY(-3px);
    border-color: rgba(255, 255, 255, 0.12);
  }

  .pro-card:hover { border-color: rgba(0, 219, 233, 0.35); box-shadow: 0 10px 20px rgba(0, 219, 233, 0.08); }
  .standard-card:hover { border-color: rgba(59, 130, 246, 0.35); box-shadow: 0 10px 20px rgba(59, 130, 246, 0.08); }
  .lobby-card:hover { border-color: rgba(244, 114, 182, 0.35); box-shadow: 0 10px 20px rgba(244, 114, 182, 0.08); }
  .spec-card:hover { border-color: rgba(168, 85, 247, 0.35); box-shadow: 0 10px 20px rgba(168, 85, 247, 0.08); }
  .nearby-card:hover { border-color: rgba(234, 179, 8, 0.35); box-shadow: 0 10px 20px rgba(234, 179, 8, 0.08); }

  .choice-card:hover .card-glow { opacity: 0.06; }
  .choice-card:hover .arrow-icon { transform: translateX(3px); }
  .pro-card:hover .arrow-icon { color: #00dbe9; }
  .standard-card:hover .arrow-icon { color: #3b82f6; }
  .nearby-card:hover .arrow-icon { color: #eab308; }

  .card-glow {
    position: absolute;
    inset: 0;
    opacity: 0.01;
    transition: opacity 0.25s ease;
    pointer-events: none;
  }
  .glow-cyan-radial { background: radial-gradient(circle at 50% 0%, #00dbe9 0%, transparent 60%); }
  .glow-blue-radial { background: radial-gradient(circle at 50% 0%, #3b82f6 0%, transparent 60%); }
  .glow-pink-radial { background: radial-gradient(circle at 50% 0%, #f472b6 0%, transparent 60%); }
  .glow-purple-radial { background: radial-gradient(circle at 50% 0%, #a855f7 0%, transparent 60%); }
  .glow-yellow-radial { background: radial-gradient(circle at 50% 0%, #eab308 0%, transparent 60%); }

  .card-content { display: flex; flex-direction: column; height: 100%; position: relative; z-index: 2; flex: 1; }
  
  .badge {
    align-self: flex-start;
    color: #030712;
    font-size: 0.6rem;
    font-weight: 800;
    letter-spacing: 0.05em;
    padding: 0.2rem 0.5rem;
    border-radius: 3px;
    margin-bottom: 1rem;
  }
  .cyan-bg { background: linear-gradient(135deg, #00dbe9, #3b82f6); }

  .icon-wrap {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border-radius: 10px;
    background: rgba(255, 255, 255, 0.01);
    border: 1px solid rgba(255, 255, 255, 0.04);
    margin-bottom: 1rem;
  }
  .icon-wrap span { font-size: 20px; }
  .text-cyan { color: #00dbe9; }
  .text-blue { color: #3b82f6; }
  .text-pink { color: #f472b6; }
  .text-purple { color: #a855f7; }
  .text-yellow { color: #eab308; }

  .choice-card h2 { font-size: 1.25rem; font-weight: 700; margin-bottom: 0.5rem; letter-spacing: -0.01em; }
  .choice-card p { font-size: 0.82rem; color: #9ca3af; line-height: 1.45; margin-bottom: 1rem; }

  .features-list { display: flex; flex-direction: column; gap: 0.5rem; list-style: none; margin-bottom: 1.5rem; }
  .features-list li { font-size: 0.78rem; color: #e5e7eb; display: flex; align-items: center; gap: 0.4rem; }

  .card-footer {
    margin-top: auto;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-top: 1px solid rgba(255, 255, 255, 0.04);
    padding-top: 0.85rem;
  }
  .launch-label { font-size: 0.72rem; font-weight: 700; letter-spacing: 0.05em; color: #fff; }
  .arrow-icon { color: #9ca3af; font-size: 18px; transition: all 0.2s ease; }

  /* Interactive Elements for Lobby / Spectating */
  .card-actions { display: flex; gap: 0.5rem; margin-bottom: 1rem; }
  .action-btn {
    flex: 1;
    font-size: 0.75rem;
    font-weight: 700;
    padding: 0.5rem 0.65rem;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
    border: 1px solid transparent;
  }
  
  .pink-outline-btn { background: transparent; border-color: rgba(244, 114, 182, 0.25); color: #f472b6; }
  .pink-outline-btn:hover { background: rgba(244, 114, 182, 0.08); border-color: #f472b6; }
  
  .pink-filled-btn { background: #f472b6; color: #030712; }
  .pink-filled-btn:hover { filter: brightness(1.1); }
  
  .purple-filled-btn { background: #a855f7; color: #fff; width: 100%; }
  .purple-filled-btn:hover { filter: brightness(1.1); }

  .interactive-form {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    margin-bottom: 1rem;
    background: rgba(0, 0, 0, 0.15);
    padding: 0.85rem;
    border-radius: 8px;
    border: 1px solid rgba(255, 255, 255, 0.03);
  }
  .code-input {
    width: 100%;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.06);
    color: #fff;
    border-radius: 5px;
    padding: 0.45rem 0.65rem;
    font-size: 0.85rem;
    font-weight: bold;
    text-align: center;
    letter-spacing: 0.08em;
    font-family: 'JetBrains Mono', monospace;
  }
  .code-input:focus { outline: none; border-color: #00dbe9; }
  .form-actions { display: flex; gap: 0.4rem; }
  .submit-btn {
    flex: 2;
    border: none;
    border-radius: 5px;
    padding: 0.45rem;
    font-size: 0.72rem;
    font-weight: 700;
    cursor: pointer;
    color: #030712;
  }
  .text-pink-btn { background: #f472b6; }
  .text-purple-btn { background: #a855f7; color: #fff; }
  .submit-btn:hover { filter: brightness(1.1); }
  
  .cancel-btn {
    flex: 1;
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: #9ca3af;
    border-radius: 5px;
    padding: 0.45rem;
    font-size: 0.72rem;
    font-weight: 700;
    cursor: pointer;
  }
  .cancel-btn:hover { background: rgba(255, 255, 255, 0.03); color: #fff; }

  .role-selector {
    display: flex;
    justify-content: space-around;
    gap: 0.75rem;
    margin: 0.15rem 0;
  }
  .radio-label {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    color: #9ca3af;
    cursor: pointer;
  }
  .radio-label input { cursor: pointer; accent-color: #a855f7; }

  /* Guide Section (Sleeker and smaller margins) */
  .guide-section {
    background: rgba(13, 15, 22, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.03);
    border-radius: 12px;
    padding: 1.75rem;
    margin-top: 0.5rem;
  }
  .guide-header { display: flex; align-items: center; gap: 0.6rem; margin-bottom: 1.25rem; }
  .guide-header span { font-size: 22px; }
  .guide-header h2 { font-family: 'Space Grotesk', sans-serif; font-size: 1.1rem; font-weight: 700; }

  .guide-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 1.5rem;
  }
  .guide-step { display: flex; flex-direction: column; gap: 0.5rem; position: relative; }
  .step-num {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: rgba(0, 219, 233, 0.08);
    border: 1px solid rgba(0, 219, 233, 0.25);
    color: #00dbe9;
    font-size: 0.75rem;
    font-weight: 700;
  }
  .guide-step h3 { font-size: 0.88rem; font-weight: 700; margin-top: 0.1rem; }
  .guide-step p { font-size: 0.78rem; color: #9ca3af; line-height: 1.4; }
  .port-code { font-family: 'JetBrains Mono', monospace; background: rgba(255, 255, 255, 0.04); color: #00dbe9; padding: 0.05rem 0.25rem; border-radius: 3px; }

  /* Modal Overlay Styles */
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.75);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }
  .modal-card {
    background: #0a0c10;
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 12px;
    width: 100%;
    max-width: 440px;
    padding: 1.75rem;
    box-shadow: 0 15px 30px rgba(0,0,0,0.5);
    animation: zoomIn 0.15s ease-out;
  }
  @keyframes zoomIn { 0% { transform: scale(0.96); opacity: 0; } 100% { transform: scale(1); opacity: 1; } }

  .modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem; }
  .modal-title { display: flex; align-items: center; gap: 0.4rem; }
  .modal-title span { color: #a855f7; font-size: 22px; }
  .modal-title h2 { font-size: 1.1rem; font-weight: 700; }
  .close-btn { background: none; border: none; color: #9ca3af; font-size: 1.1rem; cursor: pointer; }
  .close-btn:hover { color: #fff; }

  .modal-body { display: flex; flex-direction: column; gap: 1rem; }
  .modal-desc { font-size: 0.8rem; color: #9ca3af; line-height: 1.45; }
  
  .input-group { display: flex; flex-direction: column; gap: 0.35rem; }
  .input-group label { font-size: 0.72rem; font-weight: 600; text-transform: uppercase; color: #9ca3af; letter-spacing: 0.04em; }
  .input-group input {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 6px;
    color: #fff;
    padding: 0.55rem 0.75rem;
    font-size: 0.85rem;
    transition: all 0.2s;
  }
  .input-group input:focus { outline: none; border-color: #a855f7; }
  .input-tip { font-size: 0.68rem; color: #52525b; }

  .test-banner {
    padding: 0.55rem 0.85rem;
    border-radius: 6px;
    font-size: 0.75rem;
    line-height: 1.35;
  }
  .pending-banner { background: rgba(59, 130, 246, 0.08); border: 1px solid rgba(59, 130, 246, 0.25); color: #93c5fd; }
  .success-banner { background: rgba(34, 197, 94, 0.08); border: 1px solid rgba(34, 197, 94, 0.25); color: #86efac; }
  .error-banner { background: rgba(239, 68, 68, 0.08); border: 1px solid rgba(239, 68, 68, 0.25); color: #fca5a5; }

  .modal-actions { display: flex; gap: 0.5rem; margin-top: 1.5rem; }
  .modal-actions .action-btn { padding: 0.55rem 1rem; font-size: 0.75rem; }
  .secondary-btn { background: transparent; border: 1px solid rgba(255, 255, 255, 0.08); color: #9ca3af; }
  .secondary-btn:hover { background: rgba(255, 255, 255, 0.02); color: #fff; }
  .primary-btn { background: #a855f7; color: #fff; }
  .primary-btn:hover { filter: brightness(1.1); }

  .mt-auto { margin-top: auto; }
  
  @media (max-width: 640px) {
    .hub-header { padding: 0.75rem 1rem; }
    .hub-main { padding: 1.25rem 0.85rem; gap: 1.5rem; }
    .guide-section { padding: 1.25rem; }
    .modal-card { padding: 1.25rem; }
  }
</style>
