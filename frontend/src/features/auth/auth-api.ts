import { apiRequest } from '../../shared/api/client';
import { setAccessToken, clearTokens } from './token-storage';

export interface TokenResponse {
  access_token: string;
  token_type: string;
}

export interface UserResponse {
  id: string;
  email: string;
  created_at: string;
  email_verified: boolean;
}

export interface SessionInfo {
  id: string;
  user_agent: string;
  ip: string;
  created_at: string;
  last_used_at: string;
  current: boolean;
}

/** Регистрация: письмо со ссылкой для подтверждения, входа сразу нет. */
export async function register(email: string, password: string): Promise<void> {
  await apiRequest('/auth/register', {
    method: 'POST',
    body: { email, password },
  });
}

export async function login(email: string, password: string): Promise<TokenResponse> {
  const tokens = await apiRequest<TokenResponse>('/auth/login', {
    method: 'POST',
    body: { email, password },
  });
  setAccessToken(tokens.access_token);
  return tokens;
}

/** Подтверждение email одноразовым токеном → авто-вход. */
export async function verifyEmail(token: string): Promise<TokenResponse> {
  const tokens = await apiRequest<TokenResponse>('/auth/verify-email', {
    method: 'POST',
    body: { token },
  });
  setAccessToken(tokens.access_token);
  return tokens;
}

/** Отзыв серверной сессии + очистка cookie и памяти. */
export async function logout(): Promise<void> {
  try {
    await apiRequest('/auth/logout', { method: 'POST' });
  } finally {
    clearTokens();
  }
}

export async function fetchMe(): Promise<UserResponse> {
  return apiRequest<UserResponse>('/auth/me');
}

export async function forgotPassword(email: string): Promise<void> {
  await apiRequest('/auth/forgot-password', {
    method: 'POST',
    body: { email },
  });
}

export async function resetPassword(token: string, newPassword: string): Promise<void> {
  await apiRequest('/auth/reset-password', {
    method: 'POST',
    body: { token, new_password: newPassword },
  });
}

export async function resendVerification(email: string): Promise<void> {
  await apiRequest('/auth/resend-verification', {
    method: 'POST',
    body: { email },
  });
}

export async function fetchSessions(): Promise<SessionInfo[]> {
  const data = await apiRequest<{ sessions: SessionInfo[] }>('/auth/sessions');
  return data.sessions ?? [];
}

export async function revokeSession(id: string): Promise<void> {
  await apiRequest(`/auth/sessions/${id}`, { method: 'DELETE' });
}

export async function revokeOtherSessions(): Promise<void> {
  await apiRequest('/auth/sessions', { method: 'DELETE' });
}
