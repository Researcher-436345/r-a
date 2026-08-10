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

import {
  chatPaper,
  deleteAnnotation,
  fetchAssistantModels,
  fetchChatContext,
  fetchChatMessages,
  updateAnnotation,
  type ChatContextUsage,
  type LLMModelOption,
  type PaperAnnotation,
} from '../api';
import type { ChatContextAttachment } from '../chat-context';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import { RichText } from '../../../shared/ui/rich-text';
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
import { AssistantReplySelectionBar } from './assistant-reply-selection-bar';
import { ReaderNoteCard } from './reader-note-card';

const MODEL_STORAGE_KEY = 'researcher.chat.model';

function formatTokens(n: number): string {
  if (n >= 10000) {
    return `${Math.round(n / 1000)}k`;
  }
  if (n >= 1000) {
    return `${(n / 1000).toFixed(1)}k`;
  }
  return String(n);
}

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
  const [historyLoading, setHistoryLoading] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [composerEmpty, setComposerEmpty] = useState(true);
  const [isSending, setIsSending] = useState(false);
  const [models, setModels] = useState<LLMModelOption[]>([]);
  const [selectedModel, setSelectedModel] = useState('');
  const [contextUsage, setContextUsage] = useState<ChatContextUsage | null>(null);
  const composerRef = useRef<ChatComposerHandle | null>(null);
  const threadRef = useRef<HTMLDivElement | null>(null);
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

  const contextTone =
    !contextUsage
      ? 'idle'
      : contextUsage.percent >= 90
        ? 'critical'
        : contextUsage.percent >= 70
          ? 'warn'
          : 'ok';

  useEffect(() => {
    let cancelled = false;
    void fetchAssistantModels()
      .then((res) => {
        if (cancelled) {
          return;
        }
        const items = res.items?.length ? res.items : [{ id: res.default, label: res.default }];
        setModels(items);
        const stored = localStorage.getItem(MODEL_STORAGE_KEY) || '';
        const pick =
          items.find((m) => m.id === stored)?.id ||
          items.find((m) => m.id === res.default)?.id ||
          items[0]?.id ||
          '';
        setSelectedModel(pick);
      })
      .catch(() => {
        /* optional */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (selectedModel) {
      localStorage.setItem(MODEL_STORAGE_KEY, selectedModel);
    }
  }, [selectedModel]);

  useEffect(() => {
    if (!paperId) {
      setMessages([]);
      setContextUsage(null);
      return;
    }

    let cancelled = false;
    setHistoryLoading(true);
    setError(null);
    setMessages([]);

    void fetchChatMessages(paperId)
      .then((items) => {
        if (cancelled) {
          return;
        }
        setMessages(
          items.map((item) => ({
            id: item.id,
            role: item.role,
            content: item.content,
            modelPayload: item.context_text ? `${item.content}\n\n${item.context_text}` : item.content,
          })),
        );
      })
      .catch((err) => {
        if (cancelled) {
          return;
        }
        setError(
          err instanceof ApiError
            ? err.detail
            : err instanceof Error
              ? err.message
              : locale === 'ru'
                ? 'Не удалось загрузить историю чата'
                : 'Could not load chat history',
        );
      })
      .finally(() => {
        if (!cancelled) {
          setHistoryLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [paperId, locale]);

  useEffect(() => {
    if (!paperId || !selectedModel) {
      return;
    }
    let cancelled = false;
    void fetchChatContext(paperId, selectedModel)
      .then((usage) => {
        if (!cancelled) {
          setContextUsage(usage);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setContextUsage(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [paperId, selectedModel, messages.length]);

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

    const tempUserId = `u-${Date.now()}`;
    const tempAssistantId = `a-${Date.now()}`;
    const userMessage: LocalChatMessage = {
      id: tempUserId,
      role: 'user',
      content,
      attachments: attachments.length ? attachments : undefined,
      segments,
      modelPayload: snapshot.modelText || content,
    };

    const assistantMessage: LocalChatMessage = {
      id: tempAssistantId,
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
        model: selectedModel || undefined,
      });

      if (res.context_usage) {
        setContextUsage(res.context_usage);
      }

      setMessages((current) =>
        current.map((msg) => {
          if (msg.id === tempUserId) {
            return {
              ...msg,
              id: res.user_message_id || res.user_message?.id || msg.id,
            };
          }
          if (msg.id === tempAssistantId) {
            return {
              ...msg,
              id: res.message_id || res.assistant_message?.id || msg.id,
              content: res.reply,
            };
          }
          return msg;
        }),
      );
    } catch (err) {
      const detail =
        err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Ошибка запроса';
      setMessages((current) =>
        current.map((msg) =>
          msg.id === tempAssistantId
            ? {
                ...msg,
                content: locale === 'ru' ? `Ошибка: ${detail}` : `Error: ${detail}`,
              }
            : msg,
        ),
      );
      setError(detail);
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
          {historyLoading ? (
            <div className="library-page__state">
              {locale === 'ru' ? 'Загружаем историю…' : 'Loading history…'}
            </div>
          ) : messages.length === 0 ? (
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
            <div className="reader-chat-thread" ref={threadRef}>
              {messages.map((message) => (
                <div
                  key={message.id}
                  className={
                    message.role === 'user'
                      ? 'reader-chat-bubble reader-chat-bubble--user'
                      : 'reader-chat-bubble reader-chat-bubble--assistant'
                  }
                >
                  <div className="reader-chat-bubble__body">
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
                            <span key={`t-${index}`} className="reader-chat-bubble__text">
                              {segment.value}
                            </span>
                          ),
                        )
                      : message.role === 'assistant' ? (
                          <RichText className="reader-chat-bubble__rich">{message.content}</RichText>
                        ) : (
                          message.content
                        )}
                  </div>
                </div>
              ))}
              <AssistantReplySelectionBar
                containerRef={threadRef}
                locale={locale}
                onAsk={(attachment) => {
                  setActiveTab('assistant');
                  composerRef.current?.insertAttachment(attachment);
                  setComposerEmpty(false);
                  composerRef.current?.focus();
                }}
              />
            </div>
          )}
        </div>

        <div className="reader-chat-input-wrap">
          {contextUsage ? (
            <div
              className={`reader-context-meter reader-context-meter--${contextTone}`}
              title={
                locale === 'ru'
                  ? `Контекст: ${contextUsage.used_tokens.toLocaleString('ru-RU')} / ${contextUsage.limit_tokens.toLocaleString('ru-RU')} токенов` +
                    (contextUsage.has_full_paper
                      ? ` · статья ≈ ${formatTokens(contextUsage.paper_tokens)}`
                      : ' · полный текст статьи ещё не готов')
                  : `Context: ${contextUsage.used_tokens.toLocaleString('en-US')} / ${contextUsage.limit_tokens.toLocaleString('en-US')} tokens` +
                    (contextUsage.has_full_paper
                      ? ` · paper ≈ ${formatTokens(contextUsage.paper_tokens)}`
                      : ' · full paper text not ready yet')
              }
            >
              <div className="reader-context-meter__track" aria-hidden="true">
                <div
                  className="reader-context-meter__fill"
                  style={{ width: `${Math.min(100, Math.max(0, contextUsage.percent))}%` }}
                />
              </div>
              <div className="reader-context-meter__meta">
                <span>
                  {locale === 'ru' ? 'Контекст' : 'Context'}{' '}
                  {Math.round(contextUsage.percent)}%
                </span>
                <span>
                  {formatTokens(contextUsage.used_tokens)} / {formatTokens(contextUsage.limit_tokens)}
                </span>
              </div>
            </div>
          ) : null}
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
              {models.length > 0 ? (
                <label className="reader-model-picker" title={selectedModel || undefined}>
                  <span className="sr-only">{locale === 'ru' ? 'Модель' : 'Model'}</span>
                  <select
                    value={selectedModel}
                    onChange={(e) => setSelectedModel(e.target.value)}
                    disabled={isSending}
                  >
                    {models.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.label}
                      </option>
                    ))}
                  </select>
                </label>
              ) : (
                <span className="reader-model-picker-fallback" title={selectedModel || undefined}>
                  {selectedModel
                    ? selectedModel.includes('/')
                      ? selectedModel.slice(selectedModel.lastIndexOf('/') + 1)
                      : selectedModel
                    : locale === 'ru'
                      ? 'модель…'
                      : 'model…'}
                </span>
              )}
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
          ) : (annotations?.length ?? 0) === 0 ? (
            <div className="library-page__state">
              Выделите текст в PDF — появится форма для заметки
            </div>
          ) : (
            (annotations ?? []).map((note) => (
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
