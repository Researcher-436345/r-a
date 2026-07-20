import { apiRequest } from '../../shared/api/client';
import { getAccessToken, setTokens } from './token-storage';

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
}

export interface UserResponse {
  id: string;
  email: string;
  created_at: string;
}

export async function register(email: string, password: string): Promise<TokenResponse> {
  const tokens = await apiRequest<TokenResponse>('/auth/register', {
    method: 'POST',
    body: { email, password },
  });
  setTokens(tokens.access_token, tokens.refresh_token);
  return tokens;
}

export async function login(email: string, password: string): Promise<TokenResponse> {
  const tokens = await apiRequest<TokenResponse>('/auth/login', {
    method: 'POST',
    body: { email, password },
  });
  setTokens(tokens.access_token, tokens.refresh_token);
  return tokens;
}

export async function fetchMe(): Promise<UserResponse> {
  return apiRequest<UserResponse>('/auth/me', {
    token: getAccessToken(),
  });
}
