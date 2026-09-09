import { Fragment } from 'react';

/** Compact metadata with the v3 vertical micro-dividers. */
export function MetadataLine({ items, className = '' }: { items: readonly string[]; className?: string }) {
  return (
    <div className={`metadata-line ${className}`.trim()}>
      {items.filter(Boolean).map((item, index) => (
        <Fragment key={`${index}-${item}`}>
          {index > 0 && <span className="meta-divider" aria-hidden="true" />}
          <span>{item}</span>
        </Fragment>
      ))}
    </div>
  );
}
