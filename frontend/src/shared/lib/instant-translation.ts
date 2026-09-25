import { useSyncExternalStore } from 'react';

const key = 'researcher.instantTranslation';
const changed = 'researcher:instant-translation';
let fallback = true;

function read() {
  try { return window.localStorage.getItem(key) !== 'false'; }
  catch { return fallback; }
}

function subscribe(listener: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === key || event.key === null) listener();
  };
  window.addEventListener(changed, listener);
  window.addEventListener('storage', onStorage);
  return () => {
    window.removeEventListener(changed, listener);
    window.removeEventListener('storage', onStorage);
  };
}

export function useInstantTranslation() {
  const enabled = useSyncExternalStore(subscribe, read, () => true);
  const setEnabled = (value: boolean) => {
    fallback = value;
    try { window.localStorage.setItem(key, String(value)); } catch { /* Private storage may be unavailable. */ }
    window.dispatchEvent(new Event(changed));
  };
  return { enabled, setEnabled };
}
