import { writable } from 'svelte/store';

export interface AppConfig {
  multiplayer: boolean;
}

export const appConfig = writable<AppConfig | null>(null);

export async function fetchConfig(): Promise<AppConfig> {
  try {
    const res = await fetch('/api/config');
    const data = await res.json();
    appConfig.set(data);
    return data;
  } catch (err) {
    console.error("Failed to fetch app config", err);
    const fallback = { multiplayer: true };
    appConfig.set(fallback);
    return fallback;
  }
}
