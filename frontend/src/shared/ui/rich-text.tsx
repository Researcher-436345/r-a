import 'katex/dist/katex.min.css';

import type { Components } from 'react-markdown';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';

interface RichTextProps {
  children: string;
  className?: string;
  /** Prefer compact inline look (feed/library abstracts) */
  compact?: boolean;
  /** Click handler for [p.N] / [стр. N] citations in assistant replies */
  onPageCite?: (page: number, quote?: string) => void;
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
 * Turn [p.12] / [стр. 12] / [p.12 «quote»] / [[p:12]] into markdown links.
 * Quote is optional and only used for the chip label / future highlight.
 */
export function linkifyPageCites(input: string): string {
  return input
    .replace(/\[\[p:(\d{1,4})\]\]/gi, (_m, page: string) => `[стр. ${page}](#cite-page-${page})`)
    .replace(
      /\[(?:p\.|стр\.?\s*)(\d{1,4})(?:\s*[«"“]([^»"”]{2,120})[»"”])?\]/gi,
      (_m, page: string, quote?: string) => {
        const cleaned = quote?.replace(/\s+/g, ' ').trim();
        if (cleaned) {
          const short = cleaned.length > 40 ? `${cleaned.slice(0, 37)}…` : cleaned;
          return `[стр. ${page} · ${short}](#cite-page-${page}?q=${encodeCiteQuote(cleaned)})`;
        }
        return `[стр. ${page}](#cite-page-${page})`;
      },
    );
}

export function RichText({ children, className, compact = false, onPageCite }: RichTextProps) {
  let source = normalizeMathMarkdown(children || '');
  if (onPageCite) {
    source = linkifyPageCites(source);
  }

  const components: Components | undefined = onPageCite
    ? {
        a: ({ href, children: linkChildren }) => {
          const match = href?.match(/^#cite-page-(\d+)(?:\?q=([^#]*))?$/);
          if (match) {
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
          return (
            <a href={href} target="_blank" rel="noreferrer">
              {linkChildren}
            </a>
          );
        },
      }
    : undefined;

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
