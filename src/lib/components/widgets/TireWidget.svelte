<script lang="ts">
  import { displayPacket } from '$lib/stores/telemetry';
  
  let pkt = $derived($displayPacket);

  // Helper to compute advanced tire slip and grip loss metrics
  function computeTireState(
    temp: number | undefined,
    slipRatio: number | undefined,
    slipAngle: number | undefined
  ) {
    const t = temp ?? 0;
    const ratio = slipRatio ?? 0;
    const angle = slipAngle ?? 0;
    
    const absRatio = Math.abs(ratio);
    const absAngle = Math.abs(angle);

    // Scale grip loss for UI feedback. 
    // High traction limit is ~0.15 slip ratio or slip angle. 
    // Multiply by 250 to map 0.25+ combined slip to 100% grip loss.
    const rawLoss = (absRatio * 250) + (absAngle * 250);
    const gripLoss = t ? Math.min(100, Math.round(rawLoss)) : 0;

    // Traction Status flag text and colors
    let statusText = 'GRIP';
    let statusColorClass = 'text-green-400';
    
    if (gripLoss >= 50) {
      statusText = absRatio > absAngle ? 'SPIN/LOCK' : 'SLIDE';
      statusColorClass = 'text-red-500 font-extrabold animate-pulse';
    } else if (gripLoss >= 15) {
      statusText = 'SLIP LIMIT';
      statusColorClass = 'text-yellow-400 font-bold';
    }

    return {
      temp: t,
      ratio,
      angle,
      gripLoss,
      statusText,
      statusColorClass
    };
  }

  // Reactive derived tire mappings
  let tires = $derived([
    { label: 'FL', data: computeTireState(pkt?.tireTempFl, pkt?.tireSlipRatioFl, pkt?.tireSlipAngleFl) },
    { label: 'FR', data: computeTireState(pkt?.tireTempFr, pkt?.tireSlipRatioFr, pkt?.tireSlipAngleFr) },
    { label: 'RL', data: computeTireState(pkt?.tireTempRl, pkt?.tireSlipRatioRl, pkt?.tireSlipAngleRl) },
    { label: 'RR', data: computeTireState(pkt?.tireTempRr, pkt?.tireSlipRatioRr, pkt?.tireSlipAngleRr) }
  ]);
</script>

<div class="pro-panel h-full w-full p-2 md:p-3 flex flex-col min-h-0 overflow-hidden select-none">
  <div class="flex justify-between items-center mb-1.5 flex-shrink-0">
    <h3 class="text-[10px] font-label-mono text-primary uppercase tracking-widest font-bold">Tire Dynamics & Grip</h3>
  </div>

  <div class="flex-1 grid grid-cols-2 gap-2 min-h-0 overflow-y-auto">
    {#each tires as tire}
      <div class="bg-black/35 rounded border border-white/5 p-2 flex flex-col gap-1.5 justify-between">
        <!-- Tire Header: Label + Status -->
        <div class="flex justify-between items-center w-full">
          <span class="text-[10px] font-label-mono font-bold text-white/50">{tire.label}</span>
          <span class="text-[8px] font-label-mono uppercase {tire.data.statusColorClass}">{pkt ? tire.data.statusText : 'OFFLINE'}</span>
        </div>

        <!-- Temp Metric -->
        <div class="flex justify-between items-center w-full">
          <div class="flex flex-col">
            <span class="text-[7px] font-label-mono text-white/30 uppercase">Temp</span>
            <span class="text-sm font-bold font-display-hero text-white/90">
              {pkt && tire.data.temp ? Math.round(tire.data.temp) : '-'}<span class="text-[8px] text-white/40 font-normal">°C</span>
            </span>
          </div>
          <div class="flex flex-col items-end text-right">
            <span class="text-[7px] font-label-mono text-white/30 uppercase">Traction</span>
            <span class="text-[9px] font-bold font-label-mono text-[#00dbe9]">
              {pkt ? `${Math.max(0, 100 - tire.data.gripLoss)}%` : '0%'}
            </span>
          </div>
        </div>

        <!-- Advanced Slip/Grip Loss Metrics -->
        <div class="grid grid-cols-2 gap-1 bg-white/[0.02] border border-white/5 rounded p-1 text-[8px] font-label-mono">
          <div class="flex flex-col">
            <span class="text-white/30 scale-[0.9] origin-left">Slip Ratio</span>
            <span class="font-medium {Math.abs(tire.data.ratio) > 0.15 ? 'text-red-400' : 'text-white/70'}">
              {pkt ? (tire.data.ratio >= 0 ? '+' : '') + tire.data.ratio.toFixed(2) : '0.00'}
            </span>
          </div>
          <div class="flex flex-col items-end text-right">
            <span class="text-white/30 scale-[0.9] origin-right">Slip Angle</span>
            <span class="font-medium {Math.abs(tire.data.angle) > 0.15 ? 'text-red-400' : 'text-white/70'}">
              {pkt ? (tire.data.angle >= 0 ? '+' : '') + tire.data.angle.toFixed(2) : '0.00'}
            </span>
          </div>
        </div>

        <!-- Grip Loss Graphic Progress Bar -->
        <div class="w-full mt-0.5 flex flex-col gap-0.5">
          <div class="flex justify-between items-center text-[7px] font-label-mono text-white/40 uppercase">
            <span>Grip Loss</span>
            <span class="font-bold {tire.data.gripLoss > 15 ? (tire.data.gripLoss >= 50 ? 'text-red-500' : 'text-yellow-400') : 'text-primary'}">
              {pkt ? tire.data.gripLoss : 0}%
            </span>
          </div>
          <div class="h-1 w-full bg-white/5 rounded overflow-hidden">
            <div 
              class="h-full rounded transition-all duration-75 {tire.data.gripLoss >= 50 ? 'bg-red-500 animate-pulse' : tire.data.gripLoss >= 15 ? 'bg-yellow-400' : 'bg-[#00dbe9]'}"
              style="width: {pkt ? tire.data.gripLoss : 0}%"
            ></div>
          </div>
        </div>
      </div>
    {/each}
  </div>

  <!-- Steering Wheel Angle Indicator at the bottom of Tire Widget -->
  <div class="mt-2 pt-2 border-t border-white/5 flex items-center justify-between gap-3 flex-shrink-0">
    <div class="flex flex-col">
      <span class="text-[7px] font-label-mono text-white/30 uppercase">Steering Angle</span>
      <span class="text-[9px] font-label-mono font-bold text-white/95">
        {#if !pkt}
          --
        {:else}
          {Math.round(Math.abs((pkt.steer / 127) * 100))}% 
          <span class="text-[8px] text-white/40 font-normal ml-0.5">
            {Math.abs(pkt.steer) < 6 ? 'CTR' : pkt.steer < 0 ? 'LEFT' : 'RIGHT'}
          </span>
        {/if}
      </span>
    </div>
    <!-- Visual horizontal steering slider -->
    <div class="flex-1 h-2 bg-white/5 rounded relative border border-white/5 overflow-hidden">
      <div class="absolute left-1/2 top-0 bottom-0 w-[1px] bg-white/30"></div>
      {#if pkt}
        {#if pkt.steer < 0}
          <div class="absolute h-full bg-[#00dbe9] rounded-l animate-pulse" style="right: 50%; width: {Math.min(50, Math.abs(pkt.steer / 127) * 50)}%"></div>
        {:else if pkt.steer > 0}
          <div class="absolute h-full bg-[#00dbe9] rounded-r animate-pulse" style="left: 50%; width: {Math.min(50, (pkt.steer / 127) * 50)}%"></div>
        {/if}
      {/if}
    </div>
  </div>
</div>
