import { useEffect, useRef } from 'react';
import { trackResultViewed, type ResearchFeature } from './analytics';

// A completed request becomes product usage only once its result is actually visible.
export function useResultAnalytics<T extends HTMLElement = HTMLDivElement>(feature: ResearchFeature, key: string | undefined, paperId?: string, resultId?: string) {
  const ref = useRef<T>(null);
  useEffect(() => {
    const element = ref.current;
    if (!element || !key) return;
    let done = false;
    let observer: IntersectionObserver | undefined;
    const report = () => {
      if (done || document.visibilityState !== 'visible') return;
      const box = element.getBoundingClientRect();
      if (box.width === 0 || box.height === 0 || box.bottom <= 0 || box.top >= innerHeight) return;
      done = true; trackResultViewed(feature, key, paperId, resultId); observer?.disconnect();
    };
    observer = typeof IntersectionObserver === 'undefined' ? undefined : new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) report();
    });
    observer?.observe(element);
    const frame = requestAnimationFrame(report);
    document.addEventListener('visibilitychange', report);
    window.addEventListener('scroll', report, { capture: true, passive: true });
    window.addEventListener('resize', report);
    return () => { observer?.disconnect(); cancelAnimationFrame(frame); document.removeEventListener('visibilitychange', report); window.removeEventListener('scroll', report, true); window.removeEventListener('resize', report); };
  }, [feature, key, paperId, resultId]);
  return ref;
}
