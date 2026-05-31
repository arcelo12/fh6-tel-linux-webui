<script lang="ts">
  import { pendingTelemetry } from '$lib/stores/telemetry';
  import { carName } from '$lib/car-name';
  import { CAR_CLASS_LABELS } from '$lib/types';

  let { onOpenSettings }: { onOpenSettings: () => void } = $props();

  let data = $derived($pendingTelemetry);
  let carLabel = $derived(data ? carName(data.carOrdinal) : '—');
  let classLabel = $derived(data ? (CAR_CLASS_LABELS[data.carClass] ?? '?') : '—');
  let piLabel = $derived(data ? String(data.carPi) : '—');

  let confirming = $state(false);
  let rejecting = $state(false);

  async function confirmStream() {
    if (!data) return;
    confirming = true;
    try {
      const res = await fetch('/api/user/port/confirm', { method: 'POST' });
      if (res.ok) {
        pendingTelemetry.set(null);
      }
    } catch (e) {
      console.error(e);
    } finally {
      confirming = false;
    }
  }

  async function rejectStream() {
    if (!data) return;
    rejecting = true;
    try {
      const res = await fetch('/api/user/port/reject', { method: 'POST' });
      if (res.ok) {
        pendingTelemetry.set(null);
        onOpenSettings(); // Open settings so they can change the port
      }
    } catch (e) {
      console.error(e);
    } finally {
      rejecting = false;
    }
  }
</script>

{#if data}
  <div class="overlay" role="dialog" aria-modal="true">
    <div class="modal">
      <div class="header">
        <span class="pulse-dot"></span>
        <h3>Data Telemetri Terdeteksi</h3>
      </div>
      
      <p class="description">
        Ada sinyal data game Forza Horizon masuk di port <strong>{data.port}</strong> Anda dari IP <code>{data.clientIp}</code>.
      </p>

      <div class="car-preview">
        <div class="car-title">{carLabel}</div>
        <div class="badges">
          <span class="badge class-badge" data-class={classLabel}>{classLabel}</span>
          <span class="badge">{piLabel} PI</span>
        </div>
      </div>

      <p class="prompt-text">Apakah ini data dari game Anda?</p>

      <div class="actions">
        <button 
          class="btn-reject" 
          disabled={confirming || rejecting} 
          onclick={rejectStream}
        >
          Bukan, Ubah Port
        </button>
        <button 
          class="btn-confirm" 
          disabled={confirming || rejecting} 
          onclick={confirmStream}
        >
          {confirming ? 'Menghubungkan...' : 'Ya, Sambungkan'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.8);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    animation: fadeIn 0.2s ease-out;
  }

  .modal {
    background: linear-gradient(135deg, var(--bg-card) 0%, var(--bg-elevated) 100%);
    border: 1px solid var(--bd-strong);
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.5), 0 0 15px rgba(21, 101, 192, 0.15);
    border-radius: 12px;
    padding: 1.5rem;
    width: 90%;
    max-width: 440px;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    animation: slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    border-bottom: 1px solid var(--bd-dim);
    padding-bottom: 0.75rem;
  }

  h3 {
    margin: 0;
    color: var(--tx-hi);
    font-size: 1.15rem;
    font-weight: 600;
  }

  .pulse-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--ac, #3b82f6);
    box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7);
    animation: pulse 1.6s infinite;
  }

  .description {
    margin: 0;
    color: var(--tx-mid);
    font-size: 0.9rem;
    line-height: 1.4;
  }

  .car-preview {
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--bd-subtle);
    border-radius: 8px;
    padding: 0.75rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .car-title {
    color: var(--tx-hi);
    font-weight: 600;
    font-size: 0.95rem;
  }

  .badges {
    display: flex;
    gap: 0.4rem;
  }

  .badge {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 0.1rem 0.4rem;
    border: 1px solid var(--bd-muted);
    border-radius: 3px;
    color: var(--tx-lo);
  }

  .class-badge[data-class="X"]  { color: #ef4444; border-color: #7f1d1d; }
  .class-badge[data-class="S2"] { color: #f97316; border-color: #7c2d12; }
  .class-badge[data-class="S1"] { color: #eab308; border-color: #713f12; }
  .class-badge[data-class="A"]  { color: #22c55e; border-color: #14532d; }
  .class-badge[data-class="B"]  { color: #3b82f6; border-color: #1e3a5f; }
  .class-badge[data-class="C"]  { color: #a855f7; border-color: #4c1d95; }

  .prompt-text {
    margin: 0.25rem 0 0 0;
    color: var(--tx-lo);
    font-weight: 600;
    font-size: 0.9rem;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }

  button {
    padding: 0.5rem 1.1rem;
    border-radius: 6px;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-reject {
    background: transparent;
    border: 1px solid var(--bd-muted);
    color: var(--tx-lo);
  }

  .btn-reject:hover:not(:disabled) {
    background: rgba(239, 68, 68, 0.08);
    border-color: rgba(239, 68, 68, 0.3);
    color: #f87171;
  }

  .btn-confirm {
    background: var(--ac, #3b82f6);
    border: 1px solid var(--ac, #3b82f6);
    color: #fff;
  }

  .btn-confirm:hover:not(:disabled) {
    filter: brightness(1.15);
    box-shadow: 0 0 10px rgba(59, 130, 246, 0.4);
  }

  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes slideUp {
    from { transform: translateY(15px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }

  @keyframes pulse {
    0% {
      transform: scale(0.95);
      box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7);
    }
    70% {
      transform: scale(1);
      box-shadow: 0 0 0 6px rgba(59, 130, 246, 0);
    }
    100% {
      transform: scale(0.95);
      box-shadow: 0 0 0 0 rgba(59, 130, 246, 0);
    }
  }
</style>
