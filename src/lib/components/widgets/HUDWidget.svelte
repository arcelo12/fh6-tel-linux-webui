<script lang="ts">
  import { displayPacket, speedMph, speedKph, rpmPercent } from '$lib/stores/telemetry';
  import { settings } from '$lib/stores/sessions';
  import { onMount } from 'svelte';

  let pkt = $derived($displayPacket);
  let speed    = $derived($settings?.useMph ? Math.round($speedMph) : Math.round($speedKph));
  let unit     = $derived($settings?.useMph ? 'MPH' : 'KPH');
  let rpm      = $derived(pkt ? Math.round(pkt.currentEngineRpm) : 0);
  let maxRpm   = $derived(pkt ? Math.round(pkt.engineMaxRpm)     : 8000);
  let idleRpm  = $derived(pkt ? Math.round(pkt.engineIdleRpm)    : 800);
  let rpmPct   = $derived($rpmPercent);
  let needleAngle = $derived(-140 + (rpmPct / 100) * 280);
  let gearLabel = $derived(pkt?.gear === 0 ? 'R' : pkt?.gear === 11 ? 'N' : (pkt?.gear ? String(pkt.gear) : '-'));
  let thrPct   = $derived(pkt ? Math.round((pkt.throttle / 255) * 100) : 0);
  let brkPct   = $derived(pkt ? Math.round((pkt.brake / 255)    * 100) : 0);
  let hbPct    = $derived(pkt ? Math.round((pkt.handbrake / 255)* 100) : 0);
  let clutchPct = $derived(pkt ? Math.round((pkt.clutch / 255)  * 100) : 0);

  // Redline threshold (top 15% of RPM range)
  let rpmDanger = $derived(rpmPct > 85);

  // LED segment blocks for digital RPM display
  const TOTAL_SEGS = 24;
  // returns 'green' | 'yellow' | 'red' | 'off'
  function segColor(i: number, lit: boolean): string {
    if (!lit) return 'off';
    const pct = ((i + 1) / TOTAL_SEGS) * 100;
    if (pct > 85) return 'red';
    if (pct > 65) return 'yellow';
    return 'green';
  }

  // Arc SVG constants
  const ARC_R = 44;
  const ARC_CIRC = 2 * Math.PI * ARC_R; // ≈ 276.46
  const ARC_GAP_DEG = 80; // gap at bottom
  const ARC_ACTIVE_DEG = 360 - ARC_GAP_DEG; // 280
  let arcOffset = $derived(ARC_CIRC - (rpmPct / 100) * (ARC_ACTIVE_DEG / 360) * ARC_CIRC);

  let gaugeStyle = $state<'arc' | 'digital' | 'analog'>('arc');

  onMount(() => {
    const saved = localStorage.getItem('gaugeStyle');
    if (saved === 'arc' || saved === 'digital' || saved === 'analog') gaugeStyle = saved;
  });

  function selectGaugeStyle(style: 'arc' | 'digital' | 'analog') {
    gaugeStyle = style;
    localStorage.setItem('gaugeStyle', style);
  }
</script>

<div class="hud-root pro-panel">
  <!-- Style Selector -->
  <div class="style-selector">
    {#each ['arc','digital','analog'] as s}
      <button class="style-btn" class:active={gaugeStyle === s} onclick={() => selectGaugeStyle(s as any)}>
        {s}
      </button>
    {/each}
  </div>

  <!-- ===== ARC MODE ===== -->
  {#if gaugeStyle === 'arc'}
  <div class="arc-layout">
    <!-- RPM Arc Gauge -->
    <div class="arc-gauge-wrap">
      <svg viewBox="0 0 100 100" class="arc-svg" style="transform: rotate(-130deg)">
        <!-- Track -->
        <circle cx="50" cy="50" r={ARC_R} fill="none"
          stroke="rgba(255,255,255,0.06)" stroke-width="7"
          stroke-dasharray="{(ARC_ACTIVE_DEG/360)*ARC_CIRC} {ARC_CIRC}"
          stroke-linecap="round"/>
        <!-- Danger zone (top 15%) -->
        <circle cx="50" cy="50" r={ARC_R} fill="none"
          stroke="rgba(255,80,80,0.18)" stroke-width="7"
          stroke-dasharray="{(0.15)*(ARC_ACTIVE_DEG/360)*ARC_CIRC} {ARC_CIRC}"
          stroke-dashoffset="-{(0.85)*(ARC_ACTIVE_DEG/360)*ARC_CIRC}"
          stroke-linecap="round"/>
        <!-- Active RPM fill -->
        <circle cx="50" cy="50" r={ARC_R} fill="none"
          stroke={rpmDanger ? '#ff5050' : '#00dbe9'}
          stroke-width="7"
          stroke-dasharray="{(ARC_ACTIVE_DEG/360)*ARC_CIRC} {ARC_CIRC}"
          stroke-dashoffset="{arcOffset}"
          stroke-linecap="round"
          style="transition: stroke-dashoffset 0.08s linear, stroke 0.15s ease; filter: drop-shadow(0 0 5px {rpmDanger ? '#ff5050' : '#00dbe9'})"/>
      </svg>
      <!-- Inner Content -->
      <div class="arc-inner">
        <div class="arc-speed">{speed}</div>
        <div class="arc-unit">{unit}</div>
        <div class="arc-rpm">{rpm.toLocaleString()} RPM</div>
        <!-- Gear -->
        <div class="gear-badge" class:redline={rpmDanger}>{gearLabel}</div>
      </div>
    </div>

    <!-- Bottom: THR / BRK / CLT / HB bars -->
    <div class="pedal-row">
      <div class="pedal-col">
        <div class="pedal-label thr">THR</div>
        <div class="pedal-track">
          <div class="pedal-fill thr" style="height: {thrPct}%"></div>
        </div>
        <div class="pedal-pct">{thrPct}%</div>
      </div>
      <div class="pedal-col">
        <div class="pedal-label brk">BRK</div>
        <div class="pedal-track">
          <div class="pedal-fill brk" style="height: {brkPct}%"></div>
        </div>
        <div class="pedal-pct">{brkPct}%</div>
      </div>
      <div class="pedal-col">
        <div class="pedal-label clt">CLT</div>
        <div class="pedal-track">
          <div class="pedal-fill clt" style="height: {clutchPct}%"></div>
        </div>
        <div class="pedal-pct">{clutchPct}%</div>
      </div>
      <div class="pedal-col">
        <div class="pedal-label hb" class:active-hb={hbPct > 10}>HB</div>
        <div class="pedal-track">
          <div class="pedal-fill hb" style="height: {hbPct}%"></div>
        </div>
        <div class="pedal-pct">{hbPct}%</div>
      </div>
    </div>
  </div>

  <!-- ===== DIGITAL MODE ===== -->
  {:else if gaugeStyle === 'digital'}
  <div class="digital-layout">
    <!-- Top speed row -->
    <div class="digital-top">
      <div class="digital-speed-block">
        <div class="digital-speed-num">{speed}</div>
        <div class="digital-speed-unit">{unit}</div>
      </div>
      <div class="digital-divider"></div>
      <div class="digital-gear-block">
        <div class="dg-label">GEAR</div>
        <div class="digital-gear" class:redline={rpmDanger}>{gearLabel}</div>
      </div>
    </div>

    <!-- RPM Segment bar -->
    <div class="rpm-seg-wrap">
      <div class="rpm-bar-label">
        <span>RPM</span>
        <span class:rpm-red={rpmDanger}>{rpm.toLocaleString()}</span>
      </div>
      <div class="rpm-segs">
        {#each Array.from({ length: TOTAL_SEGS }) as _, i}
          {@const lit = (i / TOTAL_SEGS) * 100 < rpmPct}
          <div class="rpm-seg seg-{segColor(i, lit)}"></div>
        {/each}
      </div>
      <div class="rpm-minmax">
        <span>{idleRpm.toLocaleString()}</span>
        <span>{maxRpm.toLocaleString()}</span>
      </div>
    </div>

    <!-- Pedal inputs -->
    <div class="digital-pedals">
      {#each [
        { label: 'THROTTLE', val: thrPct, cls: 'thr' },
        { label: 'BRAKE',    val: brkPct, cls: 'brk' },
        { label: 'CLUTCH',   val: clutchPct, cls: 'clt' },
        { label: 'HANDBRAKE',val: hbPct,  cls: 'hb'  },
      ] as p}
      <div class="dp-row">
        <span class="dp-label {p.cls}">{p.label}</span>
        <div class="dp-track">
          <div class="dp-fill {p.cls}" style="width: {p.val}%"></div>
        </div>
        <span class="dp-pct">{p.val}%</span>
      </div>
      {/each}
    </div>
  </div>

  <!-- ===== ANALOG MODE ===== -->
  {:else if gaugeStyle === 'analog'}
  <div class="analog-layout">
    <div class="analog-gauge-wrap">
      <!-- Analog SVG Dial -->
      <svg viewBox="0 0 100 100" class="analog-svg">
        <!-- Outer ring -->
        <circle cx="50" cy="50" r="47" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="1"/>
        <!-- Tick marks -->
        {#each Array.from({ length: 21 }) as _, i}
          {@const angle = -140 + i * 14}
          {@const rad = (angle - 90) * Math.PI / 180}
          {@const isMajor = i % 2 === 0}
          {@const isRed = i >= 17}
          {@const r1 = isMajor ? 37 : 39}
          {@const r2 = 44}
          <line
            x1={50 + r1 * Math.cos(rad)} y1={50 + r1 * Math.sin(rad)}
            x2={50 + r2 * Math.cos(rad)} y2={50 + r2 * Math.sin(rad)}
            stroke={isRed ? 'rgba(255,80,80,0.7)' : isMajor ? 'rgba(255,255,255,0.45)' : 'rgba(255,255,255,0.2)'}
            stroke-width={isMajor ? 1.2 : 0.7}/>
          {#if isMajor}
            {@const tx = 50 + 32 * Math.cos(rad)}
            {@const ty = 50 + 32 * Math.sin(rad)}
            <text x={tx} y={ty} fill={isRed ? '#ff8080' : 'rgba(255,255,255,0.4)'}
              font-size="4" font-family="'JetBrains Mono', monospace" font-weight="bold"
              text-anchor="middle" dominant-baseline="middle">{i/2}</text>
          {/if}
        {/each}
        <!-- Redline arc -->
        <circle cx="50" cy="50" r="44" fill="none"
          stroke="rgba(255,80,80,0.25)" stroke-width="3"
          stroke-dasharray="{(0.15)*(280/360)*ARC_CIRC} {ARC_CIRC}"
          stroke-dashoffset="-{(0.85)*(280/360)*ARC_CIRC}"
          style="transform: rotate(-130deg); transform-origin: 50% 50%;"/>
        <!-- Needle -->
        <line
          x1="50" y1="50"
          x2={50 + 40 * Math.cos((needleAngle - 90) * Math.PI / 180)}
          y2={50 + 40 * Math.sin((needleAngle - 90) * Math.PI / 180)}
          stroke={rpmDanger ? '#ff5050' : '#00dbe9'}
          stroke-width="1.8" stroke-linecap="round"
          style="filter: drop-shadow(0 0 4px {rpmDanger ? '#ff5050' : '#00dbe9'}); transition: all 0.06s linear;"/>
        <!-- Hub -->
        <circle cx="50" cy="50" r="3.5" fill={rpmDanger ? '#ff5050' : '#00dbe9'}/>
        <circle cx="50" cy="50" r="1.5" fill="#0c0e12"/>
      </svg>

      <!-- Center overlay text -->
      <div class="analog-center">
        <div class="analog-speed">{speed}</div>
        <div class="analog-unit">{unit}</div>
        <div class="analog-gear" class:redline={rpmDanger}>{gearLabel}</div>
        <div class="analog-rpm">{rpm.toLocaleString()}</div>
      </div>
    </div>

    <!-- Vertical pedal bars -->
    <div class="pedal-row">
      {#each [
        { label: 'THR', val: thrPct, cls: 'thr' },
        { label: 'BRK', val: brkPct, cls: 'brk' },
        { label: 'CLT', val: clutchPct, cls: 'clt' },
        { label: 'HB',  val: hbPct,  cls: 'hb'  },
      ] as p}
      <div class="pedal-col">
        <div class="pedal-label {p.cls}" class:active-hb={p.cls === 'hb' && hbPct > 10}>{p.label}</div>
        <div class="pedal-track">
          <div class="pedal-fill {p.cls}" style="height: {p.val}%"></div>
        </div>
        <div class="pedal-pct">{p.val}%</div>
      </div>
      {/each}
    </div>
  </div>
  {/if}
</div>

<style>
  /* ---- Root ---- */
  .hud-root {
    width: 100%; height: 100%;
    display: flex; flex-direction: column;
    position: relative; overflow: hidden;
    background: #08101a;
    border-radius: 10px;
    padding: 0.5rem;
  }

  /* ---- Style selector ---- */
  .style-selector {
    position: absolute; top: 8px; right: 8px;
    display: flex; gap: 3px; z-index: 20;
  }
  .style-btn {
    padding: 2px 8px;
    background: rgba(255,255,255,0.04);
    border: 1px solid rgba(255,255,255,0.08);
    border-radius: 4px;
    color: rgba(255,255,255,0.35);
    font-size: 9px; font-family: 'JetBrains Mono', monospace;
    font-weight: 700; text-transform: uppercase;
    cursor: pointer; transition: all 0.15s ease;
  }
  .style-btn:hover { background: rgba(255,255,255,0.08); color: #fff; }
  .style-btn.active { background: #00dbe9; color: #000; border-color: #00dbe9; }

  /* ====== ARC LAYOUT ====== */
  .arc-layout {
    flex: 1; display: flex; flex-direction: column;
    align-items: center; justify-content: space-between;
    gap: 4px; overflow: hidden;
    padding-top: 28px;
  }
  .arc-gauge-wrap {
    position: relative;
    width: min(280px, 75%); aspect-ratio: 1;
    flex-shrink: 1;
  }
  .arc-svg { width: 100%; height: 100%; }
  .arc-inner {
    position: absolute; inset: 0;
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    gap: 2px;
  }
  .arc-speed {
    font-family: 'Space Grotesk', sans-serif;
    font-size: clamp(2.5rem, 8vw, 4.5rem);
    font-weight: 800; line-height: 1;
    letter-spacing: -0.03em; color: #fff;
  }
  .arc-unit {
    font-size: 9px; font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.35em; color: rgba(255,255,255,0.35);
    font-weight: 700; text-transform: uppercase;
  }
  .arc-rpm {
    font-size: 9px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.35); margin-top: 2px;
  }
  .gear-badge {
    margin-top: 6px;
    width: 42px; height: 42px;
    display: flex; align-items: center; justify-content: center;
    background: rgba(0,219,233,0.1);
    border: 2px solid rgba(0,219,233,0.4);
    border-radius: 8px;
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1.6rem; font-weight: 800;
    color: #00dbe9;
    transition: all 0.1s ease;
  }
  .gear-badge.redline {
    background: rgba(255,80,80,0.15);
    border-color: rgba(255,80,80,0.6);
    color: #ff5050;
    box-shadow: 0 0 12px rgba(255,80,80,0.3);
  }

  /* ====== PEDAL BARS (shared arc + analog) ====== */
  .pedal-row {
    display: flex; gap: 8px; justify-content: center;
    padding: 4px 8px 6px;
    width: 100%;
  }
  .pedal-col {
    display: flex; flex-direction: column;
    align-items: center; gap: 3px;
    flex: 1; max-width: 52px;
  }
  .pedal-label {
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    font-weight: 700; text-transform: uppercase;
    color: rgba(255,255,255,0.3);
    letter-spacing: 0.05em;
  }
  .pedal-label.thr { color: #00dbe9; }
  .pedal-label.brk { color: #ff5050; }
  .pedal-label.clt { color: rgba(255,255,255,0.5); }
  .pedal-label.hb  { color: #ffc600; }
  .pedal-label.active-hb { color: #ff8800; text-shadow: 0 0 6px #ff8800; }

  .pedal-track {
    width: 14px; height: 64px;
    background: rgba(255,255,255,0.05);
    border-radius: 4px; overflow: hidden;
    display: flex; flex-direction: column; justify-content: flex-end;
    border: 1px solid rgba(255,255,255,0.06);
  }
  .pedal-fill {
    width: 100%; border-radius: 3px;
    transition: height 0.06s linear;
  }
  .pedal-fill.thr { background: linear-gradient(to top, #00dbe9, #00a3af); }
  .pedal-fill.brk { background: linear-gradient(to top, #ff5050, #c43030); }
  .pedal-fill.clt { background: linear-gradient(to top, rgba(255,255,255,0.6), rgba(255,255,255,0.3)); }
  .pedal-fill.hb  { background: linear-gradient(to top, #ffc600, #e09000); }
  .pedal-pct {
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.35); font-weight: 700;
  }

  /* ====== DIGITAL LAYOUT ====== */
  .digital-layout {
    flex: 1; display: flex; flex-direction: column;
    gap: 8px; padding-top: 28px; overflow: hidden;
  }
  .digital-top {
    display: flex; align-items: center; justify-content: center;
    gap: 0; padding: 12px 16px;
    background: rgba(0,0,0,0.3);
    border: 1px solid rgba(255,255,255,0.06);
    border-radius: 10px; margin: 0 4px;
  }
  .digital-speed-block { text-align: center; flex: 1; }
  .digital-speed-num {
    font-family: 'Space Grotesk', sans-serif;
    font-size: clamp(3rem, 10vw, 5rem);
    font-weight: 800; line-height: 1;
    letter-spacing: -0.03em; color: #fff;
  }
  .digital-speed-unit {
    font-size: 9px; font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.4em; color: rgba(255,255,255,0.3);
    font-weight: 700; text-transform: uppercase; margin-top: -4px;
  }
  .digital-divider {
    width: 1px; height: 60px;
    background: rgba(255,255,255,0.1);
    flex-shrink: 0; margin: 0 16px;
  }
  .digital-gear-block { text-align: center; }
  .dg-label {
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.3); font-weight: 700;
    letter-spacing: 0.1em; margin-bottom: 4px;
  }
  .digital-gear {
    width: 56px; height: 56px;
    display: flex; align-items: center; justify-content: center;
    background: rgba(0,219,233,0.1);
    border: 2px solid rgba(0,219,233,0.4);
    border-radius: 10px;
    font-family: 'Space Grotesk', sans-serif;
    font-size: 2.2rem; font-weight: 800; color: #00dbe9;
    transition: all 0.1s ease;
  }
  .digital-gear.redline {
    background: rgba(255,80,80,0.12);
    border-color: rgba(255,80,80,0.5);
    color: #ff5050;
    box-shadow: 0 0 16px rgba(255,80,80,0.25);
  }
  /* RPM Segment LED display */
  .rpm-seg-wrap { padding: 0 8px; }
  .rpm-bar-label {
    display: flex; justify-content: space-between;
    font-size: 9px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.4); margin-bottom: 5px;
  }
  .rpm-red { color: #ff5050 !important; }
  .rpm-segs {
    display: flex; gap: 2px; align-items: flex-end; height: 20px;
  }
  .rpm-seg {
    flex: 1; border-radius: 2px;
    transition: background 0.06s linear, box-shadow 0.06s linear;
  }
  /* Different heights for visual rhythm — creates a "slanted" look */
  .rpm-seg:nth-child(3n+1) { height: 14px; }
  .rpm-seg:nth-child(3n+2) { height: 18px; }
  .rpm-seg:nth-child(3n)   { height: 20px; }
  .seg-off  { background: rgba(255,255,255,0.05); }
  .seg-green  { background: #00dbe9; box-shadow: 0 0 4px rgba(0,219,233,0.5); }
  .seg-yellow { background: #ffc600; box-shadow: 0 0 4px rgba(255,198,0,0.5); }
  .seg-red    { background: #ff5050; box-shadow: 0 0 6px rgba(255,80,80,0.7); }
  .rpm-minmax {
    display: flex; justify-content: space-between;
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.2); margin-top: 3px;
  }
  /* Digital pedals */
  .digital-pedals { padding: 0 8px; display: flex; flex-direction: column; gap: 5px; }
  .dp-row {
    display: flex; align-items: center; gap: 8px;
  }
  .dp-label {
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    font-weight: 700; width: 62px; flex-shrink: 0;
    letter-spacing: 0.05em; text-transform: uppercase;
    color: rgba(255,255,255,0.3);
  }
  .dp-label.thr { color: #00dbe9; }
  .dp-label.brk { color: #ff5050; }
  .dp-label.clt { color: rgba(255,255,255,0.5); }
  .dp-label.hb  { color: #ffc600; }
  .dp-track {
    flex: 1; height: 8px;
    background: rgba(255,255,255,0.05);
    border-radius: 4px; overflow: hidden;
    border: 1px solid rgba(255,255,255,0.04);
  }
  .dp-fill {
    height: 100%; border-radius: 4px;
    transition: width 0.06s linear;
  }
  .dp-fill.thr { background: linear-gradient(to right, #00dbe9, #00a3af); box-shadow: 0 0 6px rgba(0,219,233,0.3); }
  .dp-fill.brk { background: linear-gradient(to right, #ff5050, #c43030); box-shadow: 0 0 6px rgba(255,80,80,0.3); }
  .dp-fill.clt { background: rgba(255,255,255,0.5); }
  .dp-fill.hb  { background: linear-gradient(to right, #ffc600, #e09000); }
  .dp-pct {
    font-size: 9px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.4); width: 28px; text-align: right; flex-shrink: 0;
  }

  /* ====== ANALOG LAYOUT ====== */
  .analog-layout {
    flex: 1; display: flex; flex-direction: column;
    align-items: center; justify-content: space-between;
    padding-top: 24px; gap: 4px; overflow: hidden;
  }
  .analog-gauge-wrap {
    position: relative;
    width: min(280px, 78%); aspect-ratio: 1;
    flex-shrink: 1;
  }
  .analog-svg { width: 100%; height: 100%; }
  .analog-center {
    position: absolute; inset: 0;
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    padding-top: 28%;
    gap: 1px;
  }
  .analog-speed {
    font-family: 'Space Grotesk', sans-serif;
    font-size: clamp(2rem, 6vw, 3rem);
    font-weight: 800; line-height: 1;
    letter-spacing: -0.03em; color: #fff;
  }
  .analog-unit {
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.35em; color: rgba(255,255,255,0.3);
    font-weight: 700; text-transform: uppercase;
  }
  .analog-gear {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1.5rem; font-weight: 800; color: #00dbe9;
    background: rgba(0,219,233,0.1);
    border: 1.5px solid rgba(0,219,233,0.35);
    border-radius: 6px; padding: 0 10px;
    line-height: 1.6; margin-top: 4px;
    transition: all 0.1s ease;
  }
  .analog-gear.redline { color: #ff5050; background: rgba(255,80,80,0.1); border-color: rgba(255,80,80,0.4); }
  .analog-rpm {
    font-size: 8px; font-family: 'JetBrains Mono', monospace;
    color: rgba(255,255,255,0.3); margin-top: 2px;
  }
</style>
