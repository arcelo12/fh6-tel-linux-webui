<script lang="ts">
  import { onDestroy } from 'svelte';
  import { replay, exitReplay } from '$lib/stores/telemetry';

  const SPEEDS = [0.5, 1, 2, 4];
  let rafId: number | null = null;
  let isLooping = false;

  function clearTimer() {
    isLooping = false;
    if (rafId !== null) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
  }

  onDestroy(clearTimer);

  let carry = 0;
  let lastTime = 0;

  function ensureLoop(playing: boolean, speed: number) {
    clearTimer();
    carry = 0;
    
    if (!playing) return;
    
    isLooping = true;
    lastTime = performance.now();

    function tick(now: number) {
      if (!isLooping) return;
      
      const dt = now - lastTime;
      lastTime = now;
      
      // Calculate how many 60Hz frames have passed
      const frameDelta = dt / (1000 / 60);
      
      replay.update((r) => {
        if (!r.active) return r;
        carry += (r.speed * frameDelta);
        const step = Math.floor(carry);
        carry -= step;
        
        if (step > 0) {
            const next = r.index + step;
            if (next >= r.packets.length - 1) {
              isLooping = false;
              return { ...r, index: r.packets.length - 1, playing: false };
            }
            return { ...r, index: next };
        }
        return r;
      });
      
      if (isLooping) {
        rafId = requestAnimationFrame(tick);
      }
    }
    
    rafId = requestAnimationFrame(tick);
  }

  $effect(() => {
    ensureLoop($replay.playing, $replay.speed);
  });

  function togglePlay() {
    replay.update((r) => {
      // Restart from beginning if we're parked at the end.
      const atEnd = r.index >= r.packets.length - 1;
      return { ...r, playing: !r.playing, index: atEnd ? 0 : r.index };
    });
  }

  function scrub(e: Event) {
    const v = Number((e.target as HTMLInputElement).value);
    replay.update((r) => ({ ...r, index: v, playing: false }));
  }

  function setSpeed(s: number) {
    replay.update((r) => ({ ...r, speed: s }));
  }

  function fmt(idx: number) {
    const sec = idx / 60;
    const m = Math.floor(sec / 60);
    const s = (sec % 60).toFixed(1).padStart(4, '0');
    return `${m}:${s}`;
  }

  let total = $derived($replay.packets.length);
</script>

{#if $replay.active}
  <div class="replay-bar">
    <div class="left">
      <span class="badge">REPLAY</span>
      <span class="label" title={$replay.label}>{$replay.label}</span>
    </div>

    <div class="controls">
      <button class="play" onclick={togglePlay}>
        {$replay.playing ? '⏸' : '▶'}
      </button>
      <span class="time">{fmt($replay.index)}</span>
      <input
        class="scrub"
        type="range"
        min="0"
        max={Math.max(total - 1, 0)}
        value={$replay.index}
        oninput={scrub}
      />
      <span class="time">{fmt(Math.max(total - 1, 0))}</span>
      <div class="speeds">
        {#each SPEEDS as s}
          <button class:active={$replay.speed === s} onclick={() => setSpeed(s)}>
            {s}×
          </button>
        {/each}
      </div>
    </div>

    <button class="exit" onclick={exitReplay}>Exit replay</button>
  </div>
{/if}

<style>
  .replay-bar {
    position: fixed;
    left: 20px;
    right: 20px;
    bottom: 20px;
    z-index: 110;
    display: flex;
    align-items: center;
    gap: 1.5rem;
    padding: 1rem 1.5rem;
    background: rgba(15, 20, 35, 0.95);
    border: 1px solid rgba(236, 72, 153, 0.5);
    border-radius: 16px;
    box-shadow: 0 8px 32px rgba(236, 72, 153, 0.25), 0 0 0 1px rgba(236, 72, 153, 0.1) inset;
    backdrop-filter: blur(12px);
  }
  .left {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    min-width: 0;
    flex: 0 1 280px;
  }
  .badge {
    background: linear-gradient(135deg, #a855f7, #ec4899);
    color: #fff;
    font-size: 0.75rem;
    font-weight: 900;
    letter-spacing: 0.1em;
    padding: 0.25rem 0.6rem;
    border-radius: 6px;
    text-shadow: 0 1px 2px rgba(0,0,0,0.5);
    box-shadow: 0 2px 10px rgba(236, 72, 153, 0.4);
    animation: pulse-border 2s infinite;
  }
  @keyframes pulse-border {
    0% { box-shadow: 0 0 0 0 rgba(236, 72, 153, 0.4); }
    70% { box-shadow: 0 0 0 6px rgba(236, 72, 153, 0); }
    100% { box-shadow: 0 0 0 0 rgba(236, 72, 153, 0); }
  }
  .label {
    color: #e2e8f0;
    font-size: 0.85rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .controls {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .play {
    background: var(--ac);
    color: #fff;
    border: none;
    border-radius: 50%;
    width: 2rem;
    height: 2rem;
    font-size: 0.85rem;
    cursor: pointer;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
    padding: 0;
  }
  .time {
    color: var(--tx-dim);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    min-width: 3rem;
    text-align: center;
  }
  .scrub {
    flex: 1;
    accent-color: var(--ac);
    cursor: pointer;
  }
  .speeds {
    display: flex;
    gap: 0.2rem;
  }
  .speeds button {
    background: var(--bg-elevated);
    border: 1px solid var(--bd-dim);
    color: var(--tx-dim);
    font-size: 0.7rem;
    padding: 0.2rem 0.4rem;
    border-radius: 4px;
    cursor: pointer;
  }
  .speeds button.active {
    border-color: var(--ac);
    color: var(--tx-hi);
  }
  .exit {
    background: none;
    border: 1px solid var(--bd-subtle);
    color: var(--tx-lo);
    font-size: 0.75rem;
    padding: 0.35rem 0.7rem;
    border-radius: 4px;
    cursor: pointer;
    flex-shrink: 0;
  }
  .exit:hover {
    border-color: #ef4444;
    color: #ef4444;
  }
</style>
