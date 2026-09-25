import 'katex/dist/katex.min.css';
import { ReaderMaterialLink } from '../../features/papers/components/reader-material-link';

import { ArrowUpRight } from 'lucide-react';
import type { ReactNode } from 'react';
import type { Components } from 'react-markdown';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';

import { useLocale } from '../i18n/i18n-context';

interface RichTextProps {
  children: string;
  className?: string;
  /** Prefer compact inline look (feed/library abstracts) */
  compact?: boolean;
  openInReader?: boolean;
  /** Click handler for [p.N] / [стр. N] citations in assistant replies */
  onPageCite?: (page: number, quote?: string) => void;
  /** Abstracts come from scraped pages and indexes: their images are not loaded. */
  allowImages?: boolean;
}

/** Soft-normalize common TeX delimiters so KaTeX can render arXiv-style abstracts. */
export function normalizeMathMarkdown(input: string): string {
  let text = input.replace(/\r\n/g, '\n');
  // \( ... \) → $...$ and \[ ... \] → $$...$$
  text = text.replace(/\\\(([\s\S]*?)\\\)/g, (_m, inner: string) => `$${inner}$`);
  text = text.replace(/\\\[([\s\S]*?)\\\]/g, (_m, inner: string) => `$$${inner}$$`);
  return text;
}

function encodeCiteQuote(quote: string): string {
  return encodeURIComponent(quote).replace(/%20/g, '+');
}

function decodeCiteQuote(raw: string | undefined): string | undefined {
  if (!raw) {
    return undefined;
  }
  try {
    const decoded = decodeURIComponent(raw.replace(/\+/g, '%20')).trim();
    return decoded || undefined;
  } catch {
    return undefined;
  }
}

/**
 * Turn [p.12] / [стр. 12] / [p.12 «quote»] / [p.9-10] / [[p:12]] into markdown links.
 * Quote is optional and only used for the chip label / future highlight.
 * Диапазон страниц ведёт на первую: модели охотно пишут [p.9-10].
 * Лимит цитаты щедрый — модель нередко превышает запрошенные 3–12 слов,
 * а нераспознанная цитата разваливается в сырой текст прямо в ответе.
 */
export function linkifyPageCites(input: string): string {
  return input
    .replace(/\[\[p:(\d{1,4})\]\]/gi, (_m, page: string) => `[стр. ${page}](#cite-page-${page})`)
    .replace(
      /\[(?:p\.|стр\.?\s*)(\d{1,4})(?:\s*[-–—]\s*(\d{1,4}))?(?:\s*[«"“]([^»"”]{2,400})[»"”])?\]/gi,
      (_m, page: string, pageEnd?: string, quote?: string) => {
        const label = pageEnd ? `стр. ${page}-${pageEnd}` : `стр. ${page}`;
        const cleaned = quote?.replace(/\s+/g, ' ').replace(/[[\]]/g, '').trim();
        if (cleaned) {
          const short = cleaned.length > 40 ? `${cleaned.slice(0, 37)}…` : cleaned;
          return `[${label} · ${short}](#cite-page-${page}?q=${encodeCiteQuote(cleaned)})`;
        }
        return `[${label}](#cite-page-${page})`;
      },
    );
}

/** Leaving the app is explicit: new tab, ↗ and a screen-reader note (bare URLs in abstracts autolink). */
function ExternalLink({ href, children }: { href: string; children: ReactNode }) {
  const label = useLocale() === 'en' ? 'opens in a new tab' : 'откроется в новой вкладке';
  return (
    <a href={href} target="_blank" rel="noreferrer noopener" title={label}>
      {children}
      <ArrowUpRight className="external-link-icon" aria-hidden="true" size={13} strokeWidth={2} />
      <span className="sr-only"> ({label})</span>
    </a>
  );
}

export function RichText({
  children,
  className,
  compact = false,
  openInReader = false,
  onPageCite,
  allowImages = true,
}: RichTextProps) {
  let source = normalizeMathMarkdown(children || '');
  if (onPageCite) {
    source = linkifyPageCites(source);
  }

  const components: Components = {
    a: ({ href, children: linkChildren }) => {
      const match = onPageCite ? href?.match(/^#cite-page-(\d+)(?:\?q=([^#]*))?$/) : null;
      if (match && onPageCite) {
        const page = Number(match[1]);
        const quote = decodeCiteQuote(match[2]);
        return (
          <button
            type="button"
            className="rich-text__page-cite"
            title={quote}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onPageCite(page, quote);
            }}
          >
            {linkChildren}
          </button>
        );
      }
      if (href && /^https?:\/\//i.test(href)) {
        return openInReader ? <ReaderMaterialLink href={href}>{linkChildren}</ReaderMaterialLink> : <ExternalLink href={href}>{linkChildren}</ExternalLink>;
      }
      return <a href={href}>{linkChildren}</a>;
    },
  };
  if (!allowImages) {
    // Loading them would ping whatever host the text names; the alt text stays.
    components.img = ({ alt }) => (alt ? <span>{alt}</span> : null);
  }

  return (
    <div
      className={['rich-text', compact ? 'rich-text--compact' : null, className]
        .filter(Boolean)
        .join(' ')}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={components}
      >
        {source}
      </ReactMarkdown>
    </div>
  );
}
