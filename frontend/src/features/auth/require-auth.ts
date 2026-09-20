import { isAuthenticated } from './token-storage';

export function loginHref(next = `${window.location.pathname}${window.location.search}${window.location.hash}`, expired = false) {
  const search = new URLSearchParams({ next });
  if (expired) search.set('expired', '1');
  return `/login?${search}`;
}

/** Call only for an explicit user action; background queries must be disabled for guests. */
export function requireAuthentication(): boolean {
  if (isAuthenticated()) return true;
  window.location.assign(loginHref());
  return false;
}
