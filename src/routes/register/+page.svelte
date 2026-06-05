<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { register, checkAuth, verifyAccount } from '$lib/stores/auth';

  let username = $state('');
  let email = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let verificationToken = $state('');
  let errorMsg = $state('');
  let successMsg = $state('');
  let loading = $state(false);
  let showVerification = $state(false);
  let turnstileEnabled = $state(false);
  let turnstileSiteKey = $state('');

  onMount(async () => {
    try {
      const configRes = await fetch('/api/config');
      const config = await configRes.json();
      turnstileEnabled = config.turnstileEnabled;
      turnstileSiteKey = config.turnstileSiteKey;
    } catch (e) {
      console.error('Failed to load config', e);
    }

    // If already authenticated, skip registration
    const user = await checkAuth();
    if (user) {
      goto('/');
    }
  });

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!username || !email || !password || !confirmPassword) {
      errorMsg = 'Please fill out all fields';
      return;
    }
    if (username.length < 3) {
      errorMsg = 'Username must be at least 3 characters long';
      return;
    }
    if (password.length < 8) {
      errorMsg = 'Password must be at least 8 characters long';
      return;
    }
    if (password !== confirmPassword) {
      errorMsg = 'Passwords do not match';
      return;
    }

    errorMsg = '';
    successMsg = '';
    loading = true;

    let turnstileToken = '';
    if (turnstileEnabled) {
      const formData = new FormData(e.target as HTMLFormElement);
      turnstileToken = formData.get('cf-turnstile-response') as string;
      if (!turnstileToken) {
        errorMsg = 'Please complete the Captcha';
        loading = false;
        return;
      }
    }

    try {
      const res = await register(username, email, password, turnstileToken);
      if (res.needsVerification) {
          showVerification = true;
          successMsg = 'Registration successful! Please check your email for the verification code.';
      } else {
          successMsg = 'Registration successful! Redirecting to sign in...';
          setTimeout(() => {
            goto('/login');
          }, 2000);
      }
    } catch (err: any) {
      errorMsg = err.message || 'Registration failed';
      loading = false;
    }
  }

  async function handleVerify(e: SubmitEvent) {
    e.preventDefault();
    if (!verificationToken) {
      errorMsg = 'Please enter the verification code';
      return;
    }

    errorMsg = '';
    successMsg = '';
    loading = true;

    try {
      await verifyAccount(email, verificationToken);
      successMsg = 'Account verified! Redirecting to sign in...';
      setTimeout(() => {
        goto('/login');
      }, 2000);
    } catch (err: any) {
      errorMsg = err.message || 'Verification failed';
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Register - FH6 Telemetry</title>
  {#if turnstileEnabled}
    <script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
  {/if}
</svelte:head>

<div class="auth-container">
  <!-- Glowing decorative blobs for background depth -->
  <div class="glow-sphere sphere-1"></div>
  <div class="glow-sphere sphere-2"></div>

  <div class="auth-card">
    <div class="logo">
      <span class="logo-accent">FH6</span> TELEMETRY
    </div>
    <h2>Driver Registration</h2>
    <p class="subtitle">Create an account to join lobbies and track telemetry</p>

    {#if showVerification}
    <form onsubmit={handleVerify}>
      {#if errorMsg}
        <div class="banner error-banner">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="banner-icon">
            <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
          </svg>
          <span>{errorMsg}</span>
        </div>
      {/if}

      {#if successMsg}
        <div class="banner success-banner">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="banner-icon">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
          </svg>
          <span>{successMsg}</span>
        </div>
      {/if}
      <div class="input-group">
        <label for="verificationToken">Verification Code</label>
        <input
          type="text"
          id="verificationToken"
          bind:value={verificationToken}
          placeholder="6-digit code"
          required
          disabled={loading || successMsg.includes('Redirecting')}
        />
      </div>
      <button type="submit" class="submit-btn" disabled={loading || successMsg.includes('Redirecting')}>
        {#if loading}
          <div class="spinner"></div>
        {:else}
          Verify Account
        {/if}
      </button>
    </form>
    {:else}
    <form onsubmit={handleSubmit}>
      {#if errorMsg}
        <div class="banner error-banner">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="banner-icon">
            <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
          </svg>
          <span>{errorMsg}</span>
        </div>
      {/if}

      {#if successMsg}
        <div class="banner success-banner">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="banner-icon">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
          </svg>
          <span>{successMsg}</span>
        </div>
      {/if}

      <div class="input-group">
        <label for="username">Driver Name</label>
        <input
          type="text"
          id="username"
          bind:value={username}
          placeholder="e.g. LewisH"
          required
          disabled={loading || successMsg !== ''}
        />
      </div>

      <div class="input-group">
        <label for="email">Email Address</label>
        <input
          type="email"
          id="email"
          bind:value={email}
          placeholder="driver@horizon.com"
          required
          disabled={loading || successMsg !== ''}
        />
      </div>

      <div class="input-group">
        <label for="password">Password</label>
        <input
          type="password"
          id="password"
          bind:value={password}
          placeholder="Min 8 characters"
          required
          disabled={loading || successMsg !== ''}
        />
      </div>

      <div class="input-group">
        <label for="confirm-password">Confirm Password</label>
        <input
          type="password"
          id="confirm-password"
          bind:value={confirmPassword}
          placeholder="Repeat password"
          required
          disabled={loading || successMsg !== ''}
        />
      </div>

      {#if turnstileEnabled && turnstileSiteKey}
        <div class="input-group turnstile-wrapper">
          <div class="cf-turnstile" data-sitekey={turnstileSiteKey} data-theme="dark"></div>
        </div>
      {/if}

      <button type="submit" class="submit-btn" disabled={loading || successMsg !== ''}>
        {#if loading}
          <div class="spinner"></div>
        {:else}
          Register Account
        {/if}
      </button>
    </form>
    {/if}

    <div class="footer-links">
      Already registered? <a href="/login">Sign In</a>
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
    background: #3b82f6;
    animation: pulseGlow 8s infinite alternate ease-in-out;
  }
  .sphere-2 {
    bottom: 15%;
    right: 20%;
    width: 400px;
    height: 400px;
    background: #8b5cf6;
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

  /* Banners */
  .banner {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    margin-bottom: 1.25rem;
    border-radius: 8px;
    font-size: 0.85rem;
  }
  .banner-icon {
    width: 20px;
    height: 20px;
    flex-shrink: 0;
  }
  .error-banner {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #fca5a5;
  }
  .success-banner {
    background: rgba(16, 185, 129, 0.1);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #a7f3d0;
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
