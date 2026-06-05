import { writable } from 'svelte/store';

export interface UserSession {
  id: number;
  username: string;
  role?: string;
}

let initialUser: UserSession | null = null;
if (typeof window !== 'undefined') {
  try {
    const val = localStorage.getItem('user_session');
    if (val) {
      initialUser = JSON.parse(val);
    }
  } catch {}
}

export const currentUser = writable<UserSession | null>(initialUser);

async function apiRequest<T>(path: string, method: string = 'GET', body?: any): Promise<T> {
  const opts: RequestInit = { method, headers: {} };
  if (body) {
    opts.headers = { 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`/api${path}`, opts);
  if (!res.ok) {
    let errMsg = 'Request failed';
    try {
      const data = await res.json();
      errMsg = data.error || errMsg;
    } catch {
      try {
        const text = await res.text();
        if (text) errMsg = text;
      } catch {}
    }
    throw new Error(errMsg);
  }
  return res.json();
}

export async function checkAuth(): Promise<UserSession | null> {
  try {
    const user = await apiRequest<UserSession>('/auth/me');
    currentUser.set(user);
    if (typeof window !== 'undefined') {
      localStorage.setItem('user_session', JSON.stringify(user));
    }
    return user;
  } catch (err) {
    currentUser.set(null);
    if (typeof window !== 'undefined') {
      localStorage.removeItem('user_session');
    }
    return null;
  }
}

export async function login(email: string, password: string): Promise<UserSession> {
  await apiRequest<{ username: string; message: string }>('/auth/login', 'POST', { email, password });
  const user = await checkAuth();
  if (!user) {
    throw new Error('Failed to retrieve user details after login');
  }
  return user;
}

export async function register(username: string, email: string, password: string, turnstileToken: string = ''): Promise<{ needsVerification: boolean }> {
  const res = await apiRequest<{ needs_verification?: string }>('/auth/register', 'POST', { username, email, password, turnstileToken });
  return { needsVerification: res.needs_verification === 'true' };
}

export async function verifyAccount(email: string, token: string): Promise<void> {
  await apiRequest<void>('/auth/verify', 'POST', { email, token });
}

export async function logout(): Promise<void> {
  await apiRequest<void>('/auth/logout', 'POST');
  currentUser.set(null);
  if (typeof window !== 'undefined') {
    localStorage.removeItem('user_session');
  }
}
