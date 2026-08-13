import type { AnchorHTMLAttributes, ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

export interface MarkdownLinkRenderProps {
  href: string;
  children: ReactNode;
  anchorProps: AnchorHTMLAttributes<HTMLAnchorElement>;
}

interface MarkdownMessageProps {
  content: string;
  className?: string;
  renderLink?: (props: MarkdownLinkRenderProps) => ReactNode;
}

export function MarkdownMessage({
  content,
  className = '',
  renderLink,
}: MarkdownMessageProps) {
  const rootClassName = ['chat-markdown', className].filter(Boolean).join(' ');

  return (
    <div className={rootClassName}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          a({ node: _node, children, href, ...props }) {
            if (href && renderLink) {
              return renderLink({ href, children, anchorProps: props });
            }
            return (
              <a {...props} href={href} target="_blank" rel="noreferrer noopener">
                {children}
              </a>
            );
          },
          table({ node: _node, children, ...props }) {
            return (
              <div className="chat-markdown__table-wrap">
                <table {...props}>{children}</table>
              </div>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
