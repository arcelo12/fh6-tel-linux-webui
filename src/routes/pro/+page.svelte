<script lang="ts">
  import { onMount } from 'svelte';
  import { displayPacket, isConnected, speedMph, speedKph, rpmPercent, startTelemetryListener, replay } from '$lib/stores/telemetry';
  import { carName } from '$lib/car-name';
  import { CAR_CLASS_LABELS, DRIVETRAIN_LABELS, type SessionRow } from '$lib/types';
  import { loadSettings, settings } from '$lib/stores/sessions';
  import LiveTrackMap from '$lib/components/LiveTrackMap.svelte';
  import SessionDrawer from '$lib/components/SessionDrawer.svelte';
  import SessionViewer from '$lib/components/SessionViewer.svelte';
  import SettingsModal from '$lib/components/SettingsModal.svelte';
  import ReplayBar from '$lib/components/ReplayBar.svelte';
  import TelemetryConfirmation from '$lib/components/TelemetryConfirmation.svelte';

  let showSessions = $state(false);
  let showSettings = $state(false);
  let viewerSession = $state<SessionRow | null>(null);
  let isRecording = $state(false);
  let isFullscreen = $state(false);

  function toggleFullscreen() {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen().catch(() => {});
      isFullscreen = true;
    } else {
      document.exitFullscreen().catch(() => {});
      isFullscreen = false;
    }
  }

  let recordingDuration = $state(0);
  let recordingInterval: ReturnType<typeof setInterval> | null = null;
  let innerWidth = $state(1920);
  let innerHeight = $state(1080);
  let zoomLevel = $derived(Math.min(innerWidth / 1280, innerHeight / 720));

  $effect(() => {
    if ($replay.active) {
      showSessions = false;
      viewerSession = null;
    }
  });

  onMount(() => {
    (async () => {
      await loadSettings();
      await startTelemetryListener();
      // Sync recording state from backend
      const res = await fetch('/api/session/status').catch(() => null);
      if (res?.ok) {
        const data = await res.json();
        isRecording = data.recording;
      }
    })();

    const handleFsChange = () => {
      isFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', handleFsChange);
    return () => {
      document.removeEventListener('fullscreenchange', handleFsChange);
    };
  });

  async function toggleRecording() {
    if (isRecording) {
      const res = await fetch('/api/session/stop', { method: 'POST' }).catch(() => null);
      if (res?.ok) {
        isRecording = false;
        if (recordingInterval) { clearInterval(recordingInterval); recordingInterval = null; }
        recordingDuration = 0;
      }
    } else {
      const res = await fetch('/api/session/start', { method: 'POST' }).catch(() => null);
      if (res?.ok) {
        isRecording = true;
        recordingDuration = 0;
        recordingInterval = setInterval(() => { recordingDuration++; }, 1000);
      }
    }
  }

  function formatDuration(sec: number) {
    const m = Math.floor(sec / 60).toString().padStart(2, '0');
    const s = (sec % 60).toString().padStart(2, '0');
    return `${m}:${s}`;
  }

  let pkt = $derived($displayPacket);
  let speed = $derived($settings?.useMph ? Math.round($speedMph) : Math.round($speedKph));
  let unit = $derived($settings?.useMph ? 'MPH' : 'KPH');
  let rpm = $derived(pkt ? Math.round(pkt.currentEngineRpm) : 0);
  let rpmPct = $derived($rpmPercent);
  let needleAngle = $derived(-140 + (rpmPct / 100) * 280);

  // Gauge style selection
  let gaugeStyle = $state<'arc' | 'digital' | 'analog'>('arc');

  function selectGaugeStyle(style: 'arc' | 'digital' | 'analog') {
    gaugeStyle = style;
    localStorage.setItem('gaugeStyle', style);
  }

  // Sparkline history buffers (last 60 samples at ~5 Hz)
  const HISTORY = 60;
  let powerHistory = $state<number[]>([]);
  let torqueHistory = $state<number[]>([]);
  let rpmHistory = $state<number[]>([]);
  
  // Suspension history buffers
  let suspFlHistory = $state<number[]>([]);
  let suspFrHistory = $state<number[]>([]);
  let suspRlHistory = $state<number[]>([]);
  let suspRrHistory = $state<number[]>([]);
  
  // Yaw seismometer history buffer
  let yawHistory = $state<number[]>([]);
  
  // Boost pressure history buffer
  let boostHistory = $state<number[]>([]);
  
  let sparkInterval: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    // Load saved gauge style
    const saved = localStorage.getItem('gaugeStyle');
    if (saved === 'arc' || saved === 'digital' || saved === 'analog') {
      gaugeStyle = saved;
    }

    // Sample sparkline data at 5Hz (every 200ms) instead of 20Hz
    // This prevents the browser from freezing due to excessive re-renders
    sparkInterval = setInterval(() => {
      const p = $displayPacket;
      if (!p) return;
      const hp = Math.max(0, Math.round(p.power / 745.7));
      const tq = Math.max(0, Math.round(p.torque));
      const r  = Math.round(p.currentEngineRpm);
      powerHistory  = powerHistory.length >= HISTORY ? [...powerHistory.slice(1), hp] : [...powerHistory, hp];
      torqueHistory = torqueHistory.length >= HISTORY ? [...torqueHistory.slice(1), tq] : [...torqueHistory, tq];
      rpmHistory    = rpmHistory.length >= HISTORY ? [...rpmHistory.slice(1), r] : [...rpmHistory, r];

      // Suspension travel history
      const sFl = p.suspensionFl;
      const sFr = p.suspensionFr;
      const sRl = p.suspensionRl;
      const sRr = p.suspensionRr;
      suspFlHistory = suspFlHistory.length >= HISTORY ? [...suspFlHistory.slice(1), sFl] : [...suspFlHistory, sFl];
      suspFrHistory = suspFrHistory.length >= HISTORY ? [...suspFrHistory.slice(1), sFr] : [...suspFrHistory, sFr];
      suspRlHistory = suspRlHistory.length >= HISTORY ? [...suspRlHistory.slice(1), sRl] : [...suspRlHistory, sRl];
      suspRrHistory = suspRrHistory.length >= HISTORY ? [...suspRrHistory.slice(1), sRr] : [...suspRrHistory, sRr];

      // Yaw seismometer history
      const yVal = p.yaw;
      yawHistory = yawHistory.length >= HISTORY ? [...yawHistory.slice(1), yVal] : [...yawHistory, yVal];

      // Boost history
      const bVal = p.boost;
      boostHistory = boostHistory.length >= HISTORY ? [...boostHistory.slice(1), bVal] : [...boostHistory, bVal];
    }, 200);
    return () => { if (sparkInterval) clearInterval(sparkInterval); };
  });

  function suspPath(data: number[], w = 100, h = 40): string {
    if (data.length < 2) return '';
    const pts = data.map((v, i) => {
      const x = (i / (data.length - 1)) * w;
      const val = Math.min(Math.max(v, 0), 1);
      const y = h - val * h;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    return 'M' + pts.join('L');
  }

  function suspFill(data: number[], w = 100, h = 40): string {
    if (data.length < 2) return '';
    const line = suspPath(data, w, h);
    return line + `L${w},${h}L0,${h}Z`;
  }

  function yawPath(data: number[], w = 100, h = 40): string {
    if (data.length < 2) return '';
    const pts = data.map((v, i) => {
      // Map yaw radians (-PI to +PI) to width (0 to w)
      const norm = (v + Math.PI) / (2 * Math.PI); // 0 to 1
      const x = norm * w;
      // Map time index to height (newest at top y=0, oldest at bottom y=h)
      const y = h - (i / (data.length - 1)) * h;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    return 'M' + pts.join('L');
  }

  function getHeadingLabel(deg: number): string {
    const directions = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
    const index = Math.round(((deg % 360) / 45)) % 8;
    return directions[index];
  }

  function sparkPath(data: number[], w = 100, h = 28): string {
    if (data.length < 2) return '';
    const max = Math.max(...data, 1);
    const pts = data.map((v, i) => {
      const x = (i / (data.length - 1)) * w;
      const y = h - (v / max) * h;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    return 'M' + pts.join('L');
  }

  function sparkFill(data: number[], w = 100, h = 28): string {
    if (data.length < 2) return '';
    const line = sparkPath(data, w, h);
    return line + `L${w},${h}L0,${h}Z`;
  }
  
  let carLabel = $derived(pkt ? carName(pkt.carOrdinal) : 'Waiting for Telemetry...');
  let classLabel = $derived(pkt ? (CAR_CLASS_LABELS[pkt.carClass] ?? '?') : '—');
  let piLabel = $derived(pkt ? String(pkt.carPi) : '—');
  let driveLabel = $derived(pkt ? (DRIVETRAIN_LABELS[pkt.drivetrainType] ?? '?') : '—');

  function padClock(sec: number | undefined) {
    if (!sec || sec < 0) return '--:--.---';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    const msPart = Math.floor((sec % 1) * 1000);
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${msPart.toString().padStart(3, '0')}`;
  }
</script>

<svelte:head>
  <script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600&family=JetBrains+Mono:wght@500&family=Space+Grotesk:wght@500;600;700&display=swap" rel="stylesheet" />
  <link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap" rel="stylesheet" />
</svelte:head>

<style>
  :global(:root) {
    --bg-body:    #030712;
    --bg-panel:   #060c14;
    --bg-card:    #080e18;
    --bg-elevated:#0d1420;
    --bg-track:   #151e2e;
    --bd-dim:     #131d2e;
    --bd-subtle:  #1e2a3a;
    --bd-muted:   #252f42;
    --bd-strong:  #2a3a50;
    --tx-hi:      #f9fafb;
    --tx-mid:     #e5e7eb;
    --tx-lo:      #9ca3af;
    --tx-dim:     #6b7280;
    --tx-xdim:    #4b5563;
    --tx-ghost:   #374151;
    --ac:         #3b82f6;
    --ac-dim:     #1e3a5f;
  }
  :global([data-theme="cobalt2"]) {
    --bg-body:    #122738;
    --bg-panel:   #163448;
    --bg-card:    #193549;
    --bg-elevated:#1e4060;
    --bg-track:   #1a3b58;
    --bd-dim:     #1f4e6a;
    --bd-subtle:  #235a7a;
    --bd-muted:   #2a6d91;
    --bd-strong:  #337ba0;
    --tx-hi:      #ffffff;
    --tx-mid:     #e1efff;
    --tx-lo:      #9acfdf;
    --tx-dim:     #7eb8d4;
    --tx-xdim:    #5a96b8;
    --tx-ghost:   #3d7a9c;
    --ac:         #ffc600;
    --ac-dim:     #7a5e00;
  }
  :global(body) { background-color: #0c0e12 !important; color: #e2e2e8 !important; }
  .bg-background { background-color: #0c0e12 !important; }
  .text-on-surface { color: #e2e2e8 !important; }
  .text-on-surface-variant { color: #b9cacb !important; }
  .bg-surface-container { background-color: #111318 !important; }
  .bg-surface-container-highest { background-color: #333539 !important; }
  .border-outline-variant { border-color: #282a2e !important; }
  .border-outline { border-color: #3b494b !important; }
  .text-primary { color: #00dbe9 !important; }
  .bg-primary { background-color: #00dbe9 !important; }
  .text-secondary { color: #e9b3ff !important; }
  .text-error { color: #ffb4ab !important; }
  .bg-error { background-color: #ffb4ab !important; }

  .font-display-hero { font-family: 'Space Grotesk', sans-serif !important; }
  .font-label-mono { font-family: 'JetBrains Mono', monospace !important; }
  .font-body-md { font-family: 'Inter', sans-serif !important; }

  .pro-panel { background: rgba(17, 19, 24, 0.8); border: 1px solid #3b494b; box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4); }
  .glow-cyan { filter: drop-shadow(0 0 8px rgba(0, 219, 233, 0.5)); }
  .speed-arc-bg { stroke: rgba(255, 255, 255, 0.05); stroke-dasharray: 280; stroke-dashoffset: 0; stroke-linecap: round; }
  .speed-arc-active { stroke: #00dbe9; stroke-dasharray: 280; stroke-linecap: round; transition: stroke-dashoffset 0.1s linear; }
  .hud-scanline { background: linear-gradient(rgba(18, 16, 16, 0) 50%, rgba(0, 0, 0, 0.15) 50%); background-size: 100% 2px; pointer-events: none; }
  .mini-graph-path { fill: none; stroke: #00dbe9; stroke-width: 1.5; vector-effect: non-scaling-stroke; }
  .mini-graph-bg { fill: rgba(0, 219, 233, 0.05); }
  .data-table-row { border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
  .text-label { font-size: 11px; }

  .pro-shell {
    position: fixed;
    inset: 0;
    background: #000;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
  }

  .pro-root {
    width: 100%;
    height: 100%;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background-color: #0c0e12;
    color: #e2e2e8;
    font-family: 'Inter', sans-serif;
    user-select: none;
    flex-shrink: 0;
  }
</style>

<svelte:window bind:innerWidth bind:innerHeight />

<div class="pro-shell">
<div class="pro-root">
    <div class="hud-scanline absolute inset-0 z-[100] opacity-5 pointer-events-none"></div>
    <!-- Top Navigation Bar -->
    <header
        class="z-50 flex flex-wrap justify-between items-center px-4 md:px-8 py-3 bg-surface-container border-b border-outline-variant gap-4">
        <div class="flex items-center gap-2 md:gap-4 flex-wrap">
            <a href="/" data-sveltekit-reload class="hover:opacity-80 transition-opacity font-display-hero text-lg md:text-xl tracking-tighter text-on-surface font-bold">TELEMETRY<span class="text-primary font-light">PRO</span></a>
            <div class="hidden md:block h-6 w-[1px] bg-outline-variant mx-2"></div>
            <span class="text-xs md:text-sm font-medium opacity-80">{carLabel}</span>
            <div class="flex gap-2 md:ml-4">
                <span class="bg-primary/20 text-primary border border-primary/30 text-[9px] md:text-[10px] px-2 py-0.5 rounded font-bold uppercase">{classLabel} {piLabel}</span>
                <span class="bg-white/5 border border-white/10 text-[9px] md:text-[10px] px-2 py-0.5 rounded font-bold">{driveLabel}</span>
            </div>
        </div>
        <div class="flex items-center gap-4 md:gap-6 flex-wrap">
            <button
                onclick={toggleRecording}
                class="flex items-center gap-2 px-3 py-1.5 rounded font-bold text-[10px] md:text-xs transition-all uppercase tracking-wider {isRecording ? 'bg-white/10 text-error border border-error/50 animate-pulse' : 'bg-error text-black hover:opacity-90'}">
                <span class="material-symbols-outlined text-[14px] md:text-sm">{isRecording ? 'stop' : 'fiber_manual_record'}</span>
                {#if isRecording}
                  STOP · {formatDuration(recordingDuration)}
                {:else}
                  Start Session
                {/if}
            </button>
            <div class="flex items-center gap-3 md:gap-6 border-l border-outline-variant pl-3 md:pl-6">
                <button class="flex items-center gap-1.5 text-on-surface-variant hover:text-primary transition-colors" onclick={() => (showSessions = !showSessions)} title="Sessions">
                    <span class="material-symbols-outlined text-[18px] md:text-[20px]">history</span>
                    <span class="hidden md:inline text-[10px] font-bold uppercase tracking-widest mt-0.5">History</span>
                </button>
                <button class="flex items-center gap-1.5 text-on-surface-variant hover:text-primary transition-colors" onclick={() => (showSettings = true)} title="Settings">
                    <span class="material-symbols-outlined text-[18px] md:text-[20px]">settings</span>
                    <span class="hidden md:inline text-[10px] font-bold uppercase tracking-widest mt-0.5">Settings</span>
                </button>
                <button class="flex items-center gap-1.5 text-on-surface-variant hover:text-primary transition-colors" onclick={toggleFullscreen} title="Fullscreen">
                    <span class="material-symbols-outlined text-[18px] md:text-[20px]">{isFullscreen ? 'fullscreen_exit' : 'fullscreen'}</span>
                    <span class="hidden md:inline text-[10px] font-bold uppercase tracking-widest mt-0.5">Fullscreen</span>
                </button>
                <div class="flex items-center gap-1.5 md:gap-2">
                    <div class="w-1.5 h-1.5 md:w-2 md:h-2 rounded-full {$isConnected ? 'bg-green-500 animate-pulse' : 'bg-red-500'}"></div>
                    <span class="text-[8px] md:text-[10px] font-label-mono text-white/50">{$isConnected ? 'LIVE LINK' : 'DISCONNECTED'}</span>
                </div>
            </div>
        </div>
    </header>
    <main class="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-12 gap-3 p-3 overflow-y-auto lg:overflow-hidden">
        <!-- Left Column: Session & Vehicle Stats -->
        <div class="col-span-1 lg:col-span-3 flex flex-col gap-3 min-h-0 order-2 lg:order-1">
            <!-- Engine & Power Panel with Sparklines -->
            <div class="pro-panel rounded-lg p-3 flex-1 flex flex-col min-h-0 min-h-[400px] lg:min-h-0">
                <div class="flex justify-between items-center mb-2">
                    <h3 class="text-label font-label-mono text-primary uppercase tracking-widest">Engine &amp; Power</h3>
                    <span class="text-label text-white/40">{pkt ? 'LAP ' + (pkt.lapNumber + 1) : 'LAP --'}</span>
                </div>

                <!-- Power -->
                <div class="mb-2">
                    <div class="flex justify-between text-label mb-0.5">
                        <span class="text-white/60 uppercase">Power</span>
                        <span class="font-label-mono text-primary">{pkt ? Math.max(0, Math.round(pkt.power / 745.7)) : 0}<span class="text-white/30 ml-1">HP</span></span>
                    </div>
                    <div class="w-full rounded overflow-hidden" style="height: 2.2vh;">
                        <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="w-full h-full">
                            <path d={sparkFill(powerHistory)} fill="rgba(0,219,233,0.08)" />
                            <path d={sparkPath(powerHistory)} fill="none" stroke="#00dbe9" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                        </svg>
                    </div>
                </div>

                <!-- Torque -->
                <div class="mb-2">
                    <div class="flex justify-between text-label mb-0.5">
                        <span class="text-white/60 uppercase">Torque</span>
                        <span class="font-label-mono text-secondary">{pkt ? Math.max(0, Math.round(pkt.torque)) : 0}<span class="text-white/30 ml-1">N·m</span></span>
                    </div>
                    <div class="w-full rounded overflow-hidden" style="height: 2.2vh;">
                        <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="w-full h-full">
                            <path d={sparkFill(torqueHistory)} fill="rgba(233,179,255,0.08)" />
                            <path d={sparkPath(torqueHistory)} fill="none" stroke="#e9b3ff" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                        </svg>
                    </div>
                </div>

                <!-- RPM -->
                <div class="mb-2">
                    <div class="flex justify-between text-label mb-0.5">
                        <span class="text-white/60 uppercase">Engine</span>
                        <span class="font-label-mono text-white">{pkt ? Math.round(pkt.currentEngineRpm) : 0}<span class="text-white/30 ml-1">RPM</span></span>
                    </div>
                    <div class="w-full rounded overflow-hidden" style="height: 2.2vh;">
                        <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="w-full h-full">
                            <path d={sparkFill(rpmHistory)} fill="rgba(255,255,255,0.04)" />
                            <path d={sparkPath(rpmHistory)} fill="none" stroke="rgba(255,255,255,0.4)" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                        </svg>
                    </div>
                </div>

                <!-- Boost -->
                <div class="mb-2">
                    <div class="flex justify-between text-label mb-0.5">
                        <span class="text-white/60 uppercase">Boost</span>
                        <span class="font-label-mono font-bold" style="color: #ffc600 !important;">{pkt ? pkt.boost.toFixed(1) : "0.0"}<span class="text-white/30 ml-1">PSI</span></span>
                    </div>
                    <div class="w-full rounded overflow-hidden" style="height: 2.2vh;">
                        <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="w-full h-full">
                            <path d={sparkFill(boostHistory)} fill="rgba(255,198,0,0.04)" />
                            <path d={sparkPath(boostHistory)} fill="none" stroke="#ffc600" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                        </svg>
                    </div>
                </div>

                <!-- Direction (Seismometer) -->
                <div class="mb-2">
                    <div class="flex justify-between text-label mb-0.5">
                        <span class="text-white/60 uppercase">Direction (Seismometer)</span>
                        <span class="font-label-mono text-primary">{pkt ? Math.round(((pkt.yaw * 180) / Math.PI + 360) % 360) : 0}° <span class="text-white/40 ml-0.5 uppercase">{pkt ? getHeadingLabel(((pkt.yaw * 180) / Math.PI + 360) % 360) : '—'}</span></span>
                    </div>
                    <div class="w-full rounded overflow-hidden relative border border-outline-variant bg-white/5" style="height: 5.5vh;">
                        <svg viewBox="0 0 100 40" preserveAspectRatio="none" class="w-full h-full">
                            <!-- Grid Labels -->
                            <text x="25" y="8" fill="rgba(255,255,255,0.2)" font-size="5" text-anchor="middle" font-family="monospace">W</text>
                            <text x="50" y="8" fill="rgba(255,255,255,0.3)" font-size="5" text-anchor="middle" font-family="monospace">N</text>
                            <text x="75" y="8" fill="rgba(255,255,255,0.2)" font-size="5" text-anchor="middle" font-family="monospace">E</text>

                            <!-- Grid Lines -->
                            <line x1="25" y1="0" x2="25" y2="40" stroke="rgba(255,255,255,0.08)" stroke-width="0.5" stroke-dasharray="2,2"></line>
                            <line x1="50" y1="0" x2="50" y2="40" stroke="rgba(255,255,255,0.15)" stroke-width="0.5" stroke-dasharray="2,2"></line>
                            <line x1="75" y1="0" x2="75" y2="40" stroke="rgba(255,255,255,0.08)" stroke-width="0.5" stroke-dasharray="2,2"></line>

                            <!-- Wave Path -->
                            <path d={yawPath(yawHistory)} fill="none" stroke="#00dbe9" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                        </svg>
                    </div>
                </div>

                <!-- Grip Loss -->
                <div class="mt-auto pt-2 border-t border-white/5">
                    <div class="flex justify-between items-end mb-1">
                        <span class="text-label text-white/40 uppercase">Grip Loss</span>
                        <span class="text-label font-bold text-error">{pkt ? Math.round(Math.min(100, ((Math.abs(pkt.tireSlipRatioFl) + Math.abs(pkt.tireSlipRatioFr) + Math.abs(pkt.tireSlipRatioRl) + Math.abs(pkt.tireSlipRatioRr)) / 4) * 50)) : 0}%</span>
                    </div>
                    <div class="w-full bg-white/5 rounded-full overflow-hidden" style="height:3px;">
                        <div class="h-full bg-error transition-all duration-75" style="width: {pkt ? Math.min(100, ((Math.abs(pkt.tireSlipRatioFl) + Math.abs(pkt.tireSlipRatioFr) + Math.abs(pkt.tireSlipRatioRl) + Math.abs(pkt.tireSlipRatioRr)) / 4) * 50) : 0}%;"></div>
                    </div>
                </div>
            </div>
            <!-- G-Force & Orientation -->
            <div class="pro-panel rounded-lg p-3 flex gap-4" style="flex-shrink:0; height: 10.5rem;">
                <div class="flex-1 flex flex-col items-center">
                    <span class="text-[9px] font-label-mono text-white/40 mb-2 uppercase">G-Force</span>
                    <div
                        class="w-24 h-24 rounded-full border border-outline-variant relative flex items-center justify-center bg-white/5 overflow-hidden">
                        <div class="absolute w-full h-[1px] bg-white/10"></div>
                        <div class="absolute h-full w-[1px] bg-white/10"></div>
                        <div class="w-2 h-2 bg-primary rounded-full absolute shadow-[0_0_8px_#00dbe9] transition-all -translate-x-1/2 -translate-y-1/2"
                            style="left: {pkt ? Math.min(Math.max((pkt.accelX/9.81)/2, -0.5), 0.5)*100 + 50 : 50}%; top: {pkt ? Math.min(Math.max((-pkt.accelZ/9.81)/2, -0.5), 0.5)*100 + 50 : 50}%;"></div>
                    </div>
                    <div class="mt-2 text-[10px] font-label-mono text-center">
                        LAT <span class="text-white">{pkt ? (Math.abs(pkt.accelX/9.81)).toFixed(2) : "0.00"}G</span> | LNG <span class="text-white">{pkt ? (Math.abs(pkt.accelZ/9.81)).toFixed(2) : "0.00"}G</span>
                    </div>
                </div>
                <div class="w-[1px] bg-outline-variant h-full"></div>
                <div class="flex-1 flex flex-col items-center">
                    <span class="text-[9px] font-label-mono text-white/40 mb-2 uppercase">Angle</span>
                    <div
                        class="w-24 h-24 rounded-full border border-outline-variant relative overflow-hidden bg-white/5 flex items-center justify-center">
                        <div class="w-16 h-[2px] bg-primary/40 relative transition-all" style="transform: rotate({pkt ? pkt.roll : 0}deg);">
                            <div class="absolute -top-1 left-1/2 -translate-x-1/2 w-1 h-3 bg-primary transition-all" style="transform: translateY({pkt ? Math.min(Math.max(pkt.pitch, -10), 10) : 0}px);"></div>
                        </div>
                    </div>
                    <div class="mt-2 text-[10px] font-label-mono text-center">
                        PITCH <span class="text-white">{pkt ? pkt.pitch.toFixed(1) : "0.0"}°</span> | ROLL <span class="text-white">{pkt ? pkt.roll.toFixed(1) : "0.0"}°</span>
                    </div>
                </div>
            </div>
        </div>
        <!-- Center Column: Gauges -->
        <div class="col-span-1 lg:col-span-6 flex flex-col gap-3 min-h-0 order-1 lg:order-2 min-h-[500px] lg:min-h-0">
            <div class="pro-panel rounded-lg flex-1 relative flex flex-col items-center justify-center overflow-hidden">
                <!-- Gauge Style Selector -->
                <div class="absolute top-3 right-3 flex gap-1 z-20">
                    <button 
                        onclick={() => selectGaugeStyle('arc')} 
                        class="px-2 py-0.5 text-[9px] rounded font-label-mono font-bold uppercase transition-all {gaugeStyle === 'arc' ? 'bg-primary text-black font-semibold' : 'bg-white/5 text-white/40 hover:bg-white/10 hover:text-white'}">
                        Arc
                    </button>
                    <button 
                        onclick={() => selectGaugeStyle('digital')} 
                        class="px-2 py-0.5 text-[9px] rounded font-label-mono font-bold uppercase transition-all {gaugeStyle === 'digital' ? 'bg-primary text-black font-semibold' : 'bg-white/5 text-white/40 hover:bg-white/10 hover:text-white'}">
                        Digital
                    </button>
                    <button 
                        onclick={() => selectGaugeStyle('analog')} 
                        class="px-2 py-0.5 text-[9px] rounded font-label-mono font-bold uppercase transition-all {gaugeStyle === 'analog' ? 'bg-primary text-black font-semibold' : 'bg-white/5 text-white/40 hover:bg-white/10 hover:text-white'}">
                        Analog
                    </button>
                </div>

                <!-- Background Decoration (Only for arc and analog) -->
                {#if gaugeStyle !== 'digital'}
                <div class="absolute inset-0 opacity-10 flex items-center justify-center pointer-events-none">
                    <div class="w-[500px] h-[500px] border-[40px] border-white rounded-full"></div>
                </div>
                {/if}

                <!-- Central Gauge Content -->
                <div class="relative z-10 w-full flex flex-col items-center justify-center">
                    {#if gaugeStyle === 'arc'}
                        <!-- Gauge Stats Left -->
                        <div class="absolute left-12 top-1/2 -translate-y-1/2 space-y-8">
                            <div>
                                <span class="text-[10px] font-label-mono text-white/40 uppercase block mb-1">Delta</span>
                                <span class="text-2xl font-bold text-white/30 font-label-mono">--.---</span>
                            </div>
                            <div>
                                <span class="text-[10px] font-label-mono text-white/40 uppercase block mb-1">Fuel</span>
                                <span class="text-2xl font-bold text-white font-label-mono">{pkt ? (pkt.fuel*100).toFixed(0) : 0}%</span>
                                <div class="w-24 h-1 bg-white/10 rounded-full mt-1">
                                    <div class="h-full bg-primary" style="width: {pkt ? (pkt.fuel*100) : 0}%;"></div>
                                </div>
                            </div>
                        </div>

                        <!-- Speedometer -->
                        <div class="relative w-[340px] h-[340px]">
                            <svg class="w-full h-full -rotate-140" viewBox="0 0 100 100">
                                <circle class="speed-arc-bg" cx="50" cy="50" fill="none" r="45" stroke-width="4"></circle>
                                <circle class="speed-arc-active glow-cyan" cx="50" cy="50" fill="none" r="45"
                                    stroke-width="4" style="stroke-dashoffset: {280 - (rpmPct/100)*200};"></circle>
                            </svg>
                            <div class="absolute inset-0 flex flex-col items-center justify-center">
                                <span class="text-8xl font-display-hero font-bold tracking-tighter leading-none">{speed}</span>
                                <span class="text-xs font-label-mono tracking-[0.4em] text-white/40 font-bold -mt-2">{unit}</span>
                                <div class="mt-6 flex items-center gap-4">
                                    <div
                                        class="w-12 h-12 flex items-center justify-center bg-primary/10 border border-primary/30 rounded-lg">
                                        <span class="text-3xl font-display-hero font-bold text-primary">{pkt?.gear === 0 ? "R" : pkt?.gear === 11 ? "N" : (pkt?.gear || "-")}</span>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Gauge Stats Right -->
                        <div class="absolute right-12 top-1/2 -translate-y-1/2 space-y-8 text-right">
                            <div>
                                <span class="text-[10px] font-label-mono text-white/40 uppercase block mb-1">Car Class</span>
                                <span class="text-xl font-bold font-label-mono text-secondary">{pkt ? String.fromCharCode(pkt.carClass) : "-"}</span>
                                <span class="text-xs font-bold mt-1 block">PI: {pkt ? pkt.carPi : "---"}</span>
                            </div>
                            <div>
                                <span class="text-[10px] font-label-mono text-white/40 uppercase block mb-1">RPM</span>
                                <span class="text-2xl font-bold text-white font-label-mono">{rpm.toLocaleString()}</span>
                            </div>
                        </div>
                    {:else if gaugeStyle === 'digital'}
                        <div class="w-[480px] flex flex-col items-center justify-center py-6 px-8 bg-black/40 border border-outline-variant rounded-xl backdrop-blur-md">
                            <!-- RPM Bar with segments -->
                            <div class="w-full mb-6">
                                <div class="flex justify-between text-[9px] font-label-mono text-white/40 mb-1">
                                    <span>0 RPM</span>
                                    <span class="text-primary font-bold">{rpm.toLocaleString()} RPM</span>
                                    <span>{pkt ? Math.round(pkt.engineMaxRpm) : 8000}</span>
                                </div>
                                <!-- Progress bar divided into segments -->
                                <div class="w-full h-6 bg-white/5 border border-outline-variant rounded relative overflow-hidden flex gap-[2px] p-[2px]">
                                    {#each Array.from({ length: 20 }) as _, i}
                                        {@const segmentVal = (i / 20) * 100}
                                        {@const isActive = rpmPct >= segmentVal}
                                        {@const isRedline = i >= 16}
                                        <div class="flex-1 h-full rounded-sm transition-all duration-75 {isActive ? (isRedline ? 'bg-error shadow-[0_0_8px_#ffb4ab]' : 'bg-primary shadow-[0_0_8px_#00dbe9]') : 'bg-white/5'}"></div>
                                    {/each}
                                    {#if rpmPct >= 92}
                                        <div class="absolute inset-0 bg-error/20 border border-error animate-pulse flex items-center justify-center">
                                            <span class="text-[10px] font-bold text-error tracking-wider uppercase">SHIFT LIGHT</span>
                                        </div>
                                    {/if}
                                </div>
                            </div>

                            <!-- Central Display -->
                            <div class="flex items-center gap-8 justify-center w-full">
                                <!-- Speed -->
                                <div class="text-center flex-1">
                                    <span class="text-8xl font-display-hero font-bold tracking-tighter leading-none">{speed}</span>
                                    <span class="text-xs font-label-mono tracking-[0.4em] text-white/40 font-bold block -mt-1">{unit}</span>
                                </div>
                                
                                <!-- Divider -->
                                <div class="w-[1px] h-20 bg-outline-variant flex-none"></div>

                                <!-- Gear -->
                                <div class="flex flex-col items-center flex-1">
                                    <span class="text-[9px] font-label-mono text-white/40 uppercase mb-1">GEAR</span>
                                    <div class="w-20 h-20 flex items-center justify-center bg-primary/10 border-2 border-primary/50 rounded-xl shadow-[0_0_15px_rgba(0,219,233,0.15)]">
                                        <span class="text-6xl font-display-hero font-bold text-primary">{pkt?.gear === 0 ? "R" : pkt?.gear === 11 ? "N" : (pkt?.gear || "-")}</span>
                                    </div>
                                </div>
                            </div>
                            
                            <!-- Bottom Stats (Fuel & Car class) -->
                            <div class="w-full grid grid-cols-2 gap-4 mt-6 pt-4 border-t border-white/5">
                                <div class="flex flex-col gap-1">
                                    <div class="flex justify-between text-[9px] font-label-mono text-white/40">
                                        <span>FUEL</span>
                                        <span class="text-white font-bold">{pkt ? (pkt.fuel*100).toFixed(0) : 0}%</span>
                                    </div>
                                    <div class="w-full h-1.5 bg-white/10 rounded-full overflow-hidden">
                                        <div class="h-full bg-primary" style="width: {pkt ? (pkt.fuel*100) : 0}%;"></div>
                                    </div>
                                </div>
                                <div class="flex justify-between items-center text-[10px] font-label-mono text-white/40">
                                    <div>
                                        <span>CLASS: </span>
                                        <span class="text-secondary font-bold font-label-mono">{pkt ? String.fromCharCode(pkt.carClass) : "-"} {pkt ? pkt.carPi : "---"}</span>
                                    </div>
                                    <div>
                                        <span>LAP: </span>
                                        <span class="text-white font-bold">{pkt ? (pkt.lapNumber + 1) : "--"}</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                    {:else if gaugeStyle === 'analog'}
                        <div class="relative w-[340px] h-[340px]">
                            <!-- Dial face -->
                            <svg class="w-full h-full" viewBox="0 0 100 100">
                                <!-- Outer circle -->
                                <circle cx="50" cy="50" r="45" fill="none" stroke="rgba(255,255,255,0.05)" stroke-width="1.5"></circle>
                                <!-- RPM Ticks (0 to 10) -->
                                {#each Array.from({ length: 11 }) as _, i}
                                    {@const angle = -140 + i * 28}
                                    {@const rad = (angle - 90) * Math.PI / 180}
                                    {@const x1 = 50 + 40 * Math.cos(rad)}
                                    {@const y1 = 50 + 40 * Math.sin(rad)}
                                    {@const x2 = 50 + 45 * Math.cos(rad)}
                                    {@const y2 = 50 + 45 * Math.sin(rad)}
                                    {@const tx = 50 + 34 * Math.cos(rad)}
                                    {@const ty = 50 + 34 * Math.sin(rad)}
                                    <line x1={x1} y1={y1} x2={x2} y2={y2} stroke={i >= 8 ? '#ffb4ab' : 'rgba(255,255,255,0.3)'} stroke-width="1"></line>
                                    <text x={tx} y={ty} fill={i >= 8 ? '#ffb4ab' : 'rgba(255,255,255,0.5)'} font-size="5" font-family="Space Grotesk" font-weight="bold" text-anchor="middle" dominant-baseline="middle">{i}</text>
                                {/each}
                                <!-- Redline Arc -->
                                <path d="M 75.7 82.3 A 45 45 0 0 0 84.5 21.1" fill="none" stroke="#ffb4ab" stroke-width="3" stroke-opacity="0.3"></path>
                                <!-- Needle -->
                                <line x1="50" y1="50" x2={50 + 42 * Math.cos((needleAngle - 90) * Math.PI / 180)} y2={50 + 42 * Math.sin((needleAngle - 90) * Math.PI / 180)} stroke="#00dbe9" stroke-width="2" stroke-linecap="round" class="glow-cyan"></line>
                                <!-- Center Cap -->
                                <circle cx="50" cy="50" r="4" fill="#00dbe9"></circle>
                                <circle cx="50" cy="50" r="1.5" fill="#0c0e12"></circle>
                            </svg>
                            <!-- Digital readout inside the dial -->
                            <div class="absolute inset-0 flex flex-col items-center justify-center pt-24">
                                <span class="text-5xl font-display-hero font-bold tracking-tighter leading-none">{speed}</span>
                                <span class="text-[9px] font-label-mono tracking-widest text-white/40 font-bold">{unit}</span>
                                <span class="text-xl font-display-hero font-bold text-primary bg-primary/10 border border-primary/20 rounded px-2 mt-2">{pkt?.gear === 0 ? "R" : pkt?.gear === 11 ? "N" : (pkt?.gear || "-")}</span>
                            </div>
                        </div>
                    {/if}
                </div>

                <!-- Bottom Bar: Controls & Pedals -->
                <div class="absolute bottom-6 w-full px-12 flex items-end justify-between">
                    <div class="flex-1 space-y-3 max-w-[200px]">
                        <div class="flex items-center gap-3">
                            <span class="text-[9px] font-label-mono text-white/40 w-8">THR</span>
                            <div class="flex-1 h-1 bg-white/5 rounded-full overflow-hidden">
                                <div class="h-full bg-primary" style="width: {pkt ? (pkt.throttle/255)*100 : 0}%;"></div>
                            </div>
                        </div>
                        <div class="flex items-center gap-3">
                            <span class="text-[9px] font-label-mono text-white/40 w-8">BRK</span>
                            <div class="flex-1 h-1 bg-white/5 rounded-full overflow-hidden">
                                <div class="h-full bg-error" style="width: {pkt ? (pkt.brake/255)*100 : 0}%;"></div>
                            </div>
                        </div>
                        <div class="flex items-center gap-3">
                            <span class="text-[9px] font-label-mono text-white/40 w-8">CLT</span>
                            <div class="flex-1 h-1 bg-white/5 rounded-full overflow-hidden">
                                <div class="h-full bg-white/40" style="width: {pkt ? (pkt.clutch/255)*100 : 0}%;"></div>
                            </div>
                        </div>
                        <div class="flex items-center gap-3">
                            <span class="text-[9px] font-label-mono text-white/40 w-8">HBK</span>
                            <div class="flex-1 h-1 bg-white/5 rounded-full overflow-hidden">
                                <div class="h-full bg-secondary" style="width: {pkt ? (pkt.handbrake/255)*100 : 0}%;"></div>
                            </div>
                        </div>
                    </div>
                    <div class="flex gap-4">
                        <div class="flex flex-col items-center">
                            <span class="text-[9px] font-label-mono text-white/40 uppercase mb-1">Steering</span>
                            <div
                                class="w-12 h-12 rounded-full border-2 border-white/10 relative flex items-center justify-center">
                                <div class="w-[2px] h-8 bg-primary rounded-full" style="transform: rotate({pkt ? pkt.steer * 90 : 0}deg);"></div>
                            </div>
                        </div>
                        <div class="flex flex-col items-center">
                            <span class="text-[9px] font-label-mono text-white/40 uppercase mb-1">Boost</span>
                            <div class="w-12 h-12 flex items-center justify-center border-2 border-white/10 rounded-lg">
                                <span class="text-sm font-bold text-secondary">{pkt ? pkt.boost.toFixed(1) : "0.0"}</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <!-- Right Column: Suspension & Tires -->
        <div class="col-span-1 lg:col-span-3 flex flex-col gap-3 min-h-0 order-3 lg:order-3">
            <!-- Tire Grid -->
            <div class="pro-panel rounded-lg p-4 grid grid-cols-2 gap-4 flex-none h-auto">
                <!-- FL -->
                <div class="space-y-2">
                    <div class="flex justify-between text-[9px] font-label-mono text-white/40 uppercase">
                        <span>FL</span>
                    </div>
                    <div class="bg-white/5 p-2 rounded border border-outline-variant">
                        <div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempFl) : "-"}<span class="text-[10px] text-white/40 font-normal">°C</span>
                        </div>
                        <div class="text-sm text-primary font-label-mono">{pkt ? Math.max(28.0, 28.0 + (pkt.tireTempFl - 60) * 0.1).toFixed(1) : "--"} <span
                                class="text-[8px] opacity-60 uppercase">psi</span></div>
                    </div>
                </div>
                <!-- FR -->
                <div class="space-y-2">
                    <div class="flex justify-between text-[9px] font-label-mono text-white/40 uppercase">
                        <span>FR</span>
                    </div>
                    <div class="bg-white/5 p-2 rounded border border-outline-variant">
                        <div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempFr) : "-"}<span class="text-[10px] text-white/40 font-normal">°C</span>
                        </div>
                        <div class="text-sm text-primary font-label-mono">{pkt ? Math.max(28.0, 28.0 + (pkt.tireTempFr - 60) * 0.1).toFixed(1) : "--"} <span
                                class="text-[8px] opacity-60 uppercase">psi</span></div>
                    </div>
                </div>
                <!-- RL -->
                <div class="space-y-2">
                    <div class="flex justify-between text-[9px] font-label-mono text-white/40 uppercase">
                        <span>RL</span>
                    </div>
                    <div class="bg-white/5 p-2 rounded border border-outline-variant">
                        <div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempRl) : "-"}<span class="text-[10px] text-white/40 font-normal">°C</span>
                        </div>
                        <div class="text-sm text-primary font-label-mono">{pkt ? Math.max(28.0, 28.0 + (pkt.tireTempRl - 60) * 0.1).toFixed(1) : "--"} <span
                                class="text-[8px] opacity-60 uppercase">psi</span></div>
                    </div>
                </div>
                <!-- RR -->
                <div class="space-y-2">
                    <div class="flex justify-between text-[9px] font-label-mono text-white/40 uppercase">
                        <span>RR</span>
                    </div>
                    <div class="bg-white/5 p-2 rounded border border-outline-variant">
                        <div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempRr) : "-"}<span class="text-[10px] text-white/40 font-normal">°C</span>
                        </div>
                        <div class="text-sm text-primary font-label-mono">{pkt ? Math.max(28.0, 28.0 + (pkt.tireTempRr - 60) * 0.1).toFixed(1) : "--"} <span
                                class="text-[8px] opacity-60 uppercase">psi</span></div>
                    </div>
                </div>
            </div>
            <!-- Suspension Travel Real-time Graphs -->
            <div class="pro-panel rounded-lg p-4 flex-1 flex flex-col gap-4">
                <div class="flex justify-between items-center">
                    <h3 class="text-[11px] font-label-mono text-primary uppercase tracking-widest">Suspension Travel</h3>
                    <span class="text-[9px] text-white/40 font-label-mono">FL:{pkt ? pkt.suspensionFl.toFixed(2) : "0.00"} | FR:{pkt ? pkt.suspensionFr.toFixed(2) : "0.00"}</span>
                </div>
                <div class="grid grid-cols-2 gap-4 flex-1">
                    <div class="flex flex-col gap-1">
                        <span class="text-[8px] font-label-mono text-white/30">FL TRAVEL</span>
                        <div class="flex-1 bg-white/5 border border-outline-variant relative overflow-hidden rounded h-12">
                            <svg class="absolute inset-0 w-full h-full" preserveAspectRatio="none" viewBox="0 0 100 40">
                                <path class="mini-graph-bg" d={suspFill(suspFlHistory)} fill="rgba(0,219,233,0.08)" />
                                <path class="mini-graph-path" d={suspPath(suspFlHistory)} fill="none" stroke="#00dbe9" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                            </svg>
                        </div>
                    </div>
                    <div class="flex flex-col gap-1">
                        <span class="text-[8px] font-label-mono text-white/30">FR TRAVEL</span>
                        <div class="flex-1 bg-white/5 border border-outline-variant relative overflow-hidden rounded h-12">
                            <svg class="absolute inset-0 w-full h-full" preserveAspectRatio="none" viewBox="0 0 100 40">
                                <path class="mini-graph-bg" d={suspFill(suspFrHistory)} fill="rgba(0,219,233,0.08)" />
                                <path class="mini-graph-path" d={suspPath(suspFrHistory)} fill="none" stroke="#00dbe9" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                            </svg>
                        </div>
                    </div>
                    <div class="flex flex-col gap-1">
                        <span class="text-[8px] font-label-mono text-white/30">RL TRAVEL</span>
                        <div class="flex-1 bg-white/5 border border-outline-variant relative overflow-hidden rounded h-12">
                            <svg class="absolute inset-0 w-full h-full" preserveAspectRatio="none" viewBox="0 0 100 40">
                                <path class="mini-graph-bg" d={suspFill(suspRlHistory)} fill="rgba(0,219,233,0.08)" />
                                <path class="mini-graph-path" d={suspPath(suspRlHistory)} fill="none" stroke="#00dbe9" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                            </svg>
                        </div>
                    </div>
                    <div class="flex flex-col gap-1">
                        <span class="text-[8px] font-label-mono text-white/30">RR TRAVEL</span>
                        <div class="flex-1 bg-white/5 border border-outline-variant relative overflow-hidden rounded h-12">
                            <svg class="absolute inset-0 w-full h-full" preserveAspectRatio="none" viewBox="0 0 100 40">
                                <path class="mini-graph-bg" d={suspFill(suspRrHistory)} fill="rgba(0,219,233,0.08)" />
                                <path class="mini-graph-path" d={suspPath(suspRrHistory)} fill="none" stroke="#00dbe9" stroke-width="1.5" vector-effect="non-scaling-stroke" />
                            </svg>
                        </div>
                    </div>
                </div>
            </div>
            <!-- Live Track Map -->
            <div class="pro-panel rounded-lg h-[160px] flex-none overflow-hidden relative flex flex-col">
                <div class="absolute inset-0 z-0">
                    <LiveTrackMap />
                </div>
                <div class="absolute top-2 left-3 flex items-center justify-between w-[calc(100%-24px)] z-20 pointer-events-none">
                    <span class="text-[9px] font-label-mono text-primary bg-primary/10 px-2 py-0.5 rounded border border-primary/20 uppercase shadow-md backdrop-blur">Live Circuit</span>
                </div>
            </div>
        </div>
    </main>
    <!-- Footer: Lap History & Timing -->
    <footer
        class="bg-surface-container-highest border-t border-outline-variant px-4 md:px-8 py-4 flex flex-wrap items-center justify-between gap-4">
        <div class="flex flex-wrap gap-6 md:gap-12 w-full md:w-auto">
            <div class="flex flex-col flex-1 md:flex-none">
                <span class="text-[9px] font-label-mono text-white/40 uppercase mb-1">Current Lap</span>
                <span class="text-xl md:text-2xl font-bold font-label-mono">{padClock(pkt?.currentRaceTime)}</span>
            </div>
            <div class="flex flex-col flex-1 md:flex-none">
                <span class="text-[9px] font-label-mono text-white/40 uppercase mb-1">Last Lap</span>
                <span class="text-xl md:text-2xl font-bold font-label-mono text-white/80">{padClock(pkt?.lastLap)}</span>
            </div>
            <div class="flex flex-col flex-1 md:flex-none">
                <span class="text-[9px] font-label-mono text-white/40 uppercase mb-1">Session Best</span>
                <span class="text-xl md:text-2xl font-bold font-label-mono text-secondary">{padClock(pkt?.bestLap)}</span>
            </div>
        </div>
        <div class="flex items-center justify-between md:justify-end gap-4 md:gap-8 w-full md:w-auto md:pr-4">
            <div class="text-left md:text-right">
                <span class="text-[9px] font-label-mono text-white/40 uppercase block">Total Distance</span>
                <span class="text-sm font-bold">{pkt ? (pkt.distanceTraveled / 1609.34).toFixed(1) : "0.0"} <span class="text-[10px] opacity-50">MI</span></span>
            </div>
            <button class="flex items-center gap-2 px-4 md:px-6 py-2 bg-white/5 border border-outline hover:bg-white/10 transition-all rounded-lg text-xs font-bold uppercase tracking-wider" onclick={() => (showSessions = true)}>
                <span class="material-symbols-outlined text-sm">history</span>
                <span class="hidden md:inline">Session History</span>
                <span class="inline md:hidden">History</span>
            </button>
        </div>
    </footer>
</div>
</div>

{#if showSessions}
  <SessionDrawer
    onClose={() => (showSessions = false)}
    onOpen={(session) => (viewerSession = session)}
  />
{/if}

{#if viewerSession}
  <SessionViewer
    session={viewerSession}
    useMph={$settings?.useMph ?? true}
    onClose={() => (viewerSession = null)}
  />
{/if}

<ReplayBar />

{#if showSettings}
  <SettingsModal onClose={() => (showSettings = false)} />
{/if}

<TelemetryConfirmation onOpenSettings={() => (showSettings = true)} />
