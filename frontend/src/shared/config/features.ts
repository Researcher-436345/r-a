// Build-time flags: omitted values intentionally keep optional screens disabled.
export const features = {
  requireLoginOnEntry: import.meta.env.VITE_REQUIRE_LOGIN_ON_ENTRY === 'true',
  activeSessions: import.meta.env.VITE_ENABLE_ACTIVE_SESSIONS === 'true',
};
