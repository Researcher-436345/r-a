import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface MarkdownMessageProps {
  content: string;
  className?: string;
}

export function MarkdownMessage({ content, className = '' }: MarkdownMessageProps) {
  const rootClassName = ['chat-markdown', className].filter(Boolean).join(' ');

  return (
    <div className={rootClassName}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          a({ node: _node, ...props }) {
            return <a {...props} target="_blank" rel="noreferrer noopener" />;
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
