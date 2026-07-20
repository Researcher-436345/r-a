import { Languages, MessageSquareText, NotebookPen, X } from 'lucide-react';
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from 'react';

import type { ChatContextAttachment } from '../chat-context';
import { buildPassageChipLabel } from '../chat-context';
import { normalizeSelectedQuote } from '../normalize-quote';

export interface ReaderSelection {
  page: number;
  text: string;
  rect: { x: number; y: number; w: number; h: number };
  anchor: { x: number; y: number };
}

type PopupMode = 'choose' | 'note' | 'translate';

interface ReaderSelectionPopupProps {
  selection: ReaderSelection | null;
  isSaving: boolean;
  isTranslating?: boolean;
  translation?: string | null;
  onClose: () => void;
  onSave: (payload: { note: string; quote: string }) => void;
  onAskAssistant: (attachment: ChatContextAttachment) => void;
  onTranslate: (text: string) => void;
}

function buildAttachment(selection: ReaderSelection): ChatContextAttachment {
  const raw = selection.text.replace(/\s+/g, ' ').trim();
  const preview = raw.length > 120 ? `${raw.slice(0, 117).trim()}…` : raw;

  return {
    id: `${selection.page}:${Date.now()}:${Math.random().toString(36).slice(2, 7)}`,
    page: selection.page,
    rect: selection.rect,
    locationLabel: buildPassageChipLabel(selection.page, raw),
    preview,
    text: raw,
  };
}

function computeAnchor(selection: ReaderSelection) {
  const pageElement = document.getElementById(`reader-pdf-page-${selection.page}`);
  if (!pageElement) {
    return selection.anchor;
  }

  const pageRect = pageElement.getBoundingClientRect();
  return {
    x: pageRect.left + selection.rect.x + selection.rect.w / 2,
    y: pageRect.top + selection.rect.y + selection.rect.h,
  };
}

function computePopupPosition(
  anchor: { x: number; y: number },
  selection: ReaderSelection,
  width: number,
  height: number,
) {
  const pad = 8;
  const left = Math.min(Math.max(pad, anchor.x - width / 2), window.innerWidth - width - pad);
  const preferredTop = anchor.y + pad;
  const top =
    preferredTop + height > window.innerHeight - pad
      ? Math.max(pad, anchor.y - selection.rect.h - height - pad)
      : preferredTop;

  return { left, top };
}

export function ReaderSelectionPopup({
  selection,
  isSaving,
  isTranslating = false,
  translation = null,
  onClose,
  onSave,
  onAskAssistant,
  onTranslate,
}: ReaderSelectionPopupProps) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const noteRef = useRef<HTMLTextAreaElement | null>(null);
  const [mode, setMode] = useState<PopupMode>('choose');
  const [note, setNote] = useState('');
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);
  const [isOccluded, setIsOccluded] = useState(false);

  const updatePosition = useCallback(() => {
    if (!selection || !rootRef.current) {
      return;
    }

    const anchor = computeAnchor(selection);
    const node = rootRef.current;
    const nextPosition = computePopupPosition(anchor, selection, node.offsetWidth, node.offsetHeight);
    setPosition(nextPosition);

    const toolbar = document.querySelector('.reader-toolbar');
    const toolbarBottom = toolbar?.getBoundingClientRect().bottom ?? 0;
    const popupBottom = nextPosition.top + node.offsetHeight;
    setIsOccluded(popupBottom <= toolbarBottom + 2 || nextPosition.top < toolbarBottom);
  }, [selection]);

  useEffect(() => {
    if (!selection) {
      setMode('choose');
      setNote('');
      setPosition(null);
      return;
    }
    setMode('choose');
    setNote('');
  }, [selection]);

  useEffect(() => {
    if (mode === 'note') {
      noteRef.current?.focus();
    }
  }, [mode]);

  useLayoutEffect(() => {
    updatePosition();
  }, [selection, mode, translation, isTranslating, updatePosition]);

  useEffect(() => {
    if (!selection) {
      return;
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    const onPointerDown = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (rootRef.current?.contains(target)) {
        return;
      }
      onClose();
    };

    const onReposition = () => updatePosition();

    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('mousedown', onPointerDown, true);
    window.addEventListener('scroll', onReposition, true);
    window.addEventListener('resize', onReposition);

    return () => {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('mousedown', onPointerDown, true);
      window.removeEventListener('scroll', onReposition, true);
      window.removeEventListener('resize', onReposition);
    };
  }, [selection, onClose, updatePosition]);

  if (!selection) {
    return null;
  }

  const onSubmitNote = (event: FormEvent) => {
    event.preventDefault();
    const quote = normalizeSelectedQuote(selection.text);
    if (!quote) {
      return;
    }
    onSave({ note: note.trim(), quote });
  };

  const askAssistant = () => {
    onAskAssistant(buildAttachment(selection));
  };

  const startTranslate = () => {
    setMode('translate');
    const raw = selection.text.replace(/\s+/g, ' ').trim();
    if (raw) {
      onTranslate(raw);
    }
  };

  return (
    <div
      ref={rootRef}
      className={
        [
          mode === 'note' || mode === 'translate'
            ? 'reader-selection-popup reader-selection-popup--note'
            : 'reader-selection-popup',
          isOccluded ? 'reader-selection-popup--occluded' : null,
        ]
          .filter(Boolean)
          .join(' ')
      }
      style={position ?? { left: selection.anchor.x, top: selection.anchor.y, visibility: 'hidden' }}
      role="dialog"
      aria-label="Действие с выделением"
      onMouseDown={(event) => {
        event.preventDefault();
      }}
    >
      {mode === 'choose' ? (
        <div className="reader-selection-popup__toolbar">
          <button type="button" className="reader-selection-popup__tool" onClick={askAssistant}>
            <MessageSquareText aria-hidden="true" size={15} strokeWidth={2} />
            В чат
          </button>
          <button type="button" className="reader-selection-popup__tool" onClick={() => setMode('note')}>
            <NotebookPen aria-hidden="true" size={15} strokeWidth={2} />
            Заметка
          </button>
          <button type="button" className="reader-selection-popup__tool" onClick={startTranslate}>
            <Languages aria-hidden="true" size={15} strokeWidth={2} />
            Перевод
          </button>
          <button
            type="button"
            className="reader-selection-popup__tool reader-selection-popup__tool--ghost"
            onClick={onClose}
            aria-label="Закрыть"
          >
            <X aria-hidden="true" size={15} strokeWidth={2} />
          </button>
        </div>
      ) : mode === 'translate' ? (
        <div className="reader-selection-popup__note">
          <div className="reader-selection-popup__translation">
            {isTranslating ? 'Переводим…' : translation || 'Не удалось перевести'}
          </div>
          <div className="reader-selection-popup__actions">
            <button type="button" onClick={() => setMode('choose')} disabled={isTranslating}>
              Назад
            </button>
            <button type="button" className="reader-selection-popup__save" onClick={onClose}>
              Закрыть
            </button>
          </div>
        </div>
      ) : (
        <form className="reader-selection-popup__note" onSubmit={onSubmitNote}>
          <textarea
            ref={noteRef}
            className="reader-selection-popup__input"
            rows={2}
            value={note}
            onChange={(event) => setNote(event.target.value)}
            placeholder="Комментарий к выделению…"
            disabled={isSaving}
            onMouseDown={(event) => event.stopPropagation()}
          />
          <div className="reader-selection-popup__actions">
            <button type="button" onClick={() => setMode('choose')} disabled={isSaving}>
              Назад
            </button>
            <button type="submit" className="reader-selection-popup__save" disabled={isSaving}>
              {isSaving ? '…' : 'Сохранить'}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
