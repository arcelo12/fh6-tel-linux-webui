<script lang="ts">
  import { displayPacket } from '$lib/stores/telemetry';
  let pkt = $derived($displayPacket);
</script>

<div class="pro-panel h-full w-full p-3 flex flex-col min-h-0">
  <div class="flex justify-between items-center mb-2 flex-shrink-0">
      <h3 class="text-label font-label-mono text-primary uppercase tracking-widest">Telemetry Raw Data Dump</h3>
  </div>
  <div class="flex-1 min-h-0 overflow-y-auto bg-black/40 rounded p-2 border border-white/5 font-mono text-[9px] leading-relaxed text-white/70 whitespace-pre">
      {#if pkt}
          {JSON.stringify(pkt, null, 2)}
      {:else}
          Waiting for telemetry connection...
      {/if}
  </div>
</div>
