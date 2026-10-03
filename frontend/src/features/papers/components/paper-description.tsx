import { useEffect, useId, useRef, useState } from 'react';

import { useLocale } from '../../../shared/i18n/i18n-context';
import { RichText } from '../../../shared/ui/rich-text';

export function PaperDescription({ text }: { text: string }) {
  const locale = useLocale();
  const id = useId();
  const container = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [canExpand, setCanExpand] = useState(false);

  useEffect(() => {
    const element = container.current?.querySelector<HTMLElement>('.paper-card__abstract');
    if (!element) return;
    const measure = () => {
      const lineHeight = parseFloat(getComputedStyle(element).lineHeight);
      setCanExpand(element.scrollHeight > lineHeight * 3 + 1);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [text]);

  return (
    <div
      ref={container}
      className={`paper-card__description${canExpand ? ' paper-card__description--interactive' : ''}`}
      role={canExpand ? 'button' : undefined}
      tabIndex={canExpand ? 0 : undefined}
      aria-expanded={canExpand ? expanded : undefined}
      aria-controls={canExpand ? id : undefined}
      aria-label={canExpand ? (expanded ? (locale === 'ru' ? 'Свернуть обзор' : 'Collapse description') : (locale === 'ru' ? 'Развернуть обзор' : 'Expand description')) : undefined}
      onClick={(event) => {
        if (canExpand && !(event.target as Element).closest('a, button')) setExpanded((value) => !value);
      }}
      onKeyDown={(event) => {
        if (canExpand && event.target === event.currentTarget && (event.key === 'Enter' || event.key === ' ')) {
          event.preventDefault();
          setExpanded((value) => !value);
        }
      }}
    >
      <div id={id}>
        <RichText className={`paper-card__abstract${expanded ? ' paper-card__abstract--expanded' : ''}`} compact allowImages={false}>
          {text}
        </RichText>
      </div>
      {canExpand ? <span className="paper-card__description-toggle" aria-hidden="true">{expanded ? (locale === 'ru' ? 'Свернуть' : 'Collapse') : (locale === 'ru' ? 'Развернуть' : 'Expand')}</span> : null}
    </div>
  );
}
