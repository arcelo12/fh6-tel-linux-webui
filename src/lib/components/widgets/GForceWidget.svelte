<script lang="ts">
  import { displayPacket } from '$lib/stores/telemetry';
  import { onMount } from 'svelte';

  let pkt = $derived($displayPacket);
  
  const HISTORY = 60;
  let yawHistory = $state<number[]>([]);
  let sparkInterval: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    sparkInterval = setInterval(() => {
      const p = $displayPacket;
      if (!p) return;
      yawHistory = yawHistory.length >= HISTORY ? [...yawHistory.slice(1), p.yaw] : [...yawHistory, p.yaw];
    }, 200);
    return () => { if (sparkInterval) clearInterval(sparkInterval); };
  });

  function getHeadingLabel(deg: number) {
    if (deg >= 337.5 || deg < 22.5) return 'N';
    if (deg >= 22.5 && deg < 67.5) return 'NE';
    if (deg >= 67.5 && deg < 112.5) return 'E';
    if (deg >= 112.5 && deg < 157.5) return 'SE';
    if (deg >= 157.5 && deg < 202.5) return 'S';
    if (deg >= 202.5 && deg < 247.5) return 'SW';
    if (deg >= 247.5 && deg < 292.5) return 'W';
    if (deg >= 292.5 && deg < 337.5) return 'NW';
    return '';
  }

  function yawPath(data: number[]) {
    if (data.length < 2) return '';
    return data.map((v, i) => {
      // Y-axis represents time (0 = newest at the top, 40 = oldest at the bottom)
      const y = (((HISTORY - 1) - i) / (HISTORY - 1)) * 40;
      
      // X-axis represents heading direction (0/100 = South, 25 = West, 50 = North, 75 = East)
      const deg = ((v * 180) / Math.PI + 360) % 360;
      const x = (((deg + 180) % 360) / 360) * 100;
      
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(' ');
  }
</script>

<div class="pro-panel h-full w-full p-3 flex flex-col">
  <!-- Direction (Seismometer) -->
  <div class="mb-2 flex-1">
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

  <!-- G-Force & Orientation -->
  <div class="flex gap-4 mt-2 h-24" style="flex-shrink:0;">
      <div class="flex-1 flex flex-col items-center">
          <span class="text-[9px] font-label-mono text-white/40 mb-2 uppercase">G-Force</span>
          <div class="w-16 h-16 sm:w-24 sm:h-24 rounded-full border border-outline-variant relative flex items-center justify-center bg-white/5 overflow-hidden">
              <div class="absolute w-full h-[1px] bg-white/10"></div>
              <div class="absolute h-full w-[1px] bg-white/10"></div>
              <div class="w-2 h-2 bg-primary rounded-full absolute shadow-[0_0_8px_#00dbe9] transition-all -translate-x-1/2 -translate-y-1/2"
                  style="left: {pkt ? Math.min(Math.max((pkt.accelX/9.81)/2, -0.5), 0.5)*100 + 50 : 50}%; top: {pkt ? Math.min(Math.max((-pkt.accelZ/9.81)/2, -0.5), 0.5)*100 + 50 : 50}%;"></div>
          </div>
          <div class="mt-2 text-[10px] font-label-mono text-center hidden sm:block">
              LAT <span class="text-white">{pkt ? (Math.abs(pkt.accelX/9.81)).toFixed(2) : "0.00"}G</span> | LNG <span class="text-white">{pkt ? (Math.abs(pkt.accelZ/9.81)).toFixed(2) : "0.00"}G</span>
          </div>
      </div>
      <div class="w-[1px] bg-outline-variant h-full"></div>
      <div class="flex-1 flex flex-col items-center">
          <span class="text-[9px] font-label-mono text-white/40 mb-2 uppercase">Angle</span>
          <div class="w-16 h-16 sm:w-24 sm:h-24 rounded-full border border-outline-variant relative overflow-hidden bg-white/5 flex items-center justify-center">
              <div class="w-10 sm:w-16 h-[2px] bg-primary/40 relative transition-all" style="transform: rotate({pkt ? pkt.roll : 0}deg);">
                  <div class="absolute -top-1 left-1/2 -translate-x-1/2 w-1 h-3 bg-primary transition-all" style="transform: translateY({pkt ? Math.min(Math.max(pkt.pitch, -10), 10) : 0}px);"></div>
              </div>
          </div>
          <div class="mt-2 text-[10px] font-label-mono text-center hidden sm:block">
              PITCH <span class="text-white">{pkt ? pkt.pitch.toFixed(1) : "0.0"}°</span> | ROLL <span class="text-white">{pkt ? pkt.roll.toFixed(1) : "0.0"}°</span>
          </div>
      </div>
  </div>
</div>
