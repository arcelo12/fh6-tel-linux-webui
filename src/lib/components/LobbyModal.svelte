<script lang="ts">
  import { goto } from '$app/navigation';
  
  let { onClose }: { onClose: () => void } = $props();

  let roomCodeInput = $state('');
  let loading = $state(false);
  let errorMsg = $state('');

  async function createLobby() {
    loading = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/lobby/create', { method: 'POST' });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      onClose();
      goto(`/lobby/${data.code}`);
    } catch (err: any) {
      errorMsg = err.message || 'Failed to create lobby';
    } finally {
      loading = false;
    }
  }

  function joinLobby(e: SubmitEvent) {
    e.preventDefault();
    const code = roomCodeInput.trim().toUpperCase();
    if (code.length !== 6) {
      errorMsg = 'Room code must be exactly 6 characters';
      return;
    }
    onClose();
    goto(`/lobby/${code}`);
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="modal-backdrop" onclick={onClose} role="dialog" aria-modal="true">
  <div class="modal-content" onclick={(e) => e.stopPropagation()} role="document">
    <div class="modal-hdr">
      <h3>👥 Multiplayer Telemetry</h3>
      <button class="close-btn" onclick={onClose} aria-label="Close dialog">✕</button>
    </div>

    {#if errorMsg}
      <div class="error-msg">{errorMsg}</div>
    {/if}

    <div class="modal-body">
      <!-- Create Room Section -->
      <div class="section-card">
        <h4>Host a Race</h4>
        <p class="desc">Create a new lobby and share the room code with up to 11 other players to stream and record telemetry concurrently.</p>
        <button class="action-btn create-btn" onclick={createLobby} disabled={loading}>
          {loading ? 'Creating...' : 'Create New Room'}
        </button>
      </div>

      <div class="divider">
        <span>OR</span>
      </div>

      <!-- Join Room Section -->
      <div class="section-card">
        <h4>Join Existing Room</h4>
        <p class="desc">Enter the 6-character room code shared by the host to claim a slot and connect your Forza telemetry.</p>
        <form onsubmit={joinLobby}>
          <div class="input-group">
            <input
              type="text"
              bind:value={roomCodeInput}
              placeholder="ENTER CODE"
              maxlength="6"
              required
            />
            <button type="submit" class="action-btn join-btn" disabled={loading}>
              Join Room
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(3, 7, 18, 0.75);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 250;
    animation: fadeIn 0.2s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .modal-content {
    background: var(--bg-card, #080e18);
    border: 1px solid var(--bd-subtle, #1e2a3a);
    border-radius: 12px;
    width: 100%;
    max-width: 440px;
    padding: 1.5rem;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5);
    animation: slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes slideUp {
    from { transform: translateY(20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }

  .modal-hdr {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
  }
  .modal-hdr h3 {
    font-size: 1.15rem;
    font-weight: 700;
    color: var(--tx-hi, #f9fafb);
  }
  .close-btn {
    background: none;
    border: none;
    color: var(--tx-dim, #6b7280);
    font-size: 1.1rem;
    cursor: pointer;
    padding: 0.2rem;
  }
  .close-btn:hover {
    color: var(--tx-hi, #f9fafb);
  }

  .error-msg {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 6px;
    color: #fca5a5;
    font-size: 0.8rem;
    padding: 0.5rem;
    margin-bottom: 1rem;
  }

  .modal-body {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .section-card {
    background: rgba(255, 255, 255, 0.01);
    border: 1px solid rgba(255, 255, 255, 0.03);
    border-radius: 8px;
    padding: 1rem;
  }

  h4 {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--tx-mid, #e5e7eb);
    margin-bottom: 0.4rem;
  }

  .desc {
    font-size: 0.78rem;
    color: var(--tx-dim, #6b7280);
    line-height: 1.4;
    margin-bottom: 1rem;
  }

  .divider {
    display: flex;
    align-items: center;
    text-align: center;
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--tx-xdim, #4b5563);
  }
  .divider::before, .divider::after {
    content: '';
    flex: 1;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }
  .divider:not(:empty)::before { margin-right: .5em; }
  .divider:not(:empty)::after { margin-left: .5em; }

  .action-btn {
    width: 100%;
    padding: 0.6rem;
    border: none;
    border-radius: 6px;
    font-size: 0.88rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .create-btn {
    background: var(--ac, #3b82f6);
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(59, 130, 246, 0.2);
  }
  .create-btn:hover:not(:disabled) {
    filter: brightness(1.1);
    transform: translateY(-1px);
  }

  .input-group {
    display: flex;
    gap: 0.5rem;
  }

  .input-group input {
    flex: 1;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 6px;
    padding: 0.5rem 0.75rem;
    color: #ffffff;
    font-size: 0.9rem;
    text-align: center;
    font-weight: 700;
    letter-spacing: 0.05em;
  }
  .input-group input:focus {
    outline: none;
    border-color: var(--ac, #3b82f6);
  }
  .input-group input::placeholder {
    font-weight: 500;
    letter-spacing: 0;
  }

  .join-btn {
    width: auto;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--tx-mid, #e5e7eb);
    padding: 0.5rem 1rem;
  }
  .join-btn:hover:not(:disabled) {
    background: var(--ac, #3b82f6);
    border-color: var(--ac, #3b82f6);
    color: #ffffff;
  }
</style>
