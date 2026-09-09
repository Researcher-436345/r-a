// Access token lives only in module memory (dies with the tab).
// The refresh token is an httpOnly cookie managed by the backend —
// nothing session-related is persisted in localStorage anymore.
let accessToken: string | null = null;

export function getAccessToken(): string | null {
  return accessToken;
}

export function setAccessToken(token: string): void {
  accessToken = token;
}

export function clearTokens(): void {
  accessToken = null;
}

export function isAuthenticated(): boolean {
  return accessToken !== null;
}
