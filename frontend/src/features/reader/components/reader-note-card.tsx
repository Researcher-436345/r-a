import { ChevronDown, ChevronUp, Pencil, Quote, Trash2 } from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import { RichText } from '../../../shared/ui/rich-text';
import type { PaperAnnotation } from '../api';

/** Свёрнутый превью-блок; выше — показываем «ещё». */
const NOTE_COLLAPSE_HEIGHT = 140;

interface ReaderNoteCardProps {
  note: PaperAnnotation;
  locale: 'ru' | 'en';
  isActive: boolean;
  isDeleting: boolean;
  onOpen: (note: PaperAnnotation) => void;
  onSave: (noteId: string, text: string) => Promise<void>;
  onDelete: (noteId: string) => void;
  onPageCite?: (page: number, quote?: string) => void;
}

export function ReaderNoteCard({
  note,
  locale,
  isActive,
  isDeleting,
  onOpen,
  onSave,
  onDelete,
  onPageCite,
}: ReaderNoteCardProps) {
  const editRef = useRef<HTMLTextAreaElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState(note.note);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);

  useEffect(() => {
    if (!isEditing) {
      setDraft(note.note);
    }
  }, [note.note, isEditing]);

  useEffect(() => {
    setExpanded(false);
  }, [note.id, note.note]);

  useEffect(() => {
    if (isEditing) {
      editRef.current?.focus();
    }
  }, [isEditing]);

  useLayoutEffect(() => {
    if (isEditing || !note.note) {
      setOverflows(false);
      return;
    }
    const el = contentRef.current;
    if (!el) {
      return;
    }

    const measure = () => {
      // Measure full height even when visually collapsed.
      const prevMax = el.style.maxHeight;
      const prevOverflow = el.style.overflow;
      el.style.maxHeight = 'none';
      el.style.overflow = 'visible';
      const full = el.scrollHeight;
      el.style.maxHeight = prevMax;
      el.style.overflow = prevOverflow;
      setOverflows(full > NOTE_COLLAPSE_HEIGHT + 12);
    };

    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [note.note, isEditing, expanded]);

  const startEdit = () => {
    setDraft(note.note);
    setError(null);
    setIsEditing(true);
    setExpanded(true);
  };

  const cancelEdit = () => {
    setDraft(note.note);
    setError(null);
    setIsEditing(false);
  };

  const saveEdit = async () => {
    setIsSaving(true);
    setError(null);
    try {
      await onSave(note.id, draft.trim());
      setIsEditing(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось сохранить');
    } finally {
      setIsSaving(false);
    }
  };

  const collapsed = Boolean(note.note) && !isEditing && overflows && !expanded;

  return (
    <article
      className={
        isActive
          ? 'reader-note-card reader-note-card--active'
          : 'reader-note-card reader-note-card--clickable'
      }
      data-note-id={note.id}
      style={{ borderLeftColor: note.color || undefined }}
    >
      <button
        type="button"
        className="reader-note-card__open"
        onClick={() => onOpen(note)}
        title={
          note.source_chat_message_id
            ? locale === 'ru'
              ? 'Перейти к сообщению в чате'
              : 'Jump to chat message'
            : note.rect
              ? locale === 'ru'
                ? 'Перейти к месту в PDF'
                : 'Jump to PDF'
              : locale === 'ru'
                ? 'Заметка из чата'
                : 'Note from chat'
        }
        disabled={isEditing}
      >
        <div className="reader-note-card__quote">
          <Quote aria-hidden="true" size={15} strokeWidth={2} />
          <span>{note.selected_text}</span>
        </div>
      </button>

      <div className="reader-note-card__body">
        {isEditing ? (
          <div className="reader-note-card__edit">
            <textarea
              ref={editRef}
              className="reader-note-card__edit-input"
              rows={6}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Комментарий…"
              disabled={isSaving}
            />
            {error ? <div className="reader-note-card__edit-error">{error}</div> : null}
            <div className="reader-note-card__edit-actions">
              <button type="button" onClick={cancelEdit} disabled={isSaving}>
                Отмена
              </button>
              <button
                type="button"
                className="reader-note-card__edit-save"
                onClick={() => void saveEdit()}
                disabled={isSaving}
              >
                {isSaving ? '…' : 'Сохранить'}
              </button>
            </div>
          </div>
        ) : note.note ? (
          <>
            <div
              ref={contentRef}
              className={[
                'reader-note-card__content',
                collapsed ? 'reader-note-card__content--collapsed' : null,
                expanded && overflows ? 'reader-note-card__content--expanded' : null,
              ]
                .filter(Boolean)
                .join(' ')}
              style={collapsed ? { maxHeight: NOTE_COLLAPSE_HEIGHT } : undefined}
            >
              <RichText className="reader-note-card__rich" onPageCite={onPageCite}>
                {note.note}
              </RichText>
            </div>
            {overflows ? (
              <button
                type="button"
                className="reader-note-card__expand"
                onClick={(event) => {
                  event.stopPropagation();
                  setExpanded((value) => !value);
                }}
              >
                {expanded ? (
                  <>
                    <ChevronUp aria-hidden="true" size={14} strokeWidth={2} />
                    {locale === 'ru' ? 'Свернуть' : 'Collapse'}
                  </>
                ) : (
                  <>
                    <ChevronDown aria-hidden="true" size={14} strokeWidth={2} />
                    {locale === 'ru' ? 'Показать ещё' : 'Show more'}
                  </>
                )}
              </button>
            ) : null}
          </>
        ) : (
          <p className="reader-note-card__empty">Без комментария</p>
        )}
      </div>

      <div className="reader-note-card__meta">
        <span>
          {note.page > 0
            ? locale === 'ru'
              ? `стр. ${note.page}`
              : `p. ${note.page}`
            : note.source_chat_message_id || note.selected_text.startsWith('Чат') || note.selected_text.startsWith('Chat')
              ? locale === 'ru'
                ? 'чат'
                : 'chat'
              : locale === 'ru'
                ? 'заметка'
                : 'note'}
        </span>
        <div className="reader-note-card__meta-actions">
          {!isEditing ? (
            <button
              type="button"
              className="reader-note-card__edit-button"
              title="Редактировать комментарий"
              onClick={startEdit}
            >
              <Pencil aria-hidden="true" size={14} strokeWidth={2} />
            </button>
          ) : null}
          <button
            type="button"
            className="reader-note-card__delete"
            title="Удалить"
            disabled={isDeleting}
            onClick={() => onDelete(note.id)}
          >
            <Trash2 aria-hidden="true" size={14} strokeWidth={2} />
          </button>
        </div>
      </div>
    </article>
  );
}
