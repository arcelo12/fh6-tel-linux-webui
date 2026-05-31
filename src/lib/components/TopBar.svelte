<script lang="ts">
  import { isConnected, displayPacket } from '$lib/stores/telemetry';
  import { carName } from '$lib/car-name';
  import { CAR_CLASS_LABELS, DRIVETRAIN_LABELS } from '$lib/types';
  import { logout, currentUser } from '$lib/stores/auth';
  import { appConfig } from '$lib/stores/config';
  import { goto } from '$app/navigation';

  let { useMph = true, onSettings, onSessions, onLobbies }: {
    useMph: boolean;
    onSettings: () => void;
    onSessions: () => void;
    onLobbies: () => void;
  } = $props();

  let pkt = $derived($displayPacket);
  let connected = $derived($isConnected);
  let carLabel = $derived(pkt ? carName(pkt.carOrdinal) : '—');
  let isUnknown = $derived(pkt ? carLabel.startsWith('Car #') : false);
  let classLabel = $derived(pkt ? (CAR_CLASS_LABELS[pkt.carClass] ?? '?') : '—');
  let piLabel = $derived(pkt ? String(pkt.carPi) : '—');
  let driveLabel = $derived(pkt ? (DRIVETRAIN_LABELS[pkt.drivetrainType] ?? '?') : '—');

  let copied = $state(false);
  let version = 'WebUI';

  async function copyOrdinal() {
    if (!pkt || !isUnknown) return;
    await navigator.clipboard.writeText(String(pkt.carOrdinal));
    copied = true;
    setTimeout(() => { copied = false; }, 1800);
  }
</script>

<header class="topbar">
  <div class="status">
    <span class="dot" class:live={connected}></span>
    <span class="label">{connected ? 'LIVE' : 'WAITING…'}</span>
  </div>

  <div class="car-info">
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <span
      role="button"
      tabindex="0"
      class="car-name"
      class:unknown={isUnknown}
      onclick={copyOrdinal}
      onkeydown={(e) => e.key === 'Enter' && copyOrdinal()}
      title={isUnknown ? `Ordinal: ${pkt?.carOrdinal} — click to copy` : undefined}
    >
      {copied ? 'Copied!' : carLabel}
    </span>
    <span class="badge class-badge" data-class={classLabel}>{classLabel}</span>
    <span class="badge">{piLabel}</span>
    <span class="badge">{driveLabel}</span>
  </div>

  <div class="controls">
    {#if $appConfig?.multiplayer}
      {#if $currentUser}
        <span class="username" title="Logged in as {$currentUser.username}">{$currentUser.username}</span>
        {#if $currentUser.role === 'admin'}
          <a href="/admin" class="nav-btn" title="Admin Dashboard">Admin</a>
        {/if}
        <button class="nav-btn" onclick={async () => { await logout(); goto('/login'); }} title="Sign Out">Sign Out</button>
      {/if}
      <a href="/pro" class="nav-btn pro-btn" title="Pro Mode">Pro Dashboard</a>
      <button class="nav-btn" onclick={onLobbies} title="Multiplayer Lobbies">Lobbies</button>
    {/if}
    <button class="nav-btn" onclick={onSessions} title="Sessions">Sessions</button>
    <button class="nav-btn" onclick={onSettings} title="Settings">Settings</button>
    {#if version}<span class="version">v{version}</span>{/if}
  </div>
</header>

<style>
  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 1rem;
    height: 2.8rem;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--bd-dim);
    flex-shrink: 0;
  }
  .status { display: flex; align-items: center; gap: 0.4rem; }
  .dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: #ef4444;
    transition: background 0.3s;
  }
  .dot.live { background: #22c55e; box-shadow: 0 0 6px #22c55e; }
  .label { font-size: 0.7rem; font-weight: 700; letter-spacing: 0.1em; color: var(--tx-dim); }
  .car-info { display: flex; align-items: center; gap: 0.4rem; min-width: 0; overflow: hidden; }
  .car-name {
    font-size: clamp(0.7rem, 1.6vw, 0.85rem);
    font-weight: 600;
    color: var(--tx-mid);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: clamp(80px, 22vw, 260px);
  }
  .car-name.unknown { color: var(--tx-dim); cursor: copy; }
  .car-name.unknown:hover { color: var(--tx-lo); }
  .badge {
    font-size: clamp(0.55rem, 1.2vw, 0.65rem);
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border: 1px solid var(--bd-muted);
    border-radius: 3px;
    color: var(--tx-lo);
    flex-shrink: 0;
    white-space: nowrap;
  }
  /* Class badge colours are semantic — stay fixed across themes */
  .class-badge[data-class="X"]  { color: #ef4444; border-color: #7f1d1d; }
  .class-badge[data-class="S2"] { color: #f97316; border-color: #7c2d12; }
  .class-badge[data-class="S1"] { color: #eab308; border-color: #713f12; }
  .class-badge[data-class="A"]  { color: #22c55e; border-color: #14532d; }
  .class-badge[data-class="B"]  { color: #3b82f6; border-color: #1e3a5f; }
  .class-badge[data-class="C"]  { color: #a855f7; border-color: #4c1d95; }
  .class-badge[data-class="D"]  { color: var(--tx-lo); border-color: var(--bd-subtle); }
  .controls { display: flex; align-items: center; gap: 0.4rem; }
  .username {
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--tx-lo);
    margin-right: 0.4rem;
    max-width: 110px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .version { font-size: 0.6rem; color: var(--tx-xdim); letter-spacing: 0.03em; padding: 0 0.1rem; }
  
  .nav-btn {
    background: transparent;
    border: 1px solid var(--bd-subtle);
    cursor: pointer;
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--tx-mid);
    padding: 0.35rem 0.65rem;
    border-radius: 4px;
    text-decoration: none;
    line-height: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
  }
  .nav-btn:hover {
    background: var(--bg-elevated);
    color: var(--tx-hi);
    border-color: var(--bd-muted);
  }
  .pro-btn {
    background: var(--ac);
    color: #fff;
    border: none;
    font-weight: 700;
  }
  .pro-btn:hover {
    background: var(--ac);
    color: #fff;
    opacity: 0.9;
    border: none;
  }
</style>
