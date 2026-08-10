import { Languages, MessageSquareText, NotebookPen, Sparkles, X } from 'lucide-react';
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from 'react';

import type { ChatContextAttachment } from '../chat-context';
import { buildPassageChipLabel } from '../chat-context';
import {
  HIGHLIGHT_COLORS,
  toPagePixelRect,
} from '../highlight-colors';
import { normalizeSelectedQuote } from '../normalize-quote';
import { TRANSLATION_MAX_CHARS } from '../api';

export interface ReaderSelection {
  page: number;
  text: string;
  /** Доли страницы (0–1) */
  rect: { x: number; y: number; w: number; h: number };
  anchor: { x: number; y: number };
}

type PopupMode = 'choose' | 'note' | 'translate' | 'explain';

interface ReaderSelectionPopupProps {
  selection: ReaderSelection | null;
  isSaving: boolean;
  isTranslating?: boolean;
  translation?: string | null;
  isExplaining?: boolean;
  explanation?: string | null;
  highlightColor: string;
  onHighlightColorChange: (color: string) => void;
  onClose: () => void;
  onSave: (payload: { note: string; quote: string; color: string }) => void;
  onAskAssistant: (attachment: ChatContextAttachment) => void;
  onTranslate: (text: string) => void;
  onExplain: (text: string) => void;
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

function resolvePageMetrics(selection: ReaderSelection) {
  const pageElement = document.getElementById(`reader-pdf-page-${selection.page}`);
  if (!pageElement) {
    return null;
  }

  const pageRect = pageElement.getBoundingClientRect();
  const pixelRect = toPagePixelRect(selection.rect, pageRect.width, pageRect.height);

  return {
    pageRect,
    pixelRect,
    anchor: {
      x: pageRect.left + pixelRect.x + pixelRect.w / 2,
      y: pageRect.top + pixelRect.y + pixelRect.h,
    },
  };
}

function computePopupPosition(
  anchor: { x: number; y: number },
  selectionHeight: number,
  width: number,
  height: number,
) {
  const pad = 8;
  const left = Math.min(Math.max(pad, anchor.x - width / 2), window.innerWidth - width - pad);
  const preferredTop = anchor.y + pad;
  const top =
    preferredTop + height > window.innerHeight - pad
      ? Math.max(pad, anchor.y - selectionHeight - height - pad)
      : preferredTop;

  return { left, top };
}

function shouldIgnoreOutsidePointer(target: EventTarget | null) {
  if (!(target instanceof Element)) {
    return false;
  }

  // Ресайз сплита: mousedown на ручке не должен закрывать попап
  return Boolean(target.closest('.reader-split-handle') || target.closest('.reader-zoom'));
}

export function ReaderSelectionPopup({
  selection,
  isSaving,
  isTranslating = false,
  translation = null,
  isExplaining = false,
  explanation = null,
  highlightColor,
  onHighlightColorChange,
  onClose,
  onSave,
  onAskAssistant,
  onTranslate,
  onExplain,
}: ReaderSelectionPopupProps) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const noteRef = useRef<HTMLTextAreaElement | null>(null);
  const [mode, setMode] = useState<PopupMode>('choose');
  const [note, setNote] = useState('');
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);
  const [isOccluded, setIsOccluded] = useState(false);
  const selectionKeyRef = useRef<string | null>(null);

  const updatePosition = useCallback(() => {
    if (!selection || !rootRef.current) {
      return;
    }

    const metrics = resolvePageMetrics(selection);
    if (!metrics) {
      return;
    }

    const node = rootRef.current;
    const nextPosition = computePopupPosition(
      metrics.anchor,
      metrics.pixelRect.h,
      node.offsetWidth,
      node.offsetHeight,
    );
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
      selectionKeyRef.current = null;
      return;
    }

    const key = `${selection.page}:${selection.text}`;
    if (selectionKeyRef.current !== key) {
      selectionKeyRef.current = key;
      setMode('choose');
      setNote('');
    }
  }, [selection]);

  useEffect(() => {
    if (mode === 'note') {
      noteRef.current?.focus();
    }
  }, [mode]);

  useLayoutEffect(() => {
    updatePosition();
  }, [selection, mode, translation, isTranslating, explanation, isExplaining, highlightColor, updatePosition]);

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
      if (shouldIgnoreOutsidePointer(target)) {
        return;
      }
      onClose();
    };

    const onReposition = () => updatePosition();

    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('mousedown', onPointerDown, true);
    window.addEventListener('scroll', onReposition, true);
    window.addEventListener('resize', onReposition);

    const pageElement = document.getElementById(`reader-pdf-page-${selection.page}`);
    const resizeObserver =
      pageElement && typeof ResizeObserver !== 'undefined'
        ? new ResizeObserver(() => updatePosition())
        : null;
    if (pageElement && resizeObserver) {
      resizeObserver.observe(pageElement);
    }

    const frameWrap = document.querySelector('.reader-pdf-frame-wrap');
    if (frameWrap && resizeObserver) {
      resizeObserver.observe(frameWrap);
    }

    return () => {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('mousedown', onPointerDown, true);
      window.removeEventListener('scroll', onReposition, true);
      window.removeEventListener('resize', onReposition);
      resizeObserver?.disconnect();
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
    onSave({ note: note.trim(), quote, color: highlightColor });
  };

  const askAssistant = () => {
    onAskAssistant(buildAttachment(selection));
  };

  const startExplain = () => {
    setMode('explain');
    const raw = selection.text.replace(/\s+/g, ' ').trim();
    if (raw) {
      onExplain(raw);
    }
  };

  const startTranslate = () => {
    setMode('translate');
    const raw = selection.text.replace(/\s+/g, ' ').trim();
    if (raw && Array.from(raw).length <= TRANSLATION_MAX_CHARS) {
      onTranslate(raw);
    }
  };

  const selectedCharCount = Array.from(selection.text.replace(/\s+/g, ' ').trim()).length;
  const selectionTooLong = selectedCharCount > TRANSLATION_MAX_CHARS;

  const colorPicker = (
    <div className="reader-selection-popup__colors" role="group" aria-label="Цвет выделения">
      {HIGHLIGHT_COLORS.map((swatch) => {
        const isActive = highlightColor === swatch.hex;
        return (
          <button
            key={swatch.id}
            type="button"
            className={
              isActive
                ? 'reader-selection-popup__color reader-selection-popup__color--active'
                : 'reader-selection-popup__color'
            }
            style={{ background: swatch.hex }}
            title={swatch.label}
            aria-label={swatch.label}
            aria-pressed={isActive}
            onClick={() => onHighlightColorChange(swatch.hex)}
          />
        );
      })}
    </div>
  );

  return (
    <div
      ref={rootRef}
      className={
        [
          mode === 'note' || mode === 'translate' || mode === 'explain'
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
        <div className="reader-selection-popup__choose">
          {colorPicker}
          <div className="reader-selection-popup__toolbar">
            <button type="button" className="reader-selection-popup__tool" onClick={askAssistant}>
              <MessageSquareText aria-hidden="true" size={15} strokeWidth={2} />
              В чат
            </button>
            <button type="button" className="reader-selection-popup__tool" onClick={startExplain}>
              <Sparkles aria-hidden="true" size={15} strokeWidth={2} />
              Спросить AI
            </button>
            <button type="button" className="reader-selection-popup__tool" onClick={() => setMode('note')}>
              <NotebookPen aria-hidden="true" size={15} strokeWidth={2} />
              Заметка
            </button>
            <button
              type="button"
              className="reader-selection-popup__tool"
              onClick={startTranslate}
              title={selectionTooLong ? `Максимум ${TRANSLATION_MAX_CHARS} символов` : undefined}
            >
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
        </div>
      ) : mode === 'explain' ? (
        <div className="reader-selection-popup__note">
          <div className="reader-selection-popup__translation">
            {isExplaining ? 'Думаю…' : explanation || 'Не удалось объяснить'}
          </div>
          <div className="reader-selection-popup__actions">
            <button type="button" onClick={() => setMode('choose')} disabled={isExplaining}>
              Назад
            </button>
            <button type="button" className="reader-selection-popup__save" onClick={onClose}>
              Закрыть
            </button>
          </div>
        </div>
      ) : mode === 'translate' ? (
        <div className="reader-selection-popup__note">
          <div className="reader-selection-popup__translation">
            {selectionTooLong
              ? `Выделено ${selectedCharCount} символов. Для перевода выберите не более ${TRANSLATION_MAX_CHARS}.`
              : isTranslating
                ? 'Переводим…'
                : translation || 'Не удалось перевести'}
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
          {colorPicker}
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
