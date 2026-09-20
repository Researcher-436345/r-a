import { useSyncExternalStore } from 'react';

// Access token lives only in module memory (dies with the tab).
// The refresh token is an httpOnly cookie managed by the backend —
// nothing session-related is persisted in localStorage anymore.
let accessToken: string | null = null;
const listeners = new Set<() => void>();
const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
};

export function useAuthenticated(): boolean {
  return useSyncExternalStore(subscribe, isAuthenticated, () => false);
}

export function getAccessToken(): string | null {
  return accessToken;
}

export function setAccessToken(token: string): void {
  accessToken = token;
  listeners.forEach((listener) => listener());
}

export function clearTokens(): void {
  accessToken = null;
  listeners.forEach((listener) => listener());
}

export function isAuthenticated(): boolean {
  return accessToken !== null;
}
