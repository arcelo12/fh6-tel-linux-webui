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
  import '$lib/pro-dashboard.css';
  
  // Widget Engine
  import WidgetEngine from '$lib/components/WidgetEngine.svelte';
  import EnginePowerWidget from '$lib/components/widgets/EnginePowerWidget.svelte';
  import GForceWidget from '$lib/components/widgets/GForceWidget.svelte';
  import SuspensionWidget from '$lib/components/widgets/SuspensionWidget.svelte';
  import TransmissionWidget from '$lib/components/widgets/TransmissionWidget.svelte';
  import VehicleInfoWidget from '$lib/components/widgets/VehicleInfoWidget.svelte';
  import RawDataWidget from '$lib/components/widgets/RawDataWidget.svelte';
  import HUDWidget from '$lib/components/widgets/HUDWidget.svelte';
  import TireWidget from '$lib/components/widgets/TireWidget.svelte';
  
  const WIDGETS = {
    hud: { name: 'Dashboard HUD', component: HUDWidget },
    engine: { name: 'Engine & Power', component: EnginePowerWidget },
    gforce: { name: 'G-Force & Angle', component: GForceWidget },
    suspension: { name: 'Suspension', component: SuspensionWidget },
    transmission: { name: 'Transmission', component: TransmissionWidget },
    tires: { name: 'Tire Temps', component: TireWidget },
    vehicle: { name: 'Vehicle Info', component: VehicleInfoWidget },
    map: { name: 'Live Track Map', component: LiveTrackMap },
    raw: { name: 'Raw Telemetry Dump', component: RawDataWidget }
  };

  const DEFAULT_LAYOUT = [
    { id: 'engine', x: 0, y: 0, w: 3, h: 5 },
    { id: 'gforce', x: 0, y: 5, w: 3, h: 2 },
    { id: 'hud', x: 3, y: 0, w: 6, h: 7 },
    { id: 'tires', x: 9, y: 0, w: 3, h: 3 },
    { id: 'suspension', x: 9, y: 3, w: 3, h: 2 },
    { id: 'map', x: 9, y: 5, w: 3, h: 2 }
  ];

  let dashboardLayout = $state<any[]>(DEFAULT_LAYOUT);
  let isEditMode = $state(false);
  let layoutLoaded = $state(false);

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

  onMount(() => {
    (async () => {
      try {
        await loadSettings();
        await startTelemetryListener();
        
        // Load saved layout
        const layoutRes = await nodeFetch('/api/user/layout').catch(() => null);
        if (layoutRes?.ok) {
          const layoutData = await layoutRes.json();
          if (layoutData && layoutData.layout && Array.isArray(layoutData.layout) && layoutData.layout.length > 0) {
            dashboardLayout = layoutData.layout;
          }
        }
      } catch (e) {
        console.error(e);
      } finally {
        layoutLoaded = true;
      }

      // Sync recording state from backend
      const res = await nodeFetch('/api/session/status').catch(() => null);
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
      const res = await nodeFetch('/api/session/stop', { method: 'POST' }).catch(() => null);
      if (res?.ok) {
        isRecording = false;
        if (recordingInterval) { clearInterval(recordingInterval); recordingInterval = null; }
        recordingDuration = 0;
      }
    } else {
      const res = await nodeFetch('/api/session/start', { method: 'POST' }).catch(() => null);
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
  
  let carLabel = $derived(pkt ? carName(pkt.carOrdinal) : 'Waiting for Telemetry...');
  let classLabel = $derived(pkt ? (CAR_CLASS_LABELS[pkt.carClass] ?? '?') : '—');
  let piLabel = $derived(pkt ? String(pkt.carPi) : '—');
  let driveLabel = $derived(pkt ? (DRIVETRAIN_LABELS[pkt.drivetrainType] ?? '?') : '—');

  async function handleLayoutChange(newLayout: any[]) {
      // Sync Svelte's local state with latest coordinates to prevent resets on adding/removing widgets
      dashboardLayout = newLayout;
      
      await nodeFetch('/api/user/layout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ layout: newLayout })
      }).catch(() => null);
  }

  function addWidget(id: string) {
      if (dashboardLayout.find(w => w.id === id)) return;
      dashboardLayout = [...dashboardLayout, { id, x: 0, y: 100, w: 3, h: 3 }];
  }

  function removeWidget(id: string) {
      dashboardLayout = dashboardLayout.filter(w => w.id !== id);
      // We must explicitly save it here since we bypassed it above
      nodeFetch('/api/user/layout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ layout: dashboardLayout })
      }).catch(() => null);
  }

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
  <link href="https://cdn.jsdelivr.net/npm/gridstack@12.6.0/dist/gridstack.min.css" rel="stylesheet" />
</svelte:head>

<style>
  /* Page-specific layout classes (not needed by widgets) */
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
        class="relative z-50 flex flex-wrap justify-between items-center px-4 md:px-8 py-3 bg-surface-container border-b border-outline-variant gap-4">
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
                <button class="flex items-center gap-1.5 text-on-surface-variant hover:text-primary transition-colors {isEditMode ? 'text-primary bg-primary/10 rounded px-2' : ''}" onclick={() => (isEditMode = !isEditMode)} title="Edit Dashboard">
                    <span class="material-symbols-outlined text-[18px] md:text-[20px]">{isEditMode ? 'check' : 'dashboard_customize'}</span>
                    <span class="hidden md:inline text-[10px] font-bold uppercase tracking-widest mt-0.5">{isEditMode ? 'Done' : 'Edit'}</span>
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
        
        {#if isEditMode}
        <div class="bg-primary/10 border-b border-primary/20 px-4 py-2 flex items-center gap-4 overflow-x-auto shadow-inner">
            <span class="text-[10px] font-label-mono text-primary uppercase font-bold tracking-widest">Available Widgets:</span>
            {#each Object.entries(WIDGETS) as [id, widget]}
                {#if !dashboardLayout.find(w => w.id === id)}
                    <button class="bg-black/40 border border-primary/30 rounded px-3 py-1 text-[10px] font-label-mono text-white/80 hover:bg-primary hover:text-black transition-all flex-shrink-0" onclick={() => addWidget(id)}>
                        + {widget.name}
                    </button>
                {/if}
            {/each}
        </div>
        {/if}
    </header>
    <main class="flex-1 min-h-0 relative z-0 overflow-y-auto bg-black p-2 md:p-4">
        {#if layoutLoaded}
            {#key dashboardLayout.length}
                <!-- {#key} forces re-mount of WidgetEngine when length changes to re-init gridstack properly -->
                <WidgetEngine 
                    widgets={WIDGETS} 
                    layout={dashboardLayout} 
                    editMode={isEditMode} 
                    onLayoutChange={handleLayoutChange}
                    onRemove={removeWidget}
                />
            {/key}
        {:else}
            <div class="flex items-center justify-center h-full">
                <div class="animate-spin rounded-full h-8 w-8 border-t-2 border-primary"></div>
            </div>
        {/if}
    </main>
    <!-- Footer: Lap History & Timing -->
    <footer
        class="relative z-10 bg-surface-container-highest border-t border-outline-variant px-4 md:px-8 py-4 flex flex-wrap items-center justify-between gap-4">
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
