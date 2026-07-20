import {
  ArrowUp,
  CornerDownRight,
  GitCompare,
  Highlighter,
  Layers,
  NotebookPen,
  Paperclip,
  Sparkles,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { chatPaper, deleteAnnotation, updateAnnotation, type PaperAnnotation } from '../api';
import type { ChatContextAttachment } from '../chat-context';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import { ApiError } from '../../../shared/api/client';
import {
  readerPrompts,
  readerSimilar,
  readerStrings,
  type ReaderTab,
} from '../reader-data';
import {
  ChatComposer,
  type ChatComposerHandle,
  type ComposerSegment,
} from './chat-composer';
import { ReaderNoteCard } from './reader-note-card';

const readerTabs = [
  { value: 'assistant', icon: Sparkles },
  { value: 'notes', icon: NotebookPen },
  { value: 'similar', icon: Layers },
] as const;

interface ReaderChatPanelProps {
  paperId?: string;
  annotations: PaperAnnotation[];
  activeNoteId?: string | null;
  /** Новое выделение — добавляется токеном в инпут (не заменяет старые) */
  contextAttachment?: ChatContextAttachment | null;
  focusAssistantToken?: number;
  focusNotesToken?: number;
  onClearContextAttachment?: () => void;
  onNoteSelect?: (note: PaperAnnotation) => void;
  onPassageSelect?: (attachment: ChatContextAttachment) => void;
  onNoteUpdated?: (note: PaperAnnotation) => void;
  onAnnotationsChange?: () => void;
}

interface LocalChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  attachments?: ChatContextAttachment[];
  segments?: ComposerSegment[];
  /** Полный текст для будущей модели */
  modelPayload?: string;
}

export function ReaderChatPanel({
  paperId,
  annotations,
  activeNoteId,
  contextAttachment = null,
  focusAssistantToken = 0,
  focusNotesToken = 0,
  onClearContextAttachment,
  onNoteSelect,
  onPassageSelect,
  onNoteUpdated,
  onAnnotationsChange,
}: ReaderChatPanelProps) {
  const { locale } = useI18n();
  const text = readerStrings[locale];
  const [activeTab, setActiveTab] = useState<ReaderTab>('assistant');
  const [messages, setMessages] = useState<LocalChatMessage[]>([]);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [composerEmpty, setComposerEmpty] = useState(true);
  const [isSending, setIsSending] = useState(false);
  const composerRef = useRef<ChatComposerHandle | null>(null);
  const lastInsertedId = useRef<string | null>(null);

  const tabs = useMemo(
    () =>
      readerTabs.map((tab) => ({
        ...tab,
        label:
          tab.value === 'assistant'
            ? text.tabAssistant
            : tab.value === 'notes'
              ? text.tabNotes
              : text.tabSimilar,
      })),
    [text.tabAssistant, text.tabNotes, text.tabSimilar],
  );

  useEffect(() => {
    if (!focusAssistantToken) {
      return;
    }
    setActiveTab('assistant');
  }, [focusAssistantToken]);

  useEffect(() => {
    if (!focusNotesToken) {
      return;
    }
    setActiveTab('notes');
    if (!activeNoteId) {
      return;
    }
    window.setTimeout(() => {
      const card = document.querySelector(`[data-note-id="${activeNoteId}"]`);
      card?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }, 60);
  }, [focusNotesToken, activeNoteId]);

  useEffect(() => {
    if (!contextAttachment || contextAttachment.id === lastInsertedId.current) {
      return;
    }
    lastInsertedId.current = contextAttachment.id;
    setActiveTab('assistant');
    // Composer всегда смонтирован (скрыт на других вкладках) — вставка не теряется
    composerRef.current?.insertAttachment(contextAttachment);
    onClearContextAttachment?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- insert once per attachment id
  }, [contextAttachment?.id]);

  const handleDelete = async (annotationId: string) => {
    setDeletingId(annotationId);
    setError(null);
    try {
      await deleteAnnotation(annotationId);
      onAnnotationsChange?.();
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Ошибка удаления');
    } finally {
      setDeletingId(null);
    }
  };

  const handleUpdateNote = async (noteId: string, text: string) => {
    const updated = await updateAnnotation(noteId, text);
    onNoteUpdated?.(updated);
  };

  const handleSend = async () => {
    if (isSending) {
      return;
    }
    if (!paperId) {
      return;
    }
    const snapshot = composerRef.current?.getSnapshot();
    if (!snapshot || (composerRef.current?.isEmpty() ?? true)) {
      return;
    }

    const attachments = snapshot.attachments;
    const hasText = Boolean(snapshot.plainText);
    const defaultAsk = locale === 'ru' ? 'Объясни этот фрагмент' : 'Explain this passage';
    const content = hasText ? snapshot.plainText : attachments.length ? defaultAsk : '';
    const segments: ComposerSegment[] =
      hasText || !attachments.length
        ? snapshot.segments
        : [
            ...snapshot.segments,
            { type: 'text', value: ` ${defaultAsk}` },
          ];

    if (!content && !attachments.length) {
      return;
    }

    const userMessage: LocalChatMessage = {
      id: `u-${Date.now()}`,
      role: 'user',
      content,
      attachments: attachments.length ? attachments : undefined,
      segments,
      modelPayload: snapshot.modelText || content,
    };

    const assistantId = `a-${Date.now()}`;
    const assistantMessage: LocalChatMessage = {
      id: assistantId,
      role: 'assistant',
      content: locale === 'ru' ? 'Думаю…' : 'Thinking…',
    };

    setMessages((current) => [...current, userMessage, assistantMessage]);
    composerRef.current?.clear();
    setComposerEmpty(true);

    setIsSending(true);
    setError(null);
    try {
      const requestMessage = content || defaultAsk;
      const requestContext = snapshot.modelText || snapshot.plainText || null;
      const res = await chatPaper(paperId, {
        message: requestMessage,
        context_text: requestContext,
      });

      setMessages((current) =>
        current.map((msg) => (msg.id === assistantId ? { ...msg, content: res.reply } : msg)),
      );
    } catch (err) {
      const detail =
        err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Ошибка запроса';
      setMessages((current) =>
        current.map((msg) =>
          msg.id === assistantId
            ? {
                ...msg,
                content: locale === 'ru' ? `Ошибка: ${detail}` : `Error: ${detail}`,
              }
            : msg,
        ),
      );
    } finally {
      setIsSending(false);
    }
  };

  const showAssistant = activeTab === 'assistant';

  return (
    <aside className="reader-chat-panel" aria-label="Reader assistant">
      <div className="reader-chat-tabs">
        <SegmentedControl
          ariaLabel="Reader panel tabs"
          className="segmented-control--reader-tabs"
          options={tabs}
          value={activeTab}
          onChange={setActiveTab}
        />
      </div>

      {/* Не размонтируем: иначе черновик и ref пропадают на Notes/Similar */}
      <div
        className="reader-assistant-pane"
        hidden={!showAssistant}
        aria-hidden={!showAssistant}
      >
        <div className="reader-assistant">
          {messages.length === 0 ? (
            <>
              <div className="reader-assistant__icon">
                <Sparkles aria-hidden="true" size={24} strokeWidth={2} />
              </div>

              <div className="reader-suggestion-card">
                <div className="reader-suggestion-card__header">
                  <Highlighter aria-hidden="true" size={18} strokeWidth={2} />
                  <span>{text.cardTitle}</span>
                </div>
                <p>{text.cardSub}</p>

                <div className="reader-prompts">
                  {readerPrompts[locale].map((prompt) => (
                    <button
                      className="reader-prompt-button"
                      type="button"
                      key={prompt}
                      onClick={() => {
                        composerRef.current?.setPlainText(prompt);
                        setComposerEmpty(false);
                        composerRef.current?.focus();
                      }}
                    >
                      <CornerDownRight aria-hidden="true" size={15} strokeWidth={2} />
                      <span>{prompt}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="reader-assistant__hint">{text.tryHint}</div>
            </>
          ) : (
            <div className="reader-chat-thread">
              {messages.map((message) => (
                <div
                  key={message.id}
                  className={
                    message.role === 'user'
                      ? 'reader-chat-bubble reader-chat-bubble--user'
                      : 'reader-chat-bubble reader-chat-bubble--assistant'
                  }
                >
                  <p>
                    {message.segments?.length
                      ? message.segments.map((segment, index) =>
                          segment.type === 'chip' ? (
                            <button
                              type="button"
                              key={segment.attachment.id}
                              className="reader-chat-token reader-chat-token--clickable"
                              title={segment.attachment.preview}
                              onClick={() => onPassageSelect?.(segment.attachment)}
                            >
                              {segment.attachment.locationLabel}
                            </button>
                          ) : (
                            <span key={`t-${index}`}>{segment.value}</span>
                          ),
                        )
                      : message.content}
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="reader-chat-input-wrap">
          <div className="reader-chat-input">
            <ChatComposer
              ref={composerRef}
              placeholder={
                locale === 'ru'
                  ? 'Спроси или добавь фрагменты из PDF…'
                  : 'Ask or add passages from the PDF…'
              }
              onChange={() => setComposerEmpty(composerRef.current?.isEmpty() ?? true)}
              onSubmit={handleSend}
              onChipClick={(attachment) => onPassageSelect?.(attachment)}
            />
            <div className="reader-chat-input__footer">
              <button className="reader-attach-button" type="button" title={text.attach}>
                <Paperclip aria-hidden="true" size={16} strokeWidth={2} />
              </button>
              <div className="reader-chat-input__spacer" />
              <span>{text.sendHint}</span>
              <button
                className="reader-send-button"
                type="button"
                aria-label="Send"
                onClick={handleSend}
                  disabled={composerEmpty || isSending}
              >
                <ArrowUp aria-hidden="true" size={17} strokeWidth={2} />
              </button>
            </div>
          </div>
        </div>
      </div>

      {activeTab === 'notes' ? (
        <div className="reader-notes">
          {error ? <div className="auth-error">{error}</div> : null}
          {!paperId ? (
            <div className="library-page__state">Откройте статью из библиотеки</div>
          ) : annotations.length === 0 ? (
            <div className="library-page__state">
              Выделите текст в PDF — появится форма для заметки
            </div>
          ) : (
            annotations.map((note) => (
              <ReaderNoteCard
                key={note.id}
                note={note}
                locale={locale}
                isActive={activeNoteId === note.id}
                isDeleting={deletingId === note.id}
                onOpen={(item) => onNoteSelect?.(item)}
                onSave={handleUpdateNote}
                onDelete={(id) => void handleDelete(id)}
              />
            ))
          )}
        </div>
      ) : null}

      {activeTab === 'similar' ? (
        <div className="reader-similar">
          {readerSimilar[locale].map((paper) => (
            <a className="reader-similar-card" href="#" key={paper.title}>
              <div className="reader-similar-card__title">{paper.title}</div>
              <div className="reader-similar-card__authors">{paper.authors}</div>
              <div className="reader-similar-card__footer">
                <span>
                  <GitCompare aria-hidden="true" size={13} strokeWidth={2} />
                  {paper.sim}
                </span>
                <small>{paper.tag}</small>
              </div>
            </a>
          ))}
        </div>
      ) : null}
    </aside>
  );
}
