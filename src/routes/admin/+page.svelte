<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { currentUser, checkAuth } from '$lib/stores/auth';

  let loading = $state(true);
  let activeTab = $state<'stats' | 'users' | 'rooms' | 'logs'>('stats');

  // Data state
  let stats = $state<any>(null);
  let users = $state<any[]>([]);
  let rooms = $state<any[]>([]);
  let logs = $state<any[]>([]);
  
  let refreshInterval: any;

  async function fetchStats() {
    try {
      const res = await fetch('/api/admin/stats');
      if (res.ok) stats = await res.json();
    } catch {}
  }

  async function fetchUsers() {
    try {
      const res = await fetch('/api/admin/users');
      if (res.ok) users = await res.json();
    } catch {}
  }

  async function fetchRooms() {
    try {
      const res = await fetch('/api/admin/rooms');
      if (res.ok) rooms = await res.json();
    } catch {}
  }

  async function fetchLogs() {
    try {
      const res = await fetch('/api/admin/audit-logs');
      if (res.ok) logs = await res.json();
    } catch {}
  }

  async function refreshData() {
    if (activeTab === 'stats') await fetchStats();
    if (activeTab === 'users') await fetchUsers();
    if (activeTab === 'rooms') await fetchRooms();
    if (activeTab === 'logs') await fetchLogs();
  }

  async function changeRole(userId: number, newRole: string) {
    if (!confirm(`Change user ID ${userId} to role ${newRole}?`)) return;
    try {
      await fetch('/api/admin/users/role', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ userId, role: newRole })
      });
      await fetchUsers();
    } catch (err) {
      alert("Failed to change role");
    }
  }

  async function deleteRoom(roomCode: string) {
    if (!confirm(`Force close room ${roomCode}? All players will be kicked.`)) return;
    try {
      await fetch('/api/admin/rooms/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ roomCode })
      });
      await fetchRooms();
      await fetchStats();
    } catch (err) {
      alert("Failed to delete room");
    }
  }

  onMount(async () => {
    const user = await checkAuth();
    if (!user) {
      goto('/login');
      return;
    }
    if (user.role !== 'admin') {
      alert("Unauthorized: Admin access only");
      goto('/');
      return;
    }
    
    await refreshData();
    loading = false;
    refreshInterval = setInterval(refreshData, 5000);
  });

  onDestroy(() => {
    clearInterval(refreshInterval);
  });
  
  // Effect to handle tab changes
  $effect(() => {
    if (activeTab && !loading) refreshData();
  });
</script>

<svelte:head>
  <title>Admin Dashboard – FH6 Telemetry</title>
</svelte:head>

<div class="page">
  <div class="sidebar">
    <h2>FH6 Admin</h2>
    <div class="nav-links">
      <button class:active={activeTab === 'stats'} onclick={() => activeTab = 'stats'}>📊 System Stats</button>
      <button class:active={activeTab === 'users'} onclick={() => activeTab = 'users'}>👥 User Management</button>
      <button class:active={activeTab === 'rooms'} onclick={() => activeTab = 'rooms'}>🎮 Active Rooms</button>
      <button class:active={activeTab === 'logs'} onclick={() => activeTab = 'logs'}>🛡️ Audit Logs</button>
      <a href="/" class="home-btn">← Back to App</a>
    </div>
  </div>

  <div class="content">
    {#if loading}
      <div class="spinner-container">
        <div class="spinner"></div>
      </div>
    {:else}
      {#if activeTab === 'stats'}
        <div class="section-title">
          <h1>System Overview</h1>
        </div>
        <div class="stats-grid">
          <div class="stat-card">
            <h3>Total Users</h3>
            <div class="stat-value">{stats?.totalUsers || 0}</div>
          </div>
          <div class="stat-card">
            <h3>Active Rooms</h3>
            <div class="stat-value">{stats?.activeRooms || 0}</div>
          </div>
          <div class="stat-card">
            <h3>Total Race Sessions</h3>
            <div class="stat-value">{stats?.totalSessions || 0}</div>
          </div>
          <div class="stat-card">
            <h3>Total Packets Logged</h3>
            <div class="stat-value">{stats?.totalPackets || 0}</div>
          </div>
        </div>
      {/if}

      {#if activeTab === 'users'}
        <div class="section-title">
          <h1>User Management</h1>
        </div>
        <div class="table-container">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Username</th>
                <th>Email</th>
                <th>Registered</th>
                <th>Role</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {#each users as u}
                <tr>
                  <td>{u.id}</td>
                  <td class="bold">{u.username}</td>
                  <td>{u.email}</td>
                  <td>{new Date(u.createdAt).toLocaleDateString()}</td>
                  <td>
                    <span class="role-badge {u.role}">{u.role}</span>
                  </td>
                  <td>
                    {#if u.id !== $currentUser?.id}
                      {#if u.role === 'admin'}
                        <button class="demote-btn" onclick={() => changeRole(u.id, 'user')}>Demote</button>
                      {:else}
                        <button class="promote-btn" onclick={() => changeRole(u.id, 'admin')}>Make Admin</button>
                      {/if}
                    {:else}
                      <span class="self-tag">(You)</span>
                    {/if}
                  </td>
                </tr>
              {/each}
              {#if users.length === 0}
                <tr><td colspan="6" class="empty">No users found</td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}

      {#if activeTab === 'rooms'}
        <div class="section-title">
          <h1>Active Multiplayer Rooms</h1>
        </div>
        <div class="table-container">
          <table>
            <thead>
              <tr>
                <th>Room Code</th>
                <th>Host</th>
                <th>Visibility</th>
                <th>Players</th>
                <th>Recording</th>
                <th>Uptime</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {#each rooms as r}
                <tr>
                  <td class="code-col">{r.code}</td>
                  <td class="bold">{r.hostUsername}</td>
                  <td>
                    {#if r.isPublic}
                      <span class="badge public">Public</span>
                    {:else}
                      <span class="badge private">Private</span>
                    {/if}
                  </td>
                  <td>{r.playerCount} / 12</td>
                  <td>
                    {#if r.isRecording}
                      <span class="badge rec">● REC</span>
                    {:else}
                      <span class="badge off">Off</span>
                    {/if}
                  </td>
                  <td>{Math.floor((Date.now() - r.createdAt) / 60000)} mins</td>
                  <td>
                    <button class="kill-btn" onclick={() => deleteRoom(r.code)}>Force Close</button>
                  </td>
                </tr>
              {/each}
              {#if rooms.length === 0}
                <tr><td colspan="7" class="empty">No active rooms found</td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}

      {#if activeTab === 'logs'}
        <div class="section-title">
          <h1>Security Audit Logs</h1>
        </div>
        <div class="table-container">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Username</th>
                <th>Action</th>
                <th>Target Info</th>
                <th>IP Address</th>
              </tr>
            </thead>
            <tbody>
              {#each logs as log}
                <tr>
                  <td>{new Date(log.timestampMs).toLocaleString()}</td>
                  <td class="bold">{log.username || 'System'}</td>
                  <td><span class="badge {log.action.includes('LOGIN') ? 'public' : log.action.includes('DELETE') ? 'rec' : 'private'}">{log.action}</span></td>
                  <td>{log.target}</td>
                  <td class="code-col">{log.ipAddress}</td>
                </tr>
              {/each}
              {#if logs.length === 0}
                <tr><td colspan="5" class="empty">No audit logs found</td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .page {
    display: flex;
    height: 100vh;
    background: #0a0a0a;
    color: #e5e7eb;
    font-family: 'Inter', system-ui, sans-serif;
  }

  .sidebar {
    width: 250px;
    background: #111;
    border-right: 1px solid #222;
    padding: 2rem 1rem;
    display: flex;
    flex-direction: column;
  }
  .sidebar h2 {
    margin: 0 0 2rem;
    font-size: 1.25rem;
    font-weight: 800;
    color: #f3f4f6;
    padding-left: 0.5rem;
    letter-spacing: 0.05em;
  }
  .nav-links {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .nav-links button, .nav-links a {
    text-align: left;
    padding: 0.8rem 1rem;
    background: transparent;
    border: none;
    border-radius: 8px;
    color: #9ca3af;
    font-size: 0.95rem;
    font-weight: 600;
    cursor: pointer;
    text-decoration: none;
    transition: all 0.2s;
  }
  .nav-links button:hover, .nav-links a:hover {
    background: rgba(255,255,255,0.05);
    color: #f3f4f6;
  }
  .nav-links button.active {
    background: rgba(59, 130, 246, 0.15);
    color: #3b82f6;
  }
  .home-btn {
    margin-top: 2rem;
    border-top: 1px solid #222 !important;
    border-radius: 0 !important;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 2.5rem;
  }

  .section-title h1 {
    font-size: 1.8rem;
    font-weight: 700;
    margin: 0 0 2rem;
    color: #f9fafb;
  }

  .spinner-container {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
  }
  .spinner {
    width: 40px; height: 40px;
    border: 3px solid rgba(59,130,246,0.2);
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1.5rem;
  }
  .stat-card {
    background: #171717;
    border: 1px solid #262626;
    border-radius: 12px;
    padding: 1.5rem;
  }
  .stat-card h3 {
    margin: 0 0 0.5rem;
    font-size: 0.85rem;
    color: #a3a3a3;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .stat-value {
    font-size: 2.2rem;
    font-weight: 800;
    color: #f3f4f6;
  }

  .table-container {
    background: #171717;
    border: 1px solid #262626;
    border-radius: 12px;
    overflow: hidden;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
  }
  th, td {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid #262626;
  }
  th {
    background: #111;
    color: #a3a3a3;
    font-size: 0.8rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  td {
    font-size: 0.9rem;
    color: #d4d4d8;
  }
  .bold { font-weight: 600; color: #f3f4f6; }
  .code-col { font-family: monospace; font-size: 1rem; color: #93c5fd; }
  .empty { text-align: center; color: #71717a; padding: 2rem; }

  .badge {
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 700;
  }
  .badge.public { background: rgba(34,197,94,0.1); color: #4ade80; }
  .badge.private { background: rgba(161,161,170,0.1); color: #a1a1aa; }
  .badge.rec { background: rgba(239,68,68,0.1); color: #ef4444; animation: pulse 2s infinite; }
  .badge.off { color: #52525b; }
  
  .role-badge {
    padding: 0.2rem 0.6rem;
    border-radius: 99px;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
  }
  .role-badge.admin { background: #3b82f6; color: #fff; }
  .role-badge.user { background: #3f3f46; color: #d4d4d8; }

  button {
    padding: 0.4rem 0.8rem;
    border: none;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.2s;
  }
  button:hover { opacity: 0.8; }
  .promote-btn { background: #3b82f6; color: white; }
  .demote-btn { background: #52525b; color: white; }
  .kill-btn { background: #ef4444; color: white; }
  .self-tag { color: #71717a; font-size: 0.8rem; font-style: italic; }

  @keyframes pulse { 50% { opacity: 0.5; } }
</style>
