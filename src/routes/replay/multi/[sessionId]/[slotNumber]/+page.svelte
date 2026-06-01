<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { checkAuth } from '$lib/stores/auth';
  import SessionViewer from '$lib/components/SessionViewer.svelte';
  import type { TelemetryPacket, SessionLap, SessionRow } from '$lib/types';
  import { settings, loadSettings } from '$lib/stores/sessions';

  const sessionId = Number($page.params.sessionId);
  const slotNumber = Number($page.params.slotNumber);

  let sessionData = $state<any>(null);
  let packets = $state<TelemetryPacket[]>([]);
  let loading = $state(true);
  let errorMsg = $state('');

  onMount(async () => {
    const user = await checkAuth();
    if (!user) {
      goto('/login');
      return;
    }

    if (!$settings) await loadSettings();

    try {
      // 1. Fetch History Metadata (find the specific session)
      const resMeta = await fetch('/api/lobby/history');
      if (!resMeta.ok) throw new Error('Failed to fetch history metadata');
      const history = await resMeta.json();
      sessionData = history.find((h: any) => h.id === sessionId && h.slotNumber === slotNumber);

      if (!sessionData) {
        throw new Error('Session not found in your multiplayer history');
      }

      // 2. Fetch the JSON Packets for this specific slot
      const resPackets = await fetch(`/api/export/json?session_id=${sessionId}&slot_number=${slotNumber}`);
      if (!resPackets.ok) throw new Error('Failed to download telemetry packets');
      packets = await resPackets.json();

    } catch (err: any) {
      errorMsg = err.message;
    } finally {
      loading = false;
    }
  });

  // Convert multiplayer history row to SessionRow interface
  let mockSession = $derived<SessionRow | null>(
    sessionData ? {
      id: sessionData.id,
      startedAt: sessionData.startedAt,
      endedAt: sessionData.endedAt,
      carOrdinal: sessionData.carOrdinal,
      carClass: 0,
      carPi: 0,
      bestLap: null,
      packetCount: sessionData.packetCount,
      name: sessionData.name ? `${sessionData.name} (Multiplayer)` : `Room ${sessionData.roomCode}`,
      bookmarked: false
    } : null
  );

</script>

<svelte:head>
  <title>Replay: Multiplayer Session {sessionId}</title>
</svelte:head>

<div class="replay-page">
  {#if loading}
    <div class="center-msg">
      <div class="spinner"></div>
      <p>Loading multiplayer telemetry (Slot {slotNumber})...</p>
    </div>
  {:else if errorMsg}
    <div class="center-msg error">
      <h2>Error</h2>
      <p>{errorMsg}</p>
      <button class="back-btn" onclick={() => goto('/pro')}>Return to Dashboard</button>
    </div>
  {:else if mockSession}
    <SessionViewer 
      session={mockSession} 
      useMph={$settings?.useMph ?? true} 
      onClose={() => goto('/pro')}
      preloadedPackets={packets}
      preloadedLaps={[]} 
    />
  {/if}
</div>

<style>
  .replay-page {
    width: 100vw;
    height: 100vh;
    height: 100dvh;
    background: var(--bg-body);
    color: var(--tx-hi);
  }
  .center-msg {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 1rem;
    padding: 2rem;
    text-align: center;
  }
  .error { color: #ef4444; }
  .spinner {
    width: 40px; height: 40px;
    border: 3px solid rgba(255,255,255,0.1);
    border-top-color: var(--ac);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  .back-btn {
    background: var(--bg-elevated);
    border: 1px solid var(--bd-subtle);
    color: var(--tx-hi);
    padding: 0.5rem 1rem;
    border-radius: 6px;
    cursor: pointer;
    margin-top: 1rem;
  }
  .back-btn:hover { border-color: var(--bd-dim); }
</style>
