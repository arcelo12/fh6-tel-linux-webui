const fs = require('fs');

let html = fs.readFileSync('ui/contoh.html', 'utf-8');

// Extract the body content
const bodyMatch = html.match(/<body[^>]*>([\s\S]*?)<\/body>/);
let bodyContent = bodyMatch ? bodyMatch[1] : '';

// Remove the dummy script at the end
bodyContent = bodyContent.replace(/<script>[\s\S]*?setInterval\([\s\S]*?<\/script>/, '');

// Replace bindings
bodyContent = bodyContent
  .replace('2004 Honda #52 Evasive Motorsports S2000', '{carLabel}')
  .replace('>S2<', '>{classLabel}<')
  .replace('888<', '{piLabel}<')
  .replace('>RWD<', '>{driveLabel}<')
  .replace('>148<', '>{speed}<')
  .replace('>MPH<', '>{unit}<')
  .replace('>8,450<', '>{rpm.toLocaleString()}<')
  .replace('>5<', '>{pkt?.gear === 0 ? "R" : pkt?.gear === 11 ? "N" : (pkt?.gear || "-")}<')
  .replace('LAT <span class="text-white">1.42G</span> | LNG <span class="text-white">0.31G</span>', 'LAT <span class="text-white">{pkt ? (Math.abs(pkt.accelX/9.81)).toFixed(2) : "0.00"}G</span> | LNG <span class="text-white">{pkt ? (Math.abs(pkt.accelZ/9.81)).toFixed(2) : "0.00"}G</span>')
  .replace('PITCH <span class="text-white">4.2°</span> | ROLL <span class="text-white">-2.1°</span>', 'PITCH <span class="text-white">{pkt ? pkt.pitch.toFixed(1) : "0.0"}°</span> | ROLL <span class="text-white">{pkt ? pkt.roll.toFixed(1) : "0.0"}°</span>')
  .replace('style="width: 85%;"', 'style="width: {pkt ? (pkt.throttle/255)*100 : 0}%;"')
  .replace('style="width: 0%;"', 'style="width: {pkt ? (pkt.brake/255)*100 : 0}%;"')
  .replace('style="stroke-dashoffset: 120;"', 'style="stroke-dashoffset: {280 - (rpmPct/100)*200};"')
  // fuel
  .replace('>68%<', '>{pkt ? (pkt.fuel*100).toFixed(0) : 0}%<')
  .replace('style="width: 68%;"', 'style="width: {pkt ? (pkt.fuel*100) : 0}%;"')
  // boost
  .replace('>2.1<', '>{pkt ? pkt.boost.toFixed(1) : "0.0"}<')
  // lap/timing
  .replace('>LAP 12/25<', '>{pkt ? "LAP " + (pkt.lapNumber+1) : "LAP --"}<')
  .replace('>01:24.562<', '>{padClock(pkt?.currentRaceTime)}<')
  .replace('>01:24.990<', '>{padClock(pkt?.lastLap)}<')
  .replace('>01:23.112<', '>{padClock(pkt?.bestLap)}<')
  // tires FL
  .replace('<div class="text-lg font-bold">82', '<div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempFl) : "-"}')
  .replace('<div class="text-lg font-bold">81', '<div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempFr) : "-"}')
  .replace('<div class="text-lg font-bold">89', '<div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempRl) : "-"}')
  .replace('<div class="text-lg font-bold">88', '<div class="text-lg font-bold">{pkt ? Math.round(pkt.tireTempRr) : "-"}')
  // Add an href to go back to Classic mode
  .replace('TELEMETRY<span\\n                    class="text-primary font-light">PRO</span>', '<a href="/" data-sveltekit-reload class="hover:opacity-80 transition-opacity">TELEMETRY<span class="text-primary font-light">PRO</span></a>');

const svelteComponent = \`
<script lang="ts">
  import { onMount } from 'svelte';
  import { displayPacket, isConnected, speedMph, speedKph, rpmPercent, startTelemetryListener } from '$lib/stores/telemetry';
  import { carName } from '$lib/car-name';
  import { CAR_CLASS_LABELS, DRIVETRAIN_LABELS } from '$lib/types';
  import { loadSettings, settings } from '$lib/stores/sessions';

  onMount(async () => {
    await loadSettings();
    await startTelemetryListener();
  });

  let pkt = $derived($displayPacket);
  let speed = $derived($settings?.useMph ? Math.round($speedMph) : Math.round($speedKph));
  let unit = $derived($settings?.useMph ? 'MPH' : 'KPH');
  let rpm = $derived(pkt ? Math.round(pkt.currentEngineRpm) : 0);
  let rpmPct = $derived($rpmPercent);
  
  let carLabel = $derived(pkt ? carName(pkt.carOrdinal) : 'Waiting for Telemetry...');
  let classLabel = $derived(pkt ? (CAR_CLASS_LABELS[pkt.carClass] ?? '?') : '—');
  let piLabel = $derived(pkt ? String(pkt.carPi) : '—');
  let driveLabel = $derived(pkt ? (DRIVETRAIN_LABELS[pkt.drivetrainType] ?? '?') : '—');

  function padClock(sec: number | undefined) {
    if (!sec || sec < 0) return '--:--.---';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    const msPart = Math.floor((sec % 1) * 1000);
    return \`\${m.toString().padStart(2, '0')}:\${s.toString().padStart(2, '0')}.\${msPart.toString().padStart(3, '0')}\`;
  }
</script>

<svelte:head>
  <script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
  <link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@500;600;700&display=swap" rel="stylesheet" />
  <link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap" rel="stylesheet" />
  <style>
    .pro-panel { background: rgba(17, 19, 24, 0.8); border: 1px solid #3b494b; box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4); }
    .glow-cyan { filter: drop-shadow(0 0 8px rgba(0, 219, 233, 0.5)); }
    .speed-arc-bg { stroke: rgba(255, 255, 255, 0.05); stroke-dasharray: 280; stroke-dashoffset: 0; stroke-linecap: round; }
    .speed-arc-active { stroke: #00dbe9; stroke-dasharray: 280; stroke-linecap: round; transition: stroke-dashoffset 0.1s linear; }
    .hud-scanline { background: linear-gradient(rgba(18, 16, 16, 0) 50%, rgba(0, 0, 0, 0.15) 50%); background-size: 100% 2px; pointer-events: none; }
    .mini-graph-path { fill: none; stroke: #00dbe9; stroke-width: 1.5; vector-effect: non-scaling-stroke; }
    .mini-graph-bg { fill: rgba(0, 219, 233, 0.05); }
    .data-table-row { border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
  </style>
  <script>
    tailwind.config = {
      darkMode: "class",
      theme: {
        extend: {
          colors: {
            "primary": "#00dbe9",
            "secondary": "#e9b3ff",
            "error": "#ffb4ab",
            "background": "#0c0e12",
            "surface-container": "#111318",
            "surface-container-highest": "#333539",
            "outline-variant": "#282a2e",
            "on-surface": "#e2e2e8",
            "on-surface-variant": "#b9cacb"
          },
          fontFamily: {
            "display-hero": ["Space Grotesk"],
            "label-mono": ["JetBrains Mono", "monospace"],
            "body-md": ["Inter", "sans-serif"]
          }
        }
      }
    }
  </script>
</svelte:head>

<div class="dark bg-background text-on-surface h-screen font-body-md select-none overflow-hidden flex flex-col">
\${bodyContent}
</div>
\`;

fs.mkdirSync('src/routes/pro', { recursive: true });
fs.writeFileSync('src/routes/pro/+page.svelte', svelteComponent);
console.log('Successfully transformed UI to src/routes/pro/+page.svelte');
