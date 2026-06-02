<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { invoke } from '@tauri-apps/api/tauri';
  import { listen } from '@tauri-apps/api/event';

  // Svelte 5 State Runes
  let config = $state({ bindPort: 20440, destinations: [] as any[] });
  let rx = $state(0);
  let tx = $state(0);
  let pps = $state(0);
  let avgUs = $state(0);

  // Form states
  let newName = $state("");
  let newHost = $state("");
  let newPort = $state(20500);
  let newRate = $state("60Hz");
  let bindPortInput = $state(20440);

  // Notification message
  let notification = $state({ message: "", type: "" });
  let unlistenFn: (() => void) | null = null;

  onMount(async () => {
    try {
      await refreshConfig();
      
      // Register event listener for real-time Rust metrics
      unlistenFn = await listen('stats_update', (event: any) => {
        const stats = event.payload;
        rx = stats.rxPackets;
        tx = stats.txPackets;
        pps = stats.pps;
        avgUs = stats.avgUs;
      });
    } catch (e) {
      showNotification("Gagal terhubung dengan engine Rust", "error");
    }
  });

  onDestroy(() => {
    if (unlistenFn) {
      unlistenFn();
    }
  });

  async function refreshConfig() {
    config = await invoke('get_config');
    bindPortInput = config.bindPort;
  }

  function showNotification(msg: string, type = "success") {
    notification = { message: msg, type };
    setTimeout(() => {
      notification = { message: "", type: "" };
    }, 3000);
  }

  async function handleAddNode(e: Event) {
    e.preventDefault();
    if (!newName || !newHost || newPort <= 0) {
      showNotification("Harap isi semua kolom dengan benar!", "error");
      return;
    }

    const success = await invoke('add_destination', {
      name: newName,
      host: newHost,
      port: newPort,
      rateLimit: newRate
    });

    if (success) {
      showNotification(`Node ${newName} berhasil ditambahkan!`);
      newName = "";
      newHost = "";
      newPort = 20500;
      newRate = "60Hz";
      await refreshConfig();
    } else {
      showNotification("Gagal menambahkan node", "error");
    }
  }

  async function handleToggleNode(id: string, currentEnabled: boolean) {
    const success = await invoke('toggle_destination', { id, enabled: !currentEnabled });
    if (success) {
      showNotification(currentEnabled ? "Node dinonaktifkan" : "Node diaktifkan");
      await refreshConfig();
    }
  }

  async function handleDeleteNode(id: string, name: string) {
    if (confirm(`Apakah Anda yakin ingin menghapus node "${name}"?`)) {
      const success = await invoke('delete_destination', { id });
      if (success) {
        showNotification(`Node "${name}" berhasil dihapus`);
        await refreshConfig();
      }
    }
  }

  async function handleSaveBindPort() {
    if (bindPortInput <= 0 || bindPortInput > 65535) {
      showNotification("Nomor Port tidak valid!", "error");
      return;
    }

    const success = await invoke('update_bind_port', { port: bindPortInput });
    if (success) {
      showNotification(`Port pendengar UDP disetel ke ${bindPortInput}`);
      await refreshConfig();
    } else {
      showNotification("Gagal memperbarui port", "error");
    }
  }
</script>

<main class="dashboard-shell">
  <!-- Top Navigation Header -->
  <header class="top-nav">
    <div class="logo-area">
      <span class="flag-icon">🏁</span>
      <h1>FH6 TELEMETRY <span class="accent-text">MIRROR HUB (TAURI)</span></h1>
    </div>
    <div class="status-indicator">
      <span class="indicator-dot {pps > 0 ? 'active' : ''}"></span>
      <span class="status-label">{pps > 0 ? 'STREAM LIVE CONNECTED' : 'AWAITING FORZA UDP STREAM'}</span>
    </div>
  </header>

  <!-- Notification Banner -->
  {#if notification.message}
    <div class="notification-toast {notification.type}">
      {notification.message}
    </div>
  {/if}

  <!-- Main Container Grid -->
  <div class="layout-grid">
    
    <!-- Real-Time Metrics Area -->
    <section class="metrics-container">
      <div class="metric-card cyan-glow">
        <span class="card-title">INBOUND PAYLOAD</span>
        <div class="value-large">{pps} <span class="unit">pps</span></div>
        <div class="card-bar cyan"></div>
        <p class="subtitle">Frekuensi Paket Masuk</p>
      </div>

      <div class="metric-card green-glow">
        <span class="card-title">AVG DELAY</span>
        <div class="value-large">{avgUs} <span class="unit">µs</span></div>
        <div class="card-bar green"></div>
        <p class="subtitle">Rentang Overhead Copy</p>
      </div>

      <div class="metric-card purple-glow">
        <span class="card-title">PACKETS RX</span>
        <div class="value-large">{rx.toLocaleString()}</div>
        <div class="card-bar purple"></div>
        <p class="subtitle">Total Paket Diterima</p>
      </div>

      <div class="metric-card magenta-glow">
        <span class="card-title">PACKETS TX</span>
        <div class="value-large">{tx.toLocaleString()}</div>
        <div class="card-bar magenta"></div>
        <p class="subtitle">Total Paket Diduplikasi</p>
      </div>
    </section>

    <!-- Configuration Panels -->
    <div class="panels-grid">
      
      <!-- Left Panel: Active Targets & Listener Port -->
      <div class="panel-card main-area">
        <div class="panel-header">
          <h2>RUTE DUPLIKATOR TELESIGNAL</h2>
          
          <!-- Inbound port configuration inline -->
          <div class="port-config">
            <label for="bindPort">UDP INBOUND PORT:</label>
            <input type="number" id="bindPort" bind:value={bindPortInput} min="1" max="65535"/>
            <button class="btn-save-port" onclick={handleSaveBindPort}>Set</button>
          </div>
        </div>

        <div class="destination-list">
          {#if config.destinations.length === 0}
            <div class="empty-state">
              <span class="empty-icon">📂</span>
              <p>Belum ada rute mirroring yang terdaftar. Tambahkan rute baru di panel kanan!</p>
            </div>
          {:else}
            <div class="table-container">
              <table>
                <thead>
                  <tr>
                    <th>NAMA TARGET</th>
                    <th>ALAMAT JARINGAN</th>
                    <th>RATE LIMIT</th>
                    <th>STATUS RUTE</th>
                    <th class="align-right">AKSI</th>
                  </tr>
                </thead>
                <tbody>
                  {#each config.destinations as d (d.id)}
                    <tr class="dest-row {d.enabled ? 'active-row' : 'disabled-row'}">
                      <td class="dest-name font-bold">{d.name}</td>
                      <td>
                        <span class="network-badge">{d.host}:{d.port}</span>
                      </td>
                      <td>
                        <span class="rate-badge">{d.rateLimit}</span>
                      </td>
                      <td>
                        <label class="switch">
                          <input type="checkbox" checked={d.enabled} onchange={() => handleToggleNode(d.id, d.enabled)} />
                          <span class="slider"></span>
                        </label>
                      </td>
                      <td class="align-right">
                        <button class="btn-delete" aria-label="Hapus Rute" onclick={() => handleDeleteNode(d.id, d.name)}>
                          🗑️
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      </div>

      <!-- Right Panel: Add Route Form -->
      <div class="panel-card side-area">
        <h2>TAMBAH RUTE BARU</h2>
        <p class="form-desc">Salurkan aliran telemetri 60Hz ke software pihak ketiga, dasbor, atau perangkat lainnya.</p>

        <form class="add-node-form" onsubmit={handleAddNode}>
          <div class="form-group">
            <label for="name">Nama Deskriptif</label>
            <input type="text" id="name" placeholder="Contoh: SimHub Rig, iPhone Dash" bind:value={newName} required />
          </div>

          <div class="form-row">
            <div class="form-group flex-2">
              <label for="host">IP / Host Target</label>
              <input type="text" id="host" placeholder="e.g. 192.168.1.100" bind:value={newHost} required />
            </div>
            <div class="form-group flex-1">
              <label for="port">UDP Port</label>
              <input type="number" id="port" min="1" max="65535" bind:value={newPort} required />
            </div>
          </div>

          <div class="form-group">
            <label for="rate">Batas Frekuensi Data (Throttling)</label>
            <div class="select-wrapper">
              <select id="rate" bind:value={newRate}>
                <option value="60Hz">60Hz (Tanpa Limit - Sinkron Game)</option>
                <option value="30Hz">30Hz (Pengurangan Frekuensi 1/2)</option>
                <option value="20Hz">20Hz (Pengurangan Frekuensi 1/3)</option>
                <option value="10Hz">10Hz (Pengurangan Frekuensi 1/6)</option>
              </select>
            </div>
          </div>

          <button type="submit" class="btn-submit">
            <span>DAFTARKAN TARGET RUTE</span>
          </button>
        </form>
      </div>

    </div>
  </div>
</main>

<style>
  /* Import premium Google Font */
  @import url('https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;800&family=JetBrains+Mono:wght@400;700&display=swap');

  :global(body) {
    margin: 0;
    padding: 0;
    font-family: 'Outfit', sans-serif;
    background-color: #040406;
    color: #e2e8f0;
    overflow-x: hidden;
    user-select: none;
  }

  .dashboard-shell {
    padding: 24px;
    max-width: 1200px;
    margin: 0 auto;
    min-height: calc(100vh - 48px);
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  /* Top Navigation Header Styling */
  .top-nav {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    background: rgba(10, 10, 18, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 16px;
    backdrop-filter: blur(20px);
  }

  .logo-area {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .flag-icon {
    font-size: 24px;
    filter: drop-shadow(0 0 8px rgba(0, 219, 233, 0.4));
  }

  .top-nav h1 {
    font-size: 20px;
    font-weight: 800;
    margin: 0;
    letter-spacing: 1px;
    color: #ffffff;
  }

  .accent-text {
    background: linear-gradient(135deg, #00dbe9 0%, #d946ef 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .status-indicator {
    display: flex;
    align-items: center;
    gap: 10px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid rgba(255, 255, 255, 0.05);
    padding: 6px 14px;
    border-radius: 99px;
  }

  .indicator-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: #ef4444;
    box-shadow: 0 0 10px rgba(239, 68, 68, 0.6);
  }

  .indicator-dot.active {
    background-color: #10b981;
    box-shadow: 0 0 12px rgba(16, 185, 129, 0.8);
    animation: pulse 1.5s infinite alternate;
  }

  .status-label {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.5px;
    color: #94a3b8;
  }

  /* Metric Cards Layout */
  .metrics-container {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
  }

  .metric-card {
    background: rgba(12, 12, 22, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.04);
    border-radius: 16px;
    padding: 20px;
    backdrop-filter: blur(15px);
    transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), border-color 0.3s ease;
    position: relative;
    overflow: hidden;
  }

  .metric-card:hover {
    transform: translateY(-4px);
    border-color: rgba(255, 255, 255, 0.1);
  }

  .card-title {
    font-size: 11px;
    font-weight: 600;
    color: #64748b;
    letter-spacing: 1px;
  }

  .value-large {
    font-family: 'JetBrains Mono', monospace;
    font-size: 28px;
    font-weight: 700;
    color: #ffffff;
    margin: 12px 0 8px 0;
  }

  .unit {
    font-size: 14px;
    color: #64748b;
    font-weight: 400;
  }

  .subtitle {
    font-size: 12px;
    color: #475569;
    margin: 0;
  }

  .card-bar {
    height: 3px;
    width: 60px;
    border-radius: 99px;
    margin-bottom: 8px;
  }

  /* Color themes for metrics */
  .card-bar.cyan { background: #00dbe9; }
  .card-bar.green { background: #10b981; }
  .card-bar.purple { background: #8b5cf6; }
  .card-bar.magenta { background: #d946ef; }

  .cyan-glow:hover { box-shadow: 0 10px 30px -10px rgba(0, 219, 233, 0.15); }
  .green-glow:hover { box-shadow: 0 10px 30px -10px rgba(16, 185, 129, 0.15); }
  .purple-glow:hover { box-shadow: 0 10px 30px -10px rgba(139, 92, 246, 0.15); }
  .magenta-glow:hover { box-shadow: 0 10px 30px -10px rgba(217, 70, 239, 0.15); }

  /* Panels Grid Styling */
  .panels-grid {
    display: grid;
    grid-template-columns: 2.2fr 1.1fr;
    gap: 20px;
  }

  .panel-card {
    background: rgba(10, 10, 18, 0.5);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 20px;
    padding: 24px;
    backdrop-filter: blur(20px);
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    padding-bottom: 16px;
  }

  h2 {
    font-size: 16px;
    font-weight: 800;
    margin: 0;
    letter-spacing: 0.5px;
    color: #ffffff;
    background: linear-gradient(135deg, #ffffff 0%, #94a3b8 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .port-config {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.06);
    padding: 4px 6px 4px 12px;
    border-radius: 10px;
  }

  .port-config label {
    font-size: 10px;
    font-weight: 700;
    color: #64748b;
    letter-spacing: 0.5px;
  }

  .port-config input {
    background: transparent;
    border: none;
    outline: none;
    color: #00dbe9;
    font-family: 'JetBrains Mono', monospace;
    font-weight: 700;
    width: 60px;
    font-size: 13px;
    padding: 0;
  }

  .btn-save-port {
    background: rgba(0, 219, 233, 0.1);
    border: 1px solid rgba(0, 219, 233, 0.2);
    color: #00dbe9;
    font-size: 11px;
    font-weight: 700;
    padding: 4px 10px;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-save-port:hover {
    background: #00dbe9;
    color: #040406;
    box-shadow: 0 0 10px rgba(0, 219, 233, 0.3);
  }

  /* Table and List Styling */
  .table-container {
    width: 100%;
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
  }

  th {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: #475569;
    padding: 10px 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  }

  td {
    padding: 14px 16px;
    font-size: 13px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.02);
  }

  .dest-row {
    transition: background 0.2s ease;
  }

  .dest-row:hover {
    background: rgba(255, 255, 255, 0.01);
  }

  .dest-name {
    color: #ffffff;
  }

  .network-badge {
    font-family: 'JetBrains Mono', monospace;
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.04);
    color: #94a3b8;
    padding: 3px 8px;
    border-radius: 6px;
  }

  .rate-badge {
    background: rgba(139, 92, 246, 0.08);
    border: 1px solid rgba(139, 92, 246, 0.15);
    color: #a78bfa;
    font-size: 11px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .align-right { text-align: right; }

  /* Custom Switch Styling */
  .switch {
    position: relative;
    display: inline-block;
    width: 38px;
    height: 20px;
  }

  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: #334155;
    transition: .3s cubic-bezier(0.16, 1, 0.3, 1);
    border-radius: 20px;
  }

  .slider:before {
    position: absolute;
    content: "";
    height: 14px;
    width: 14px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: .3s cubic-bezier(0.16, 1, 0.3, 1);
    border-radius: 50%;
  }

  input:checked + .slider {
    background-color: #10b981;
    box-shadow: 0 0 10px rgba(16, 185, 129, 0.3);
  }

  input:checked + .slider:before {
    transform: translateX(18px);
  }

  /* Form and Inputs Styling */
  .form-desc {
    font-size: 12px;
    color: #64748b;
    line-height: 1.5;
    margin-bottom: 24px;
  }

  .add-node-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-row {
    display: flex;
    gap: 12px;
  }

  .flex-1 { flex: 1; }
  .flex-2 { flex: 2; }

  label {
    font-size: 11px;
    font-weight: 700;
    color: #475569;
    letter-spacing: 0.5px;
  }

  input[type="text"], input[type="number"], select {
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 10px;
    padding: 10px 14px;
    color: #ffffff;
    font-size: 13px;
    outline: none;
    transition: all 0.2s ease;
  }

  input[type="text"]:focus, input[type="number"]:focus, select:focus {
    border-color: rgba(0, 219, 233, 0.4);
    box-shadow: 0 0 12px rgba(0, 219, 233, 0.15);
  }

  .select-wrapper {
    position: relative;
    width: 100%;
  }

  select {
    width: 100%;
    appearance: none;
    cursor: pointer;
  }

  .btn-submit {
    background: linear-gradient(135deg, #00dbe9 0%, #8b5cf6 100%);
    border: none;
    padding: 12px;
    border-radius: 10px;
    color: #040406;
    font-size: 12px;
    font-weight: 800;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    margin-top: 10px;
  }

  .btn-submit:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(0, 219, 233, 0.3);
  }

  .btn-delete {
    background: rgba(239, 68, 68, 0.05);
    border: 1px solid rgba(239, 68, 68, 0.15);
    color: #ef4444;
    padding: 6px 10px;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s ease;
    font-size: 12px;
  }

  .btn-delete:hover {
    background: #ef4444;
    color: #ffffff;
    box-shadow: 0 0 10px rgba(239, 68, 68, 0.3);
  }

  /* Toast notification */
  .notification-toast {
    position: fixed;
    bottom: 24px;
    right: 24px;
    padding: 12px 24px;
    border-radius: 12px;
    font-size: 13px;
    font-weight: 600;
    color: #ffffff;
    box-shadow: 0 10px 30px rgba(0,0,0,0.5);
    z-index: 9999;
    backdrop-filter: blur(10px);
    animation: slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .notification-toast.success {
    background: rgba(16, 185, 129, 0.15);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #34d399;
  }

  .notification-toast.error {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: #475569;
    text-align: center;
    gap: 12px;
  }

  .empty-icon {
    font-size: 32px;
    opacity: 0.5;
  }

  .empty-state p {
    font-size: 12px;
    max-width: 320px;
    line-height: 1.5;
    margin: 0;
  }

  /* Animations */
  @keyframes pulse {
    0% { opacity: 0.6; box-shadow: 0 0 8px rgba(16, 185, 129, 0.4); }
    100% { opacity: 1; box-shadow: 0 0 16px rgba(16, 185, 129, 0.8); }
  }

  @keyframes slideUp {
    from { transform: translateY(20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }
</style>
