<script lang="ts">
  import { displayPacket } from '$lib/stores/telemetry';
  import { onMount } from 'svelte';

  let pkt = $derived($displayPacket);
  
  const HISTORY = 60;
  let suspFlHistory = $state<number[]>([]);
  let suspFrHistory = $state<number[]>([]);
  let suspRlHistory = $state<number[]>([]);
  let suspRrHistory = $state<number[]>([]);
  let sparkInterval: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    sparkInterval = setInterval(() => {
      const p = $displayPacket;
      if (!p) return;
      suspFlHistory = suspFlHistory.length >= HISTORY ? [...suspFlHistory.slice(1), p.suspensionFl] : [...suspFlHistory, p.suspensionFl];
      suspFrHistory = suspFrHistory.length >= HISTORY ? [...suspFrHistory.slice(1), p.suspensionFr] : [...suspFrHistory, p.suspensionFr];
      suspRlHistory = suspRlHistory.length >= HISTORY ? [...suspRlHistory.slice(1), p.suspensionRl] : [...suspRlHistory, p.suspensionRl];
      suspRrHistory = suspRrHistory.length >= HISTORY ? [...suspRrHistory.slice(1), p.suspensionRr] : [...suspRrHistory, p.suspensionRr];
    }, 200);
    return () => { if (sparkInterval) clearInterval(sparkInterval); };
  });

  function suspPath(data: number[]) {
    if (data.length < 2) return '';
    const max = Math.max(...data, 1);
    const min = Math.min(...data, 0);
    const range = max - min || 1;
    return data.map((v, i) => {
      const x = (i / (HISTORY - 1)) * 100;
      const y = 40 - ((v - min) / range) * 40;
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(' ');
  }
</script>

<div class="pro-panel h-full w-full p-3 flex flex-col">
  <div class="flex justify-between items-center mb-2">
      <h3 class="text-label font-label-mono text-primary uppercase tracking-widest">Suspension Data</h3>
  </div>

  <div class="flex-1 grid grid-cols-2 gap-2">
      {#each [
          { label: 'FL', val: pkt?.suspensionFl ?? 0, hist: suspFlHistory },
          { label: 'FR', val: pkt?.suspensionFr ?? 0, hist: suspFrHistory },
          { label: 'RL', val: pkt?.suspensionRl ?? 0, hist: suspRlHistory },
          { label: 'RR', val: pkt?.suspensionRr ?? 0, hist: suspRrHistory }
      ] as shock}
      <div class="bg-black/20 rounded p-1.5 border border-white/5 relative overflow-hidden flex flex-col">
          <div class="flex justify-between items-center mb-1 relative z-10">
              <span class="text-[8px] font-label-mono text-white/50">{shock.label}</span>
              <span class="text-[9px] font-label-mono text-white font-bold">{shock.val.toFixed(2)}</span>
          </div>
          <!-- Shock absorber visualization -->
          <div class="flex gap-1 items-end h-[35px] relative z-10">
              <div class="w-1.5 h-full bg-white/10 rounded-full overflow-hidden flex items-end">
                  <!-- The higher the value, the more compressed the suspension is (usually) -->
                  <div class="w-full bg-primary transition-all duration-75" style="height: {Math.min(100, Math.max(0, shock.val * 100))}%;"></div>
              </div>
              <div class="flex-1 h-full relative">
                  <svg viewBox="0 0 100 40" preserveAspectRatio="none" class="w-full h-full opacity-60">
                      <path d={suspPath(shock.hist)} fill="none" stroke="#00dbe9" stroke-width="2" vector-effect="non-scaling-stroke" />
                  </svg>
              </div>
          </div>
      </div>
      {/each}
  </div>
</div>
