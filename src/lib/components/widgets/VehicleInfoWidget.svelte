<script lang="ts">
  import { displayPacket } from '$lib/stores/telemetry';
  import { CAR_CLASS_LABELS, DRIVETRAIN_LABELS } from '$lib/types';
  let pkt = $derived($displayPacket);
</script>

<div class="pro-panel h-full w-full p-3 flex flex-col">
  <div class="flex justify-between items-center mb-2">
      <h3 class="text-label font-label-mono text-primary uppercase tracking-widest">Vehicle Info</h3>
  </div>
  <div class="flex-1 flex flex-col gap-2">
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Class</span>
          <span class="text-[10px] font-label-mono text-secondary font-bold">
              {pkt ? String.fromCharCode(pkt.carClass) : "-"} <span class="text-white/30 ml-1">({pkt ? CAR_CLASS_LABELS[pkt.carClass] || 'Unknown' : '---'})</span>
          </span>
      </div>
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Perf Index</span>
          <span class="text-[10px] font-label-mono text-white font-bold">{pkt ? pkt.carPi : "---"}</span>
      </div>
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Drivetrain</span>
          <span class="text-[10px] font-label-mono text-white font-bold">{pkt ? DRIVETRAIN_LABELS[pkt.drivetrainType] || 'UNK' : '---'}</span>
      </div>
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Cylinders</span>
          <span class="text-[10px] font-label-mono text-white font-bold">{pkt ? Math.max(0, pkt.numCylinders) : "-"}</span>
      </div>
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between mt-auto">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Mass / Weight</span>
          <span class="text-[10px] font-label-mono text-white font-bold">{pkt ? Math.round(pkt.mass) : 0}<span class="text-white/30 ml-1">KG</span></span>
      </div>
  </div>
</div>
