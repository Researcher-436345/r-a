// Base URL of the assistant backend service.
// Override at build/dev time with VITE_API_URL (see .env).
const env = (import.meta as unknown as { env?: Record<string, string | undefined> }).env;

export const API_URL = env?.VITE_API_URL ?? 'http://localhost:8080';
