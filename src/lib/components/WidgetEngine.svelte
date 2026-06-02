<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import '$lib/pro-dashboard.css';
  import 'gridstack/dist/gridstack.min.css';
  import { GridStack } from 'gridstack';

  let { 
    widgets, 
    layout, 
    editMode = false,
    onLayoutChange,
    onRemove,
  }: { 
    widgets: Record<string, any>, 
    layout: any[],
    editMode?: boolean,
    onLayoutChange?: (newLayout: any[]) => void,
    onRemove?: (id: string) => void,
  } = $props();

  let gridContainer: HTMLDivElement | null = null;
  let grid: GridStack | null = null;
  // Keep a mutable ref so the setTimeout callback can read the latest editMode
  let editModeRef = editMode;
  $effect(() => { editModeRef = editMode; });

  function applyEditMode(enabled: boolean) {
    if (!grid) return;
    if (enabled) {
      grid.setStatic(false);
      grid.enableMove(true);
      grid.enableResize(true);
    } else {
      grid.enableMove(false);
      grid.enableResize(false);
      grid.setStatic(true);
    }
  }

  onMount(() => {
    if (!gridContainer) return;

    // Wait a tick for Svelte to fully apply gs-* attributes to the DOM
    setTimeout(() => {
      if (!gridContainer) return;
      grid = GridStack.init({
        disableDrag: true,
        disableResize: true,
        staticGrid: true,
        float: true,
        margin: 8,
        cellHeight: 80,
        column: 12,
      }, gridContainer);

      // Apply current editMode after init (it may have changed before setTimeout ran)
      applyEditMode(editModeRef);

      grid.on('change', (e, items) => {
        if (!onLayoutChange) return;
        const updatedLayout = grid!.save();
        onLayoutChange(updatedLayout as any[]);
      });
    }, 10);
    
    return () => {
      if (grid) {
        grid.destroy(false);
        grid = null;
      }
    };
  });

  $effect(() => {
    // editMode changed — apply immediately if grid is ready, else it's handled post-init
    applyEditMode(editMode);
  });
</script>

<div bind:this={gridContainer} class="grid-stack">
  {#each layout as item (item.id)}
    <!-- We don't bind gs-x back to Svelte to prevent collision with GridStack's DOM updates -->
    <div 
      class="grid-stack-item" 
      gs-id={item.id} 
      gs-x={item.x} 
      gs-y={item.y} 
      gs-w={item.w} 
      gs-h={item.h}
    >
      <div class="grid-stack-item-content">
        {#if widgets[item.id]}
          <svelte:component this={widgets[item.id].component} />
        {:else}
          <div class="widget-placeholder">Unknown Widget: {item.id}</div>
        {/if}
        <div class="widget-edit-overlay" class:is-editing={editMode}>
           <div class="widget-title">{widgets[item.id]?.name || item.id}</div>
           <button
             class="widget-delete-btn"
             onclick={(e) => { e.stopPropagation(); onRemove?.(item.id); }}
             title="Remove widget"
           >
             ✕
           </button>
        </div>
      </div>
    </div>
  {/each}
</div>

<style>
  /* Ensure the grid-stack container stays within its parent and never bleeds into footer/header */
  :global(.grid-stack) {
    min-height: 200px;
    width: 100%;
  }
  :global(.grid-stack-item) {
    /* Isolate stacking context so widgets don't float above header/footer */
    isolation: isolate;
  }
  .grid-stack-item-content {
    background: var(--bg-panel, rgba(30, 30, 30, 0.5));
    border-radius: 8px;
    border: 1px solid var(--bd-dim, rgba(255,255,255,0.1));
    overflow: hidden;
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .widget-edit-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 219, 233, 0.1);
    border: 2px dashed #00dbe9;
    z-index: 100;
    pointer-events: auto; /* IMPORTANT: MUST BE AUTO TO CATCH DRAG EVENTS */
    cursor: move;
    display: none; /* hidden by default */
    align-items: center;
    justify-content: center;
  }
  .widget-edit-overlay.is-editing {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .widget-title {
    background: #00dbe9;
    color: #000;
    padding: 2px 10px;
    border-radius: 4px;
    font-weight: bold;
    font-size: 0.8rem;
    pointer-events: none;
  }
  .widget-delete-btn {
    background: rgba(239, 68, 68, 0.9);
    color: #fff;
    border: none;
    border-radius: 4px;
    padding: 2px 10px;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    pointer-events: auto;
    transition: background 0.15s ease, transform 0.1s ease;
    line-height: 1.4;
  }
  .widget-delete-btn:hover {
    background: #ef4444;
    transform: scale(1.08);
  }
  .widget-placeholder {
    padding: 1rem;
    color: #ef4444;
    text-align: center;
  }
</style>
