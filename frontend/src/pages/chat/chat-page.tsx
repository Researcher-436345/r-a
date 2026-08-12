import { useNavigate, useParams, useSearch } from '@tanstack/react-router';
import {
  Check,
  Copy,
  CornerDownRight,
  Loader2,
  PanelLeftClose,
  PanelLeftOpen,
  Sparkles,
  SquarePen,
} from 'lucide-react';
import {
  Fragment,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import {
  getResearchChat,
  listResearchChats,
  streamResearchMessage,
  type ResearchChatSummary,
} from '../../features/chat/api';
import { MarkdownMessage } from '../../features/chat/components/markdown-message';
import { ResearchComposer } from '../../features/chat/components/research-composer';
import {
  createChatId,
  parseResearchMode,
  type ChatMessage,
  type ResearchMode,
} from '../../features/chat/types';
import { ApiError } from '../../shared/api/client';
import { useI18n, type Locale } from '../../shared/i18n/i18n-context';

interface ChatRouteSearch {
  q?: unknown;
  query?: unknown;
  question?: unknown;
  mode?: unknown;
}

type ChatScreen = 'conversation' | 'new';
const PENDING_STATUS_INTERVAL_MS = 2_400;

const chatCopy = {
  ru: {
    newChat: 'Новый чат',
    today: 'Сегодня',
    earlier: 'Ранее',
    currentTitle: 'Производительный веб-поиск',
    upgrade: 'Перейти на Pro',
    emptyTitle: 'Что исследуем сегодня?',
    historyLabel: 'История диалогов',
    collapseHistory: 'Свернуть список чатов',
    expandHistory: 'Развернуть список чатов',
    pendingStatuses: [
      'Формулирую поисковые запросы…',
      'Ищу релевантные статьи…',
      'Проверяю свежие публикации…',
      'Читаю первоисточники…',
      'Сопоставляю результаты…',
    ],
    requestFailed: 'Не удалось выполнить исследовательский поиск',
    copyMarkdown: 'Копировать Markdown',
    copiedMarkdown: 'Скопировано',
    inputLabel: 'Сообщение',
    modeLabel: 'Режим исследования',
    suggestions: [
      'Что интересного вышло за последнее время?',
      'Собери обзор по свежим методам RLHF',
      'Какие статьи стоит прочитать по агентам?',
    ],
  },
  en: {
    newChat: 'New chat',
    today: 'Today',
    earlier: 'Earlier',
    currentTitle: 'High-performance web search',
    upgrade: 'Upgrade to Pro',
    emptyTitle: 'What are we researching today?',
    historyLabel: 'Conversation history',
    collapseHistory: 'Collapse chat list',
    expandHistory: 'Expand chat list',
    pendingStatuses: [
      'Formulating search queries…',
      'Finding relevant papers…',
      'Checking recent publications…',
      'Reading primary sources…',
      'Comparing the findings…',
    ],
    requestFailed: 'The research search could not be completed',
    copyMarkdown: 'Copy Markdown',
    copiedMarkdown: 'Copied',
    inputLabel: 'Message',
    modeLabel: 'Research mode',
    suggestions: [
      'What interesting work was published recently?',
      'Review the latest RLHF methods',
      'Which papers about agents should I read?',
    ],
  },
} satisfies Record<
  Locale,
  {
    newChat: string;
    today: string;
    earlier: string;
    currentTitle: string;
    upgrade: string;
    emptyTitle: string;
    historyLabel: string;
    collapseHistory: string;
    expandHistory: string;
    pendingStatuses: string[];
    requestFailed: string;
    copyMarkdown: string;
    copiedMarkdown: string;
    inputLabel: string;
    modeLabel: string;
    suggestions: string[];
  }
>;

function readInitialQuestion(search: ChatRouteSearch) {
  const candidate = [search.q, search.query, search.question].find(
    (value): value is string => typeof value === 'string' && Boolean(value.trim()),
  );

  return candidate?.trim() ?? '';
}

function conversationTitle(question: string, locale: Locale) {
  const compact = question.replace(/\s+/g, ' ').trim();
  const isSearchConversation =
    /(?:web\s*search|deep\s*research|веб[- ]?поиск|глубок\w*\s+исследован)/i.test(
      compact,
    ) && /(?:производ|подход|perform|approach|efficien)/i.test(compact);

  if (isSearchConversation) {
    return chatCopy[locale].currentTitle;
  }

  return compact.length > 46 ? `${compact.slice(0, 45).trimEnd()}…` : compact;
}

function createMessageId(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function PendingResearchMessage({ locale }: { locale: Locale }) {
  const [statusIndex, setStatusIndex] = useState(0);
  const statuses = chatCopy[locale].pendingStatuses;

  useEffect(() => {
    setStatusIndex(0);
    const interval = window.setInterval(() => {
      setStatusIndex((current) => (current + 1) % statuses.length);
    }, PENDING_STATUS_INTERVAL_MS);

    return () => window.clearInterval(interval);
  }, [locale, statuses.length]);

  return (
    <div
      className="chat-message chat-message--assistant chat-message--pending"
      role="status"
      aria-live="polite"
    >
      <Loader2
        className="chat-message__loader"
        aria-hidden="true"
        size={17}
        strokeWidth={2}
      />
      <span className="chat-message__pending-status" key={statusIndex}>
        {statuses[statusIndex]}
      </span>
    </div>
  );
}

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement('textarea');
  textarea.value = value;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  const copied = document.execCommand('copy');
  textarea.remove();
  if (!copied) {
    throw new Error('Clipboard copy failed');
  }
}

function AssistantMessage({ content, locale }: { content: string; locale: Locale }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const timeout = window.setTimeout(() => setCopied(false), 2_000);
    return () => window.clearTimeout(timeout);
  }, [copied]);

  const handleCopy = async () => {
    try {
      await copyText(content);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  const label = copied
    ? chatCopy[locale].copiedMarkdown
    : chatCopy[locale].copyMarkdown;

  return (
    <article className="chat-message chat-message--assistant" aria-live="polite">
      <MarkdownMessage content={content} />
      <div className="chat-message__actions">
        <button
          className="chat-message__copy"
          type="button"
          onClick={() => void handleCopy()}
          aria-label={label}
          title={label}
        >
          {copied ? (
            <Check aria-hidden="true" size={14} strokeWidth={2} />
          ) : (
            <Copy aria-hidden="true" size={14} strokeWidth={2} />
          )}
          <span>{label}</span>
        </button>
      </div>
    </article>
  );
}

function ChatMessageView({
  message,
  locale,
}: {
  message: ChatMessage;
  locale: Locale;
}) {
  if (message.pending) {
    return <PendingResearchMessage locale={locale} />;
  }

  if (message.role === 'user') {
    return (
      <article className="chat-message chat-message--user">
        <p>{message.content}</p>
      </article>
    );
  }

  return <AssistantMessage content={message.content} locale={locale} />;
}

export function ChatPage() {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const { chatId } = useParams({ strict: false }) as { chatId?: string };
  const routeSearch = useSearch({ strict: false }) as ChatRouteSearch;
  const initialQuestion = readInitialQuestion(routeSearch);
  const routeMode = parseResearchMode(routeSearch.mode);
  const routeConversationKey = `${chatId ?? 'chat'}\u0000${routeMode}\u0000${initialQuestion}`;
  const copy = chatCopy[locale];

  const [screen, setScreen] = useState<ChatScreen>(
    chatId === 'new' ? 'new' : 'conversation',
  );
  const [isHistoryOpen, setIsHistoryOpen] = useState(
    () =>
      typeof window === 'undefined' ||
      typeof window.matchMedia !== 'function' ||
      !window.matchMedia('(max-width: 640px)').matches,
  );
  const [activeQuestion, setActiveQuestion] = useState(initialQuestion);
  const [composerMode, setComposerMode] = useState<ResearchMode>(routeMode);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [chatHistory, setChatHistory] = useState<ResearchChatSummary[]>([]);
  const [draft, setDraft] = useState('');
  const [isSending, setIsSending] = useState(false);
  const loadedRouteConversationKeyRef = useRef('');
  const activeStreamRef = useRef<AbortController | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const threadEndRef = useRef<HTMLDivElement | null>(null);

  const title = useMemo(
    () => activeQuestion ? conversationTitle(activeQuestion, locale) : copy.newChat,
    [activeQuestion, locale],
  );

  const refreshChatHistory = async () => {
    try {
      setChatHistory(await listResearchChats());
    } catch {
      // The active conversation remains usable if the history list cannot refresh.
    }
  };

  const streamIntoMessage = async (
    targetChatId: string,
    content: string,
    mode: ResearchMode,
    assistantId: string,
    requestKey: string,
  ) => {
    const controller = new AbortController();
    activeStreamRef.current = controller;
    setIsSending(true);
    try {
      const storedMessage = await streamResearchMessage({
        chatId: targetChatId,
        message: content,
        mode,
        signal: controller.signal,
        onDelta: (delta) => {
          if (loadedRouteConversationKeyRef.current !== requestKey) {
            return;
          }
          setMessages((current) =>
            current.map((message) =>
              message.id === assistantId
                ? {
                    ...message,
                    content: message.content + delta,
                    pending: false,
                  }
                : message,
            ),
          );
        },
      });
      if (loadedRouteConversationKeyRef.current === requestKey) {
        setMessages((current) =>
          current.map((message) =>
            message.id === assistantId
              ? {
                  ...message,
                  id: storedMessage.id,
                  sources: storedMessage.sources,
                  pending: false,
                }
              : message,
          ),
        );
      }
    } catch (error) {
      if (controller.signal.aborted || loadedRouteConversationKeyRef.current !== requestKey) {
        return;
      }
      const detail = error instanceof Error ? error.message : String(error);
      setMessages((current) =>
        current.map((message) =>
          message.id === assistantId
            ? {
                ...message,
                content: `${message.content ? `${message.content}\n\n---\n\n` : ''}> **${copy.requestFailed}.** ${detail}`,
                pending: false,
              }
            : message,
        ),
      );
    } finally {
      if (activeStreamRef.current === controller) {
        activeStreamRef.current = null;
        if (loadedRouteConversationKeyRef.current === requestKey) {
          setIsSending(false);
        }
      }
      void refreshChatHistory();
    }
  };

  useEffect(() => {
    void refreshChatHistory();
  }, []);

  useEffect(() => {
    if (loadedRouteConversationKeyRef.current === routeConversationKey) {
      return;
    }

    activeStreamRef.current?.abort();
    activeStreamRef.current = null;
    loadedRouteConversationKeyRef.current = routeConversationKey;
    setDraft('');
    setIsSending(false);

    if (!chatId || chatId === 'new') {
      setActiveQuestion('');
      setMessages([]);
      setComposerMode('web');
      setScreen('new');
      return;
    }

    setScreen('conversation');
    setActiveQuestion(initialQuestion);
    setComposerMode(routeMode);
    setMessages([]);

    void (async () => {
      try {
        const storedChat = await getResearchChat(chatId);
        if (loadedRouteConversationKeyRef.current !== routeConversationKey) {
          return;
        }
        const firstQuestion =
          storedChat.messages.find((message) => message.role === 'user')?.content ??
          storedChat.title;
        setActiveQuestion(firstQuestion);
        setComposerMode(storedChat.mode);
        setMessages(
          storedChat.messages.map((message) => ({
            id: message.id,
            role: message.role,
            content: message.content,
            sources: message.sources,
          })),
        );
      } catch (error) {
        if (
          error instanceof ApiError &&
          error.status === 404 &&
          initialQuestion &&
          loadedRouteConversationKeyRef.current === routeConversationKey
        ) {
          const assistantId = createMessageId('assistant');
          setMessages([
            { id: createMessageId('user'), role: 'user', content: initialQuestion },
            { id: assistantId, role: 'assistant', content: '', pending: true },
          ]);
          const now = new Date().toISOString();
          setChatHistory((current) => [
            {
              id: chatId,
              title: conversationTitle(initialQuestion, locale),
              mode: routeMode,
              created_at: now,
              updated_at: now,
            },
            ...current.filter((chat) => chat.id !== chatId),
          ]);
          await streamIntoMessage(
            chatId,
            initialQuestion,
            routeMode,
            assistantId,
            routeConversationKey,
          );
          return;
        }
        if (loadedRouteConversationKeyRef.current !== routeConversationKey) {
          return;
        }
        const detail = error instanceof Error ? error.message : String(error);
        setMessages([
          {
            id: createMessageId('assistant'),
            role: 'assistant',
            content: `> **${copy.requestFailed}.** ${detail}`,
          },
        ]);
      }
    })();
  }, [chatId, initialQuestion, routeConversationKey, routeMode]);

  useEffect(() => {
    if (messages.length <= 2) {
      return;
    }
    threadEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [messages]);

  const openNewChat = () => {
    activeStreamRef.current?.abort();
    activeStreamRef.current = null;
    setDraft('');
    setIsSending(false);
    setScreen('new');
    if (window.matchMedia('(max-width: 640px)').matches) {
      setIsHistoryOpen(false);
    }
    window.requestAnimationFrame(() => composerRef.current?.focus());
    void navigate({
      to: '/chat/$chatId',
      params: { chatId: 'new' },
      search: { q: '', mode: 'web' },
    });
  };

  const startNewConversation = (question: string) => {
    const nextChatId = createChatId();
    const nextMode = composerMode;

    setActiveQuestion(question);
    setDraft('');

    void navigate({
      to: '/chat/$chatId',
      params: { chatId: nextChatId },
      search: { q: question, mode: nextMode },
    });
  };

  const sendFollowUp = (content: string) => {
    if (!chatId || chatId === 'new') {
      return;
    }
    const assistantId = createMessageId('assistant');
    const responseMode = composerMode;

    setMessages((current) => [
      ...current,
      {
        id: createMessageId('user'),
        role: 'user',
        content,
      },
      {
        id: assistantId,
        role: 'assistant',
        content: '',
        pending: true,
      },
    ]);
    setDraft('');
    void streamIntoMessage(
      chatId,
      content,
      responseMode,
      assistantId,
      routeConversationKey,
    );
  };

  const submitDraft = () => {
    const content = draft.trim();
    if (!content || isSending) {
      return;
    }

    if (screen === 'new') {
      startNewConversation(content);
      return;
    }

    sendFollowUp(content);
  };

  const chooseSuggestion = (suggestion: string) => {
    setDraft(suggestion);
    window.requestAnimationFrame(() => composerRef.current?.focus());
  };

  const openStoredChat = (chat: ResearchChatSummary) => {
    setScreen('conversation');
    if (window.matchMedia('(max-width: 640px)').matches) {
      setIsHistoryOpen(false);
    }
    void navigate({
      to: '/chat/$chatId',
      params: { chatId: chat.id },
      search: { q: '', mode: chat.mode },
    });
  };

  const today = new Date().toDateString();
  const todayChats = chatHistory.filter(
    (chat) => new Date(chat.updated_at).toDateString() === today,
  );
  const earlierChats = chatHistory.filter(
    (chat) => new Date(chat.updated_at).toDateString() !== today,
  );

  return (
    <div className="chat-page">
      {isHistoryOpen ? (
        <aside className="chat-history" aria-label={copy.historyLabel}>
          <div className="chat-history__header">
            <button
              className="chat-history__new"
              type="button"
              onClick={openNewChat}
            >
              <SquarePen aria-hidden="true" size={15} strokeWidth={2} />
              <span>{copy.newChat}</span>
            </button>
            <button
              className="chat-history__collapse"
              type="button"
              onClick={() => setIsHistoryOpen(false)}
              aria-label={copy.collapseHistory}
              title={copy.collapseHistory}
            >
              <PanelLeftClose aria-hidden="true" size={16} strokeWidth={2} />
            </button>
          </div>

          <nav className="chat-history__nav">
            <section className="chat-history__group">
              <h2>{copy.today}</h2>
              <div className="chat-history__items">
                {todayChats.map((chat) => (
                  <button
                    className={
                      screen === 'conversation' && chat.id === chatId
                        ? 'chat-history__item chat-history__item--active'
                        : 'chat-history__item'
                    }
                    type="button"
                    aria-current={
                      screen === 'conversation' && chat.id === chatId
                        ? 'page'
                        : undefined
                    }
                    onClick={() => openStoredChat(chat)}
                    title={chat.title}
                    key={chat.id}
                  >
                    <span>{chat.title}</span>
                  </button>
                ))}
              </div>
            </section>

            {earlierChats.length > 0 ? (
              <section className="chat-history__group">
                <h2>{copy.earlier}</h2>
                <div className="chat-history__items">
                  {earlierChats.map((chat) => (
                    <button
                      className={
                        screen === 'conversation' && chat.id === chatId
                          ? 'chat-history__item chat-history__item--active'
                          : 'chat-history__item'
                      }
                      type="button"
                      aria-current={
                        screen === 'conversation' && chat.id === chatId
                          ? 'page'
                          : undefined
                      }
                      onClick={() => openStoredChat(chat)}
                      title={chat.title}
                      key={chat.id}
                    >
                      <span>{chat.title}</span>
                    </button>
                  ))}
                </div>
              </section>
            ) : null}
          </nav>
        </aside>
      ) : null}

      <section
        className="chat-page__main"
        aria-label={locale === 'ru' ? 'Исследовательский чат' : 'Research chat'}
      >
        <header className="chat-header">
          {!isHistoryOpen ? (
            <button
              className="chat-header__history-toggle"
              type="button"
              onClick={() => setIsHistoryOpen(true)}
              aria-label={copy.expandHistory}
              title={copy.expandHistory}
            >
              <PanelLeftOpen aria-hidden="true" size={16} strokeWidth={2} />
            </button>
          ) : null}
          <h1>{screen === 'new' ? copy.newChat : title}</h1>
          <span className="chat-header__spacer" />
          <button
            className="chat-header__upgrade"
            type="button"
            aria-label={copy.upgrade}
            title={copy.upgrade}
          >
            <Sparkles aria-hidden="true" size={15} strokeWidth={2} />
            <span>{copy.upgrade}</span>
          </button>
        </header>

        {screen === 'new' ? (
          <section className="chat-empty-state" aria-labelledby="chat-empty-title">
            <h2 id="chat-empty-title">{copy.emptyTitle}</h2>
            <div className="chat-empty-state__suggestions">
              {copy.suggestions.map((suggestion) => (
                <button
                  className="chat-empty-state__suggestion"
                  type="button"
                  onClick={() => chooseSuggestion(suggestion)}
                  key={suggestion}
                >
                  <CornerDownRight aria-hidden="true" size={15} strokeWidth={2} />
                  <span>{suggestion}</span>
                </button>
              ))}
            </div>
          </section>
        ) : (
          <div className="chat-page__scroll">
            <div className="chat-thread">
              {messages.map((message) => (
                <Fragment key={message.id}>
                  <ChatMessageView message={message} locale={locale} />
                </Fragment>
              ))}
              <div
                className="chat-thread__end"
                ref={threadEndRef}
                aria-hidden="true"
              />
            </div>
          </div>
        )}

        <div className="chat-page__composer">
          <ResearchComposer
            ref={composerRef}
            className={
              screen === 'conversation'
                ? 'chat-composer chat-composer--conversation'
                : 'chat-composer chat-composer--new'
            }
            value={draft}
            mode={composerMode}
            placeholder={t('ask.placeholder')}
            attachLabel={t('ask.attach')}
            webSearchLabel={t('ask.webSearch')}
            deepResearchLabel={t('ask.deepResearch')}
            sendHint={t('ask.sendHint')}
            sendLabel={t('ask.send')}
            inputAriaLabel={copy.inputLabel}
            modeAriaLabel={copy.modeLabel}
            onChange={setDraft}
            onModeChange={setComposerMode}
            onSubmit={submitDraft}
            disabled={isSending}
          />
        </div>
      </section>
    </div>
  );
}
