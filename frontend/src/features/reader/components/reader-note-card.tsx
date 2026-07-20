import { Pencil, Quote, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import type { PaperAnnotation } from '../api';

interface ReaderNoteCardProps {
  note: PaperAnnotation;
  locale: 'ru' | 'en';
  isActive: boolean;
  isDeleting: boolean;
  onOpen: (note: PaperAnnotation) => void;
  onSave: (noteId: string, text: string) => Promise<void>;
  onDelete: (noteId: string) => void;
}

export function ReaderNoteCard({
  note,
  locale,
  isActive,
  isDeleting,
  onOpen,
  onSave,
  onDelete,
}: ReaderNoteCardProps) {
  const editRef = useRef<HTMLTextAreaElement | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState(note.note);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isEditing) {
      setDraft(note.note);
    }
  }, [note.note, isEditing]);

  useEffect(() => {
    if (isEditing) {
      editRef.current?.focus();
    }
  }, [isEditing]);

  const startEdit = () => {
    setDraft(note.note);
    setError(null);
    setIsEditing(true);
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

  return (
    <article
      className={
        isActive
          ? 'reader-note-card reader-note-card--active'
          : 'reader-note-card reader-note-card--clickable'
      }
      data-note-id={note.id}
    >
      <button
        type="button"
        className="reader-note-card__open"
        onClick={() => onOpen(note)}
        title="Перейти к месту в PDF"
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
              rows={3}
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
          <p>{note.note}</p>
        ) : (
          <p className="reader-note-card__empty">Без комментария</p>
        )}
      </div>

      <div className="reader-note-card__meta">
        <span>{locale === 'ru' ? `стр. ${note.page}` : `p. ${note.page}`}</span>
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
