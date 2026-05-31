<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { login, checkAuth } from '$lib/stores/auth';

  let email = $state('');
  let password = $state('');
  let errorMsg = $state('');
  let loading = $state(false);

  onMount(async () => {
    // If already authenticated, skip login page
    const user = await checkAuth();
    if (user) {
      goto('/');
    }
  });

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!email || !password) {
      errorMsg = 'Please enter both email and password';
      return;
    }
    errorMsg = '';
    loading = true;
    try {
      await login(email, password);
      goto('/');
    } catch (err: any) {
      errorMsg = err.message || 'Login failed';
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Login - FH6 Telemetry</title>
</svelte:head>

<div class="auth-container">
  <!-- Glowing decorative blobs for background depth -->
  <div class="glow-sphere sphere-1"></div>
  <div class="glow-sphere sphere-2"></div>

  <div class="auth-card">
    <div class="logo">
      <span class="logo-accent">FH6</span> TELEMETRY
    </div>
    <h2>Welcome Back</h2>
    <p class="subtitle">Sign in to track, record, and share telemetry logs</p>

    <form onsubmit={handleSubmit}>
      {#if errorMsg}
        <div class="error-banner">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="error-icon">
            <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
          </svg>
          <span>{errorMsg}</span>
        </div>
      {/if}

      <div class="input-group">
        <label for="email">Email Address</label>
        <input
          type="email"
          id="email"
          bind:value={email}
          placeholder="driver@horizon.com"
          required
          disabled={loading}
        />
      </div>

      <div class="input-group">
        <label for="password">Password</label>
        <input
          type="password"
          id="password"
          bind:value={password}
          placeholder="••••••••"
          required
          disabled={loading}
        />
      </div>

      <button type="submit" class="submit-btn" disabled={loading}>
        {#if loading}
          <div class="spinner"></div>
        {:else}
          Sign In
        {/if}
      </button>
    </form>

    <div class="footer-links">
      New driver? <a href="/register">Create an account</a>
    </div>
  </div>
</div>

<style>
  .auth-container {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    min-height: 100dvh;
    width: 100vw;
    background: #030712;
    overflow: hidden;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
  }

  /* Decorative Glows */
  .glow-sphere {
    position: absolute;
    border-radius: 50%;
    filter: blur(120px);
    opacity: 0.15;
    z-index: 1;
    pointer-events: none;
  }
  .sphere-1 {
    top: 15%;
    left: 20%;
    width: 350px;
    height: 350px;
    background: #3b82f6; /* Blue glow */
    animation: pulseGlow 8s infinite alternate ease-in-out;
  }
  .sphere-2 {
    bottom: 15%;
    right: 20%;
    width: 400px;
    height: 400px;
    background: #8b5cf6; /* Purple glow */
    animation: pulseGlow 10s infinite alternate-reverse ease-in-out;
  }

  @keyframes pulseGlow {
    0% { transform: scale(1) translate(0, 0); opacity: 0.15; }
    100% { transform: scale(1.2) translate(20px, -20px); opacity: 0.22; }
  }

  /* Glassmorphism Card */
  .auth-card {
    position: relative;
    z-index: 10;
    width: 100%;
    max-width: 420px;
    padding: 2.5rem;
    margin: 1.5rem;
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 16px;
    backdrop-filter: blur(20px);
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
    text-align: center;
    animation: fadeInCard 0.6s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes fadeInCard {
    0% { transform: translateY(20px); opacity: 0; }
    100% { transform: translateY(0); opacity: 1; }
  }

  .logo {
    font-size: 1.5rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    color: #f3f4f6;
    margin-bottom: 1.5rem;
  }
  .logo-accent {
    color: #3b82f6;
    text-shadow: 0 0 15px rgba(59, 130, 246, 0.5);
  }

  h2 {
    font-size: 1.65rem;
    font-weight: 700;
    color: #f9fafb;
    margin-bottom: 0.5rem;
  }

  .subtitle {
    font-size: 0.88rem;
    color: #9ca3af;
    margin-bottom: 2rem;
    line-height: 1.4;
  }

  form {
    text-align: left;
  }

  /* Error Banner */
  .error-banner {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    margin-bottom: 1.25rem;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 8px;
    color: #fca5a5;
    font-size: 0.85rem;
  }
  .error-icon {
    width: 20px;
    height: 20px;
    flex-shrink: 0;
  }

  .input-group {
    margin-bottom: 1.25rem;
  }

  label {
    display: block;
    font-size: 0.78rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #9ca3af;
    margin-bottom: 0.4rem;
  }

  input {
    width: 100%;
    padding: 0.75rem 1rem;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    color: #f3f4f6;
    font-size: 0.92rem;
    transition: all 0.25s ease;
  }
  input:focus {
    outline: none;
    border-color: #3b82f6;
    background: rgba(255, 255, 255, 0.06);
    box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
  }
  input::placeholder {
    color: #4b5563;
  }
  input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .submit-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    padding: 0.8rem;
    margin-top: 1.75rem;
    background: linear-gradient(135deg, #3b82f6, #2563eb);
    border: none;
    border-radius: 8px;
    color: #ffffff;
    font-size: 0.95rem;
    font-weight: 600;
    cursor: pointer;
    box-shadow: 0 4px 15px rgba(59, 130, 246, 0.3);
    transition: all 0.25s ease;
  }
  .submit-btn:hover:not(:disabled) {
    transform: translateY(-1px);
    box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
    filter: brightness(1.1);
  }
  .submit-btn:active:not(:disabled) {
    transform: translateY(0);
  }
  .submit-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    box-shadow: none;
  }

  .spinner {
    width: 18px;
    height: 18px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: #ffffff;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .footer-links {
    margin-top: 1.75rem;
    font-size: 0.85rem;
    color: #9ca3af;
  }
  .footer-links a {
    color: #3b82f6;
    text-decoration: none;
    font-weight: 600;
    transition: color 0.2s ease;
  }
  .footer-links a:hover {
    color: #60a5fa;
    text-decoration: underline;
  }
</style>
