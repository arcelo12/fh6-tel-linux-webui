<script lang="ts">
  import { displayPacket } from '$lib/stores/telemetry';
  let pkt = $derived($displayPacket);
</script>

<div class="pro-panel h-full w-full p-3 flex flex-col">
  <div class="flex justify-between items-center mb-2">
      <h3 class="text-label font-label-mono text-primary uppercase tracking-widest">Transmission &amp; Wheels</h3>
  </div>
  <div class="flex-1 flex flex-col gap-2">
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between items-center">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Gear</span>
          <div class="w-6 h-6 rounded bg-primary/10 border border-primary/30 flex items-center justify-center text-primary font-bold font-display-hero">
              {pkt?.gear === 0 ? "R" : pkt?.gear === 11 ? "N" : (pkt?.gear || "-")}
          </div>
      </div>
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex flex-col gap-1">
          <div class="flex justify-between">
              <span class="text-[9px] font-label-mono text-white/40 uppercase">Clutch</span>
              <span class="text-[10px] font-label-mono text-white font-bold">{pkt ? Math.round((pkt.clutch / 255) * 100) : 0}%</span>
          </div>
          <div class="w-full bg-white/10 rounded-full h-1 overflow-hidden">
              <div class="h-full bg-white transition-all duration-75" style="width: {pkt ? (pkt.clutch / 255) * 100 : 0}%"></div>
          </div>
      </div>
      <div class="bg-black/20 rounded px-2 py-1.5 border border-white/5 flex justify-between mt-auto">
          <span class="text-[9px] font-label-mono text-white/40 uppercase">Speed X/Y/Z</span>
          <span class="text-[10px] font-label-mono text-white/60">
              {pkt ? Math.round(pkt.speedX) : 0} | {pkt ? Math.round(pkt.speedY) : 0} | {pkt ? Math.round(pkt.speedZ) : 0}
          </span>
      </div>
  </div>
</div>
