<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { Map as LMap, TileLayer, LayerGroup } from 'leaflet';
  import TrackMap from './TrackMap.svelte';
  import { effectiveMapConfig } from '$lib/mapDefaults';
  import { xyzSimpleCRS } from '$lib/mapCrs';
  import type { TelemetryPacket, AppSettings } from '$lib/types';

  let {
    points,
    currentIndex = -1,
    drawLine = true,
    colorByLap = true,
    fixedTrace = false,
    settings,
    compact = false,
    multiplayers = null,
  }: {
    points: TelemetryPacket[];
    currentIndex?: number;
    drawLine?: boolean;
    colorByLap?: boolean;
    /** Complete known track (replay / recorded view): fit once, then let the
     *  user zoom in & pan but lock them inside the track's bounds/zoom. */
    fixedTrace?: boolean;
    settings: AppSettings;
    compact?: boolean;
    multiplayers?: { slotNumber: number; name: string; color: string; packet: TelemetryPacket }[] | null;
  } = $props();

  const LAP_COLORS = [
    '#3b82f6', '#22c55e', '#f59e0b', '#a855f7', '#ef4444', '#06b6d4', '#ec4899',
  ];

  // Resolves to the FH6 Japan preset unless the user opted into overriding.
  let cfg = $derived(effectiveMapConfig(settings));

  // Independent per-axis linear fit from two reference points: world X → pixel
  // X, world Z → pixel Y. A rotation/similarity can't represent the axis
  // reflection an overhead game map needs (world Z grows north, pixel Y grows
  // south); per-axis slopes carry their own sign, so this stays correct.
  let calib = $derived.by(() => {
    const aw = cfg.calAWorld, bw = cfg.calBWorld;
    const ap = cfg.calAPix, bp = cfg.calBPix;
    const dWX = bw[0] - aw[0];
    const dWZ = bw[1] - aw[1];
    // Need the two points to differ on both world axes.
    if (Math.abs(dWX) < 1e-6 || Math.abs(dWZ) < 1e-6) return null;
    const mX = (bp[0] - ap[0]) / dWX;
    const mZ = (bp[1] - ap[1]) / dWZ;
    return {
      mX,
      mZ,
      bX: ap[0] - mX * aw[0],
      bY: ap[1] - mZ * aw[1],
    };
  });

  let usable = $derived(!!calib);

  function worldToPix(p: TelemetryPacket): [number, number] {
    const c = calib!;
    return [c.mX * p.positionX + c.bX, c.mZ * p.positionZ + c.bY];
  }

  let host = $state<HTMLDivElement | null>(null);
  let L: typeof import('leaflet') | null = null;
  let map: LMap | null = null;
  let tiles: TileLayer | null = null;
  let traceLayer: LayerGroup | null = null;

  onMount(async () => {
    await initMap();
    redraw();
  });

  // Re-initialize map if settings become available after mount
  // (e.g. spectator page loads settings async after component mounts)
  $effect(() => {
    void cfg; // track settings changes
    if (!map && host && usable) {
      initMap().then(() => redraw());
    }
  });

  async function initMap() {
    if (map || !usable || !host) return;
    L = await import('leaflet');
    await import('leaflet/dist/leaflet.css');

    map = L.map(host, {
      crs: xyzSimpleCRS(L),
      attributionControl: false,
      zoomControl: !compact,
      minZoom: cfg.minZoom,
      maxZoom: cfg.viewMaxZoom,
      maxBoundsViscosity: 1.0,
    });
    tiles = L.tileLayer(cfg.tileUrl, {
      minZoom: cfg.minZoom,
      maxZoom: cfg.viewMaxZoom,
      maxNativeZoom: cfg.maxZoom,
      tileSize: cfg.tileSize,
      noWrap: true,
    }).addTo(map);
    traceLayer = L.layerGroup().addTo(map);
    map.setView(
      map.unproject(L.point(cfg.defaultCenter[0], cfg.defaultCenter[1]), cfg.maxZoom),
      cfg.defaultZoom
    );
  }

  onDestroy(() => {
    map?.remove();
    map = null;
  });

  function pixToLatLng(px: [number, number]) {
    return map!.unproject(L!.point(px[0], px[1]), cfg.maxZoom);
  }

  function redraw() {
    if (!map || !L || !traceLayer || !calib) return;
    traceLayer.clearLayers();

    const valid = points.filter((p) => p.positionX !== 0 || p.positionZ !== 0);

    // Draw polyline trace (solo/replay mode)
    if (valid.length > 1 && drawLine) {
      // One polyline per lap so each lap carries its own colour.
      let seg: ReturnType<typeof pixToLatLng>[] = [];
      let lap = valid[0].lapNumber;
      const flush = () => {
        if (seg.length > 1) {
          L!.polyline(seg, {
            color: colorByLap ? LAP_COLORS[lap % LAP_COLORS.length] : '#3b82f6',
            weight: compact ? 2 : 3,
            opacity: 0.9,
          }).addTo(traceLayer!);
        }
      };
      for (const p of valid) {
        if (colorByLap && p.lapNumber !== lap && seg.length) {
          flush();
          seg = [seg[seg.length - 1]];
          lap = p.lapNumber;
        }
        seg.push(pixToLatLng(worldToPix(p)));
      }
      flush();
    }

    if (multiplayers && multiplayers.length > 0) {
      // Draw markers for each multiplayer participant
      const activeLatLngs: ReturnType<typeof pixToLatLng>[] = [];
      for (const p of multiplayers) {
        if (p.packet && (p.packet.positionX !== 0 || p.packet.positionZ !== 0)) {
          const ll = pixToLatLng(worldToPix(p.packet));
          activeLatLngs.push(ll);

          const headingDeg = ((p.packet.yaw * 180) / Math.PI) % 360;
          const sz = compact ? 20 : 26;
          const icon = L.divIcon({
            className: 'player-arrow-multi',
            html: `<div style="position: relative; display: flex; flex-direction: column; align-items: center; pointer-events: none;">` +
                  `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24">` +
                  `<path transform="rotate(${headingDeg} 12 12)" ` +
                  `d="M12 2 L19 21 L12 15 L5 21 Z" fill="${p.color}" ` +
                  `stroke="#000" stroke-width="1.5" stroke-linejoin="round"/></svg>` +
                  `<span style="background: rgba(10, 15, 30, 0.85); color: #fff; font-size: 8px; font-weight: 700; border-radius: 4px; padding: 1px 4px; white-space: nowrap; margin-top: 1px; border: 1px solid ${p.color}; max-width: 70px; overflow: hidden; text-overflow: ellipsis; box-shadow: 0 2px 4px rgba(0,0,0,0.5);">${p.name}</span>` +
                  `</div>`,
            iconSize: [sz, sz + 15],
            iconAnchor: [sz / 2, sz / 2],
          });
          L!.marker(ll, { icon, interactive: false }).addTo(traceLayer!);
        }
      }

      // Frame all active drivers on the map
      if (activeLatLngs.length > 0) {
        clearBounds();
        const b = L.latLngBounds(activeLatLngs);
        map.fitBounds(b, { padding: [30, 30], maxZoom: cfg.defaultZoom });
      }
    } else {
      const mi =
        currentIndex >= 0 && currentIndex < points.length
          ? currentIndex
          : valid.length - 1;
      const mp = points[mi] ?? valid[valid.length - 1];
      if (mp && (mp.positionX !== 0 || mp.positionZ !== 0)) {
        const ll = pixToLatLng(worldToPix(mp));
        const headingDeg = ((mp.yaw * 180) / Math.PI) % 360;
        const sz = compact ? 22 : 28;
        const icon = L.divIcon({
          className: 'player-arrow',
          html:
            `<svg width="${sz}" height="${sz}" viewBox="0 0 24 24">` +
            `<path transform="rotate(${headingDeg} 12 12)" ` +
            `d="M12 2 L19 21 L12 15 L5 21 Z" fill="#fbbf24" ` +
            `stroke="#000" stroke-width="1.5" stroke-linejoin="round"/></svg>`,
          iconSize: [sz, sz],
          iconAnchor: [sz / 2, sz / 2],
        });
        L.marker(ll, { icon, interactive: false }).addTo(traceLayer);

        if (fixedTrace && valid.length > 1) {
          if (!boundsApplied) {
            const b = L.latLngBounds(valid.map((p) => pixToLatLng(worldToPix(p))));
            map.fitBounds(b, { padding: [20, 20], maxZoom: cfg.defaultZoom });
            map.setMinZoom(map.getZoom());
            map.setMaxBounds(b.pad(0.05));
            boundsApplied = true;
          }
        } else if (drawLine && valid.length > 1) {
          clearBounds();
          const b = L.latLngBounds(valid.map((p) => pixToLatLng(worldToPix(p))));
          map.fitBounds(b, { padding: [20, 20], maxZoom: cfg.defaultZoom });
        } else {
          clearBounds();
          map.setView(ll, map.getZoom(), { animate: false });
        }
      }
    }
  }

  let boundsApplied = false;
  function clearBounds() {
    if (!boundsApplied || !map) return;
    map.setMinZoom(cfg.minZoom);
    map.setMaxBounds(undefined);
    boundsApplied = false;
  }

  // Re-render trace/marker on data changes; keep the user's pan/zoom (only the
  // very first paint auto-fits).
  $effect(() => {
    void points;
    void currentIndex;
    void drawLine;
    // IMPORTANT: multiplayers must be tracked here too so the map redraws
    // when player positions update in spectator/lobby mode.
    void multiplayers;
    if (map) redraw();
  });
</script>

{#if usable}
  <div class="map-host" class:compact bind:this={host}></div>
{:else}
  <!-- No tile URL or no calibration → plain vector trace. -->
  <TrackMap {points} {currentIndex} {colorByLap} {drawLine} {compact} />
{/if}

<style>
  .map-host {
    width: 100%;
    height: 100%;
    flex: 1;
    min-height: 0;
    border-radius: 4px;
    overflow: hidden;
    background: var(--bg-card);
  }
  .map-host.compact {
    height: 100%;
  }
  :global(.leaflet-container) {
    background: var(--bg-card);
    font: inherit;
  }
  /* Strip Leaflet's default divIcon box so only the arrow shows. */
  :global(.player-arrow), :global(.player-arrow-multi) {
    background: none;
    border: none;
  }
</style>
