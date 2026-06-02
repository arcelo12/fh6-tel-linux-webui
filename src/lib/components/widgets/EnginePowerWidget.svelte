<script lang="ts">
  import { displayPacket, rpmPercent } from '$lib/stores/telemetry';
  import { onMount } from 'svelte';

  let pkt = $derived($displayPacket);
  
  // Sparkline history buffers (last 60 samples at ~5 Hz)
  const HISTORY = 60;
  let powerHistory = $state<number[]>([]);
  let torqueHistory = $state<number[]>([]);
  let rpmHistory = $state<number[]>([]);
  let boostHistory = $state<number[]>([]);
  let sparkInterval: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    sparkInterval = setInterval(() => {
      const p = $displayPacket;
      if (!p) return;
      const hp = Math.max(0, Math.round(p.power / 745.7));
      const tq = Math.max(0, Math.round(p.torque));
      const r  = Math.round(p.currentEngineRpm);
      powerHistory  = powerHistory.length >= HISTORY ? [...powerHistory.slice(1), hp] : [...powerHistory, hp];
      torqueHistory = torqueHistory.length >= HISTORY ? [...torqueHistory.slice(1), tq] : [...torqueHistory, tq];
      rpmHistory    = rpmHistory.length >= HISTORY ? [...rpmHistory.slice(1), r] : [...rpmHistory, r];
      boostHistory  = boostHistory.length >= HISTORY ? [...boostHistory.slice(1), p.boost] : [...boostHistory, p.boost];
    }, 200);
    return () => { if (sparkInterval) clearInterval(sparkInterval); };
  });

  function sparkPath(data: number[]) {
    if (data.length < 2) return '';
    const max = Math.max(...data, 1);
    const min = Math.min(...data, 0);
    const range = max - min || 1;
    return data.map((v, i) => {
      const x = (i / (HISTORY - 1)) * 100;
      const y = 28 - ((v - min) / range) * 28;
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(' ');
  }

  function sparkFill(data: number[]) {
    const path = sparkPath(data);
    if (!path) return '';
    return `${path} L 100,28 L 0,28 Z`;
  }
</script>

<div class="pro-panel h-full w-full p-3 flex flex-col">
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
</div>
