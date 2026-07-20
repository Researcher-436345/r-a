import { getAccessToken, clearTokens } from '../../features/auth/token-storage';
import { tryRefreshSession } from '../../features/auth/refresh-session';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export class ApiError extends Error {
  status: number;
  detail: string;

  constructor(status: number, detail: string) {
    super(detail);
    this.name = 'ApiError';
    this.status = status;
    this.detail = detail;
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown;
  token?: string | null;
  /** Внутренний флаг: не пытаться refresh повторно */
  _authRetried?: boolean;
};

function redirectToLoginOnExpiredSession() {
  if (typeof window === 'undefined') {
    return;
  }

  const { pathname, search, hash } = window.location;
  const currentPath = `${pathname}${search}${hash}`;
  const isAuthPage = pathname === '/login' || pathname === '/register';

  clearTokens();

  if (isAuthPage) {
    return;
  }

  const params = new URLSearchParams({
    expired: '1',
    next: currentPath,
  });
  window.location.assign(`/login?${params.toString()}`);
}

function shouldAttemptRefresh(path: string, status: number, retried: boolean) {
  if (status !== 401 || retried) {
    return false;
  }
  return !path.startsWith('/auth/');
}

async function parseErrorDetail(response: Response) {
  let detail = `Request failed with status ${response.status}`;
  try {
    const data = (await response.json()) as { detail?: string | Array<{ msg?: string }> };
    if (typeof data.detail === 'string') {
      detail = data.detail;
    } else if (Array.isArray(data.detail) && data.detail[0]?.msg) {
      detail = data.detail[0].msg;
    }
  } catch {
    // ignore JSON parse errors
  }
  return detail;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);

  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json');
  }

  const token = options.token ?? getAccessToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    if (shouldAttemptRefresh(path, response.status, Boolean(options._authRetried))) {
      const refreshed = await tryRefreshSession();
      if (refreshed) {
        return apiRequest<T>(path, {
          ...options,
          token: getAccessToken(),
          _authRetried: true,
        });
      }
      redirectToLoginOnExpiredSession();
    } else if (response.status === 401 && !path.startsWith('/auth/')) {
      redirectToLoginOnExpiredSession();
    }

    throw new ApiError(response.status, await parseErrorDetail(response));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
