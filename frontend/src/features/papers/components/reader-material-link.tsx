import type { CSSProperties, ReactNode } from 'react';

import { cleanPaperTitle, textFromChildren } from '../link-kind';

interface ReaderMaterialLinkProps {
  href: string;
  children?: ReactNode;
  title?: string;
  className?: string;
  style?: CSSProperties;
}

export function ReaderMaterialLink({
  href,
  children,
  title,
  className,
  style,
}: ReaderMaterialLinkProps) {
  const hint = cleanPaperTitle(title || textFromChildren(children));
  const target = `/open?${new URLSearchParams({ url: href, title: hint })}`;

  return (
    <a
      href={target}
      target="_blank"
      rel="noopener noreferrer"
      className={className}
      style={style}
      aria-label={title || undefined}
      onClick={(event) => event.stopPropagation()}
    >
      {children}
    </a>
  );
}
