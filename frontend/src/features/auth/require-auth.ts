import { isAuthenticated } from './token-storage';

/** Keep a destination within this application, including same-origin absolute links. */
export function localReturnHref(path: string | null | undefined, origin = window.location.origin): string {
  if (!path || /[\\\s]/.test(path) || (!path.startsWith('/') && !/^https?:\/\//i.test(path))) return '/';
  try {
    const url = new URL(path, origin);
    return url.origin === origin ? `${url.pathname}${url.search}${url.hash}` : '/';
  } catch {
    return '/';
  }
}

export function isReaderPaperPath(path: string): boolean {
  return /^\/reader\/[^/]+\/?$/.test(path);
}

/** Only guest-accessible local pages can be a destination when skipping sign-in. */
export function guestReturnHref(path: string | null | undefined, origin = window.location.origin): string {
  const local = localReturnHref(path, origin);
  const pathname = local.split(/[?#]/)[0];
  return /^(?:\/|\/welcome|\/reader(?:\/[^/]+)?|\/design-system)$/.test(pathname) ? local : '/';
}

export function loginHref(
  next = `${window.location.pathname}${window.location.search}${window.location.hash}`,
  expired = false,
  from = `${window.location.pathname}${window.location.search}${window.location.hash}`,
) {
  const search = new URLSearchParams({ next: localReturnHref(next), from: guestReturnHref(from) });
  if (expired) search.set('expired', '1');
  return `/login?${search}`;
}

/** Call only for an explicit user action; background queries must be disabled for guests. */
export function requireAuthentication(): boolean {
  if (isAuthenticated()) return true;
  window.location.assign(loginHref());
  return false;
}
