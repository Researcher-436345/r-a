import 'katex/dist/katex.min.css';

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';

interface RichTextProps {
  children: string;
  className?: string;
  /** Prefer compact inline look (feed/library abstracts) */
  compact?: boolean;
}

/** Soft-normalize common TeX delimiters so KaTeX can render arXiv-style abstracts. */
export function normalizeMathMarkdown(input: string): string {
  let text = input.replace(/\r\n/g, '\n');
  // \( ... \) → $...$ and \[ ... \] → $$...$$
  text = text.replace(/\\\(([\s\S]*?)\\\)/g, (_m, inner: string) => `$${inner}$`);
  text = text.replace(/\\\[([\s\S]*?)\\\]/g, (_m, inner: string) => `$$${inner}$$`);
  return text;
}

export function RichText({ children, className, compact = false }: RichTextProps) {
  const source = normalizeMathMarkdown(children || '');
  return (
    <div
      className={['rich-text', compact ? 'rich-text--compact' : null, className]
        .filter(Boolean)
        .join(' ')}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
        {source}
      </ReactMarkdown>
    </div>
  );
}
