import { isAuthenticated } from './token-storage';

/** Only guest-accessible local pages can be a destination when skipping sign-in. */
export function guestReturnHref(path: string | null | undefined): string {
  if (!path || !path.startsWith('/') || path.startsWith('//') || /[\\\s]/.test(path)) return '/';
  const pathname = path.split(/[?#]/)[0];
  return /^(?:\/|\/welcome|\/reader(?:\/[^/]+)?|\/design-system)$/.test(pathname) ? path : '/';
}

export function loginHref(
  next = `${window.location.pathname}${window.location.search}${window.location.hash}`,
  expired = false,
  from = `${window.location.pathname}${window.location.search}${window.location.hash}`,
) {
  const search = new URLSearchParams({ next, from: guestReturnHref(from) });
  if (expired) search.set('expired', '1');
  return `/login?${search}`;
}

/** Call only for an explicit user action; background queries must be disabled for guests. */
export function requireAuthentication(): boolean {
  if (isAuthenticated()) return true;
  window.location.assign(loginHref());
  return false;
}
