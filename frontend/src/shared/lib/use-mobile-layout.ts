import { useEffect, useState, type RefObject } from 'react';

export const MOBILE_LAYOUT_QUERY = '(max-width: 820px)';

export function useMobileLayout() {
  const [isMobile, setIsMobile] = useState(() =>
    typeof window !== 'undefined' && window.matchMedia(MOBILE_LAYOUT_QUERY).matches,
  );

  useEffect(() => {
    const media = window.matchMedia(MOBILE_LAYOUT_QUERY);
    const update = () => setIsMobile(window.matchMedia(MOBILE_LAYOUT_QUERY).matches);
    update();
    media.addEventListener('change', update);
    window.addEventListener('resize', update);
    return () => {
      media.removeEventListener('change', update);
      window.removeEventListener('resize', update);
    };
  }, []);

  return isMobile;
}

// На iOS клавиатура уменьшает visualViewport, но не высоту layout viewport.
export function useMobileViewport(shellRef: RefObject<HTMLElement>) {
  useEffect(() => {
    const shell = shellRef.current;
    const viewport = window.visualViewport;
    if (!shell || !viewport) return;
    const media = window.matchMedia(MOBILE_LAYOUT_QUERY);
    const update = () => {
      if (!window.matchMedia(MOBILE_LAYOUT_QUERY).matches || viewport.scale !== 1) {
        shell.style.removeProperty('--app-height');
        delete shell.dataset.keyboardOpen;
        return;
      }
      const editing = document.activeElement?.matches('input, textarea, [contenteditable="true"]');
      const keyboardOpen = Boolean(editing && window.innerHeight - viewport.height > 120);
      shell.dataset.keyboardOpen = String(keyboardOpen);
      if (keyboardOpen) shell.style.setProperty('--app-height', `${viewport.height}px`);
      else shell.style.removeProperty('--app-height');
    };
    update();
    viewport.addEventListener('resize', update);
    viewport.addEventListener('scroll', update);
    media.addEventListener('change', update);
    window.addEventListener('resize', update);
    document.addEventListener('focusin', update);
    document.addEventListener('focusout', update);
    return () => {
      viewport.removeEventListener('resize', update);
      viewport.removeEventListener('scroll', update);
      media.removeEventListener('change', update);
      window.removeEventListener('resize', update);
      document.removeEventListener('focusin', update);
      document.removeEventListener('focusout', update);
      shell.style.removeProperty('--app-height');
      delete shell.dataset.keyboardOpen;
    };
  }, [shellRef]);
}
