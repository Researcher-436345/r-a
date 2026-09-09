const configuredURL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';
const loopbackHosts = new Set(['localhost', '127.0.0.1', '[::1]']);

// Keep local auth cookies same-site when the UI is opened via a loopback alias.
function resolveAPIURL() {
  const url = new URL(configuredURL, window.location.origin);
  if (loopbackHosts.has(url.hostname) && loopbackHosts.has(window.location.hostname)) {
    url.hostname = window.location.hostname;
  }
  return url.toString().replace(/\/$/, '');
}

export const API_URL = resolveAPIURL();
