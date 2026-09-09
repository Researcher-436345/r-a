import { setAccessToken } from './token-storage';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

let refreshInFlight: Promise<boolean> | null = null;

/**
 * Silent refresh via the httpOnly refresh cookie (POST /auth/refresh).
 * One request dedupes all parallel 401s and the app-start probe.
 */
export async function tryRefreshSession(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      try {
        const response = await fetch(`${API_URL}/auth/refresh`, {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        });
        if (!response.ok) {
          return false;
        }
        const data = (await response.json()) as { access_token: string };
        setAccessToken(data.access_token);
        return true;
      } catch {
        return false;
      }
    })().finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}
