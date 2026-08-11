import { MessageSquareText, NotebookPen, X } from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';

import { buildReplyAttachment, type ChatContextAttachment } from '../chat-context';

export interface AssistantReplySelectionPayload {
  text: string;
  messageId: string | null;
}

interface AssistantReplySelectionBarProps {
  containerRef: RefObject<HTMLElement | null>;
  locale: 'ru' | 'en';
  onAsk: (attachment: ChatContextAttachment) => void;
  onSaveNote: (payload: AssistantReplySelectionPayload) => void;
  isSavingNote?: boolean;
}

interface BarState {
  text: string;
  messageId: string | null;
  left: number;
  top: number;
}

function selectionInside(container: HTMLElement): {
  text: string;
  rect: DOMRect;
  messageId: string | null;
} | null {
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) {
    return null;
  }
  const range = selection.getRangeAt(0);
  const common = range.commonAncestorContainer;
  const node = common.nodeType === Node.ELEMENT_NODE ? (common as Element) : common.parentElement;
  if (!node || !container.contains(node)) {
    return null;
  }
  // Only assistant bubbles — not user messages / composer
  if (!node.closest('.reader-chat-bubble--assistant')) {
    return null;
  }
  const text = selection.toString().replace(/\s+/g, ' ').trim();
  if (text.length < 2) {
    return null;
  }
  const messageId =
    node.closest('[data-chat-message-id]')?.getAttribute('data-chat-message-id') ?? null;
  return { text, rect: range.getBoundingClientRect(), messageId };
}

export function AssistantReplySelectionBar({
  containerRef,
  locale,
  onAsk,
  onSaveNote,
  isSavingNote = false,
}: AssistantReplySelectionBarProps) {
  const [bar, setBar] = useState<BarState | null>(null);
  const rootRef = useRef<HTMLDivElement | null>(null);

  const refresh = () => {
    const container = containerRef.current;
    if (!container) {
      setBar(null);
      return;
    }
    const hit = selectionInside(container);
    if (!hit) {
      setBar(null);
      return;
    }
    const width = rootRef.current?.offsetWidth ?? 280;
    const height = rootRef.current?.offsetHeight ?? 36;
    const pad = 8;
    const left = Math.min(
      Math.max(pad, hit.rect.left + hit.rect.width / 2 - width / 2),
      window.innerWidth - width - pad,
    );
    let top = hit.rect.top - height - 8;
    if (top < pad) {
      top = hit.rect.bottom + 8;
    }
    setBar({ text: hit.text, messageId: hit.messageId, left, top });
  };

  useEffect(() => {
    const onSel = () => {
      window.requestAnimationFrame(refresh);
    };
    document.addEventListener('selectionchange', onSel);
    document.addEventListener('mouseup', onSel);
    document.addEventListener('keyup', onSel);
    const container = containerRef.current;
    container?.addEventListener('scroll', () => setBar(null), { passive: true });
    return () => {
      document.removeEventListener('selectionchange', onSel);
      document.removeEventListener('mouseup', onSel);
      document.removeEventListener('keyup', onSel);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- bind once per container
  }, [containerRef]);

  useLayoutEffect(() => {
    if (bar) {
      refresh();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bar?.text]);

  if (!bar) {
    return null;
  }

  const askLabel = locale === 'ru' ? 'Уточнить' : 'Ask';
  const noteLabel = locale === 'ru' ? 'В заметки' : 'To notes';
  const closeLabel = locale === 'ru' ? 'Закрыть' : 'Close';

  return (
    <div
      ref={rootRef}
      className="assistant-reply-selection"
      style={{ left: bar.left, top: bar.top }}
      role="toolbar"
      aria-label={locale === 'ru' ? 'Действия с выделением' : 'Selection actions'}
    >
      <button
        type="button"
        className="assistant-reply-selection__btn"
        onMouseDown={(event) => {
          event.preventDefault();
        }}
        onClick={() => {
          const attachment = buildReplyAttachment(bar.text);
          onAsk(attachment);
          window.getSelection()?.removeAllRanges();
          setBar(null);
        }}
      >
        <MessageSquareText aria-hidden="true" size={14} strokeWidth={2} />
        {askLabel}
      </button>
      <button
        type="button"
        className="assistant-reply-selection__btn"
        disabled={isSavingNote}
        onMouseDown={(event) => {
          event.preventDefault();
        }}
        onClick={() => {
          onSaveNote({ text: bar.text, messageId: bar.messageId });
          window.getSelection()?.removeAllRanges();
          setBar(null);
        }}
      >
        <NotebookPen aria-hidden="true" size={14} strokeWidth={2} />
        {isSavingNote ? '…' : noteLabel}
      </button>
      <button
        type="button"
        className="assistant-reply-selection__btn assistant-reply-selection__btn--ghost"
        aria-label={closeLabel}
        onMouseDown={(event) => event.preventDefault()}
        onClick={() => {
          window.getSelection()?.removeAllRanges();
          setBar(null);
        }}
      >
        <X aria-hidden="true" size={14} strokeWidth={2} />
      </button>
    </div>
  );
}
