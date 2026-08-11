import {
  ArrowUp,
  Check,
  CornerDownRight,
  GitCompare,
  Highlighter,
  Layers,
  NotebookPen,
  Paperclip,
  Sparkles,
  X,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';

import {
  chatPaperStream,
  createAnnotation,
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
const CHAT_NOTE_COLOR = '#cbb8de';
const FREE_NOTE_COLOR = '#a9c7e0';

interface LocalChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  attachments?: ChatContextAttachment[];
  segments?: ComposerSegment[];
  /** Полный текст для будущей модели */
  modelPayload?: string;
}

function formatTokens(n: number): string {
  if (n >= 10000) {
    return `${Math.round(n / 1000)}k`;
  }
  if (n >= 1000) {
    return `${(n / 1000).toFixed(1)}k`;
  }
  return String(n);
}

function messageNoteBody(message: LocalChatMessage): string {
  return (message.content || message.modelPayload || '').trim();
}

function canSaveMessageAsNote(message: LocalChatMessage): boolean {
  const body = messageNoteBody(message);
  if (!body) {
    return false;
  }
  if (body === 'Думаю…' || body === 'Thinking…') {
    return false;
  }
  if (body.startsWith('Ошибка:') || body.startsWith('Error:')) {
    return false;
  }
  return true;
}

function isPersistedMessageId(id: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(id);
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
  /** Клик по [p.N] в ответе ассистента — прыжок на страницу PDF */
  onPageCite?: (page: number, quote?: string) => void;
  onNoteUpdated?: (note: PaperAnnotation) => void;
  onNoteCreated?: (note: PaperAnnotation) => void;
  onAnnotationsChange?: () => void;
  focusChatMessageId?: string | null;
  focusChatMessageToken?: number;
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
  onPageCite,
  onNoteUpdated,
  onNoteCreated,
  onAnnotationsChange,
  focusChatMessageId = null,
  focusChatMessageToken = 0,
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
  const [savingNoteMessageId, setSavingNoteMessageId] = useState<string | null>(null);
  const [savedNoteMessageId, setSavedNoteMessageId] = useState<string | null>(null);
  const [flashMessageId, setFlashMessageId] = useState<string | null>(null);
  /** Одно выделение — цитата над инпутом; несколько — чипы в композере */
  const [focusQuote, setFocusQuote] = useState<ChatContextAttachment | null>(null);
  const [freeNoteDraft, setFreeNoteDraft] = useState('');
  const [isSavingFreeNote, setIsSavingFreeNote] = useState(false);
  const freeNoteRef = useRef<HTMLTextAreaElement | null>(null);
  const composerRef = useRef<ChatComposerHandle | null>(null);
  const threadRef = useRef<HTMLDivElement | null>(null);
  const lastInsertedId = useRef<string | null>(null);
  const focusQuoteRef = useRef<ChatContextAttachment | null>(null);
  focusQuoteRef.current = focusQuote;

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
    if (!focusChatMessageToken || !focusChatMessageId) {
      return;
    }
    setActiveTab('assistant');
    window.setTimeout(() => {
      const row = document.querySelector(`[data-chat-message-id="${focusChatMessageId}"]`);
      row?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      setFlashMessageId(focusChatMessageId);
      window.setTimeout(() => {
        setFlashMessageId((current) => (current === focusChatMessageId ? null : current));
      }, 1600);
    }, 60);
  }, [focusChatMessageToken, focusChatMessageId]);

  const syncComposerEmpty = () => {
    const emptyComposer = composerRef.current?.isEmpty() ?? true;
    setComposerEmpty(emptyComposer && !focusQuoteRef.current);
  };

  const addContextAttachment = (attachment: ChatContextAttachment) => {
    setActiveTab('assistant');
    const chipCount = composerRef.current?.getSnapshot().attachments.length ?? 0;
    const currentQuote = focusQuoteRef.current;

    if (currentQuote && chipCount === 0) {
      composerRef.current?.insertAttachment(currentQuote);
      composerRef.current?.insertAttachment(attachment);
      setFocusQuote(null);
    } else if (!currentQuote && chipCount === 0) {
      setFocusQuote(attachment);
    } else {
      composerRef.current?.insertAttachment(attachment);
    }

    setComposerEmpty(false);
    composerRef.current?.focus();
  };

  useEffect(() => {
    syncComposerEmpty();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only when quote pin changes
  }, [focusQuote]);

  useEffect(() => {
    if (!contextAttachment || contextAttachment.id === lastInsertedId.current) {
      return;
    }
    lastInsertedId.current = contextAttachment.id;
    addContextAttachment(contextAttachment);
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

  const handleSaveMessageAsNote = async (message: LocalChatMessage) => {
    if (!paperId || !canSaveMessageAsNote(message) || savingNoteMessageId) {
      return;
    }
    const body = messageNoteBody(message);
    const quote =
      message.role === 'assistant'
        ? locale === 'ru'
          ? 'Чат · ответ AI'
          : 'Chat · AI reply'
        : locale === 'ru'
          ? 'Чат · вопрос'
          : 'Chat · question';

    setSavingNoteMessageId(message.id);
    setError(null);
    try {
      const created = await createAnnotation(paperId, {
        page: 0,
        selected_text: quote,
        note: body,
        color: CHAT_NOTE_COLOR,
        source_chat_message_id: isPersistedMessageId(message.id) ? message.id : null,
      });
      onNoteCreated?.(created);
      setSavedNoteMessageId(message.id);
      setActiveTab('notes');
      window.setTimeout(() => {
        setSavedNoteMessageId((current) => (current === message.id ? null : current));
      }, 1800);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.detail
          : err instanceof Error
            ? err.message
            : locale === 'ru'
              ? 'Не удалось сохранить в заметки'
              : 'Could not save to notes',
      );
    } finally {
      setSavingNoteMessageId(null);
    }
  };

  const handleSaveSelectionAsNote = async (payload: {
    text: string;
    messageId: string | null;
  }) => {
    if (!paperId || savingNoteMessageId) {
      return;
    }
    const body = payload.text.trim();
    if (body.length < 2) {
      return;
    }
    const quote = locale === 'ru' ? 'Чат · фрагмент ответа' : 'Chat · reply excerpt';
    const trackingId = payload.messageId || `selection-${Date.now()}`;

    setSavingNoteMessageId(trackingId);
    setError(null);
    try {
      const created = await createAnnotation(paperId, {
        page: 0,
        selected_text: quote,
        note: body,
        color: CHAT_NOTE_COLOR,
        source_chat_message_id:
          payload.messageId && isPersistedMessageId(payload.messageId)
            ? payload.messageId
            : null,
      });
      onNoteCreated?.(created);
      if (payload.messageId) {
        setSavedNoteMessageId(payload.messageId);
        window.setTimeout(() => {
          setSavedNoteMessageId((current) =>
            current === payload.messageId ? null : current,
          );
        }, 1800);
      }
      setActiveTab('notes');
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.detail
          : err instanceof Error
            ? err.message
            : locale === 'ru'
              ? 'Не удалось сохранить в заметки'
              : 'Could not save to notes',
      );
    } finally {
      setSavingNoteMessageId(null);
    }
  };

  const handleCreateFreeNote = async () => {
    if (!paperId || isSavingFreeNote) {
      return;
    }
    const body = freeNoteDraft.trim();
    if (!body) {
      return;
    }

    setIsSavingFreeNote(true);
    setError(null);
    try {
      const created = await createAnnotation(paperId, {
        page: 0,
        selected_text: locale === 'ru' ? 'Свободная заметка' : 'Free note',
        note: body,
        color: FREE_NOTE_COLOR,
      });
      onNoteCreated?.(created);
      setFreeNoteDraft('');
      freeNoteRef.current?.blur();
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.detail
          : err instanceof Error
            ? err.message
            : locale === 'ru'
              ? 'Не удалось сохранить заметку'
              : 'Could not save note',
      );
    } finally {
      setIsSavingFreeNote(false);
    }
  };

  const handleSend = async () => {
    if (isSending) {
      return;
    }
    if (!paperId) {
      return;
    }
    const snapshot = composerRef.current?.getSnapshot();
    if (!snapshot) {
      return;
    }

    const attachments = focusQuote
      ? [focusQuote, ...snapshot.attachments]
      : snapshot.attachments;
    const hasText = Boolean(snapshot.plainText);
    const defaultAsk = locale === 'ru' ? 'Объясни этот фрагмент' : 'Explain this passage';
    const content = hasText ? snapshot.plainText : attachments.length ? defaultAsk : '';

    if (!content && !attachments.length) {
      return;
    }

    const quoteSegments: ComposerSegment[] = focusQuote
      ? [{ type: 'chip', attachment: focusQuote }]
      : [];
    const segments: ComposerSegment[] =
      hasText || !attachments.length
        ? [...quoteSegments, ...snapshot.segments]
        : [
            ...quoteSegments,
            ...snapshot.segments,
            { type: 'text', value: ` ${defaultAsk}` },
          ];

    const contextParts = [
      ...(focusQuote ? [focusQuote.text] : []),
      snapshot.modelText,
      !snapshot.modelText && !focusQuote ? snapshot.plainText : '',
    ]
      .map((part) => part.trim())
      .filter(Boolean);
    const requestContext = contextParts.join('\n\n') || null;

    const tempUserId = `u-${Date.now()}`;
    const tempAssistantId = `a-${Date.now()}`;
    const userMessage: LocalChatMessage = {
      id: tempUserId,
      role: 'user',
      content,
      attachments: attachments.length ? attachments : undefined,
      segments,
      modelPayload: requestContext || content,
    };

    const assistantMessage: LocalChatMessage = {
      id: tempAssistantId,
      role: 'assistant',
      content: locale === 'ru' ? 'Думаю…' : 'Thinking…',
    };

    setMessages((current) => [...current, userMessage, assistantMessage]);
    composerRef.current?.clear();
    setFocusQuote(null);
    setComposerEmpty(true);

    setIsSending(true);
    setError(null);
    try {
      const requestMessage = content || defaultAsk;
      let streamed = '';
      const res = await chatPaperStream(
        paperId,
        {
          message: requestMessage,
          context_text: requestContext,
          model: selectedModel || undefined,
        },
        {
          onDelta: (chunk) => {
            streamed += chunk;
            const next = streamed;
            setMessages((current) =>
              current.map((msg) =>
                msg.id === tempAssistantId ? { ...msg, content: next } : msg,
              ),
            );
          },
        },
      );

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
              content: res.reply || streamed,
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
                  data-chat-message-id={message.id}
                  className={
                    [
                      message.role === 'user'
                        ? 'reader-chat-row reader-chat-row--user'
                        : 'reader-chat-row reader-chat-row--assistant',
                      flashMessageId === message.id ? 'reader-chat-row--flash' : '',
                    ]
                      .filter(Boolean)
                      .join(' ')
                  }
                >
                  {message.role === 'user' && canSaveMessageAsNote(message) ? (
                    <button
                      type="button"
                      className={
                        savedNoteMessageId === message.id || savingNoteMessageId === message.id
                          ? 'reader-chat-row__note-btn reader-chat-row__note-btn--visible'
                          : 'reader-chat-row__note-btn'
                      }
                      title={locale === 'ru' ? 'Добавить в заметки' : 'Save to notes'}
                      aria-label={locale === 'ru' ? 'Добавить в заметки' : 'Save to notes'}
                      disabled={savingNoteMessageId === message.id}
                      onClick={() => void handleSaveMessageAsNote(message)}
                    >
                      {savedNoteMessageId === message.id ? (
                        <Check aria-hidden="true" size={14} strokeWidth={2.2} />
                      ) : (
                        <NotebookPen aria-hidden="true" size={14} strokeWidth={2} />
                      )}
                    </button>
                  ) : null}
                  <div
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
                            <RichText className="reader-chat-bubble__rich" onPageCite={onPageCite}>
                              {message.content}
                            </RichText>
                          ) : (
                            message.content
                          )}
                    </div>
                  </div>
                  {message.role === 'assistant' && canSaveMessageAsNote(message) ? (
                    <button
                      type="button"
                      className={
                        savedNoteMessageId === message.id || savingNoteMessageId === message.id
                          ? 'reader-chat-row__note-btn reader-chat-row__note-btn--visible'
                          : 'reader-chat-row__note-btn'
                      }
                      title={locale === 'ru' ? 'Добавить в заметки' : 'Save to notes'}
                      aria-label={locale === 'ru' ? 'Добавить в заметки' : 'Save to notes'}
                      disabled={savingNoteMessageId === message.id}
                      onClick={() => void handleSaveMessageAsNote(message)}
                    >
                      {savedNoteMessageId === message.id ? (
                        <Check aria-hidden="true" size={14} strokeWidth={2.2} />
                      ) : (
                        <NotebookPen aria-hidden="true" size={14} strokeWidth={2} />
                      )}
                    </button>
                  ) : null}
                </div>
              ))}
              <AssistantReplySelectionBar
                containerRef={threadRef}
                locale={locale}
                isSavingNote={Boolean(savingNoteMessageId)}
                onAsk={(attachment) => {
                  addContextAttachment(attachment);
                }}
                onSaveNote={(payload) => {
                  void handleSaveSelectionAsNote(payload);
                }}
              />
            </div>
          )}
        </div>

        <div className="reader-chat-input-wrap">
          {focusQuote ? (
            <div className="reader-chat-focus-quote">
              <button
                type="button"
                className="reader-chat-focus-quote__body"
                title={focusQuote.preview || focusQuote.text}
                onClick={() => onPassageSelect?.(focusQuote)}
              >
                <span className="reader-chat-focus-quote__marks" aria-hidden="true">
                  “”
                </span>
                <span className="reader-chat-focus-quote__content">
                  <span className="reader-chat-focus-quote__label">{focusQuote.locationLabel}</span>
                  <span className="reader-chat-focus-quote__text">
                    {focusQuote.preview || focusQuote.text}
                  </span>
                </span>
              </button>
              <button
                type="button"
                className="reader-chat-focus-quote__close"
                title={locale === 'ru' ? 'Убрать фрагмент' : 'Remove passage'}
                aria-label={locale === 'ru' ? 'Убрать фрагмент' : 'Remove passage'}
                onClick={() => {
                  setFocusQuote(null);
                  setComposerEmpty(composerRef.current?.isEmpty() ?? true);
                }}
              >
                <X aria-hidden="true" size={14} strokeWidth={2} />
              </button>
            </div>
          ) : null}
          <div className="reader-chat-input">
            <ChatComposer
              ref={composerRef}
              placeholder={
                focusQuote
                  ? locale === 'ru'
                    ? 'Спроси об этом фрагменте…'
                    : 'Ask about this passage…'
                  : locale === 'ru'
                    ? 'Спроси или добавь фрагменты из PDF…'
                    : 'Ask or add passages from the PDF…'
              }
              onChange={syncComposerEmpty}
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
              {contextUsage ? (
                <div
                  className={`reader-context-chip reader-context-chip--${contextTone}`}
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
                  <span className="reader-context-chip__track" aria-hidden="true">
                    <span
                      className="reader-context-chip__fill"
                      style={{ width: `${Math.min(100, Math.max(0, contextUsage.percent))}%` }}
                    />
                  </span>
                  <span className="reader-context-chip__label">
                    {Math.round(contextUsage.percent)}%
                  </span>
                </div>
              ) : null}
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
          ) : (
            <>
              <form
                className="reader-notes-composer"
                onSubmit={(event) => {
                  event.preventDefault();
                  void handleCreateFreeNote();
                }}
              >
                <textarea
                  ref={freeNoteRef}
                  className="reader-notes-composer__input"
                  rows={3}
                  value={freeNoteDraft}
                  onChange={(event) => setFreeNoteDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
                      event.preventDefault();
                      void handleCreateFreeNote();
                    }
                  }}
                  placeholder={
                    locale === 'ru'
                      ? 'Новая заметка по статье…'
                      : 'New note about this paper…'
                  }
                  disabled={isSavingFreeNote}
                />
                <div className="reader-notes-composer__footer">
                  <span className="reader-notes-composer__hint">
                    {locale === 'ru' ? '⌘/Ctrl + Enter' : '⌘/Ctrl + Enter'}
                  </span>
                  <button
                    type="submit"
                    className="reader-notes-composer__submit"
                    disabled={isSavingFreeNote || !freeNoteDraft.trim()}
                  >
                    {isSavingFreeNote
                      ? '…'
                      : locale === 'ru'
                        ? 'Добавить'
                        : 'Add note'}
                  </button>
                </div>
              </form>

              {(annotations?.length ?? 0) === 0 ? (
                <div className="library-page__state">
                  {locale === 'ru'
                    ? 'Пока пусто — напишите выше или выделите текст в PDF / чате'
                    : 'Empty so far — write above or select text in the PDF / chat'}
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
            </>
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
