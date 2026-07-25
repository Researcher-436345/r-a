import { useNavigate, useParams, useSearch } from '@tanstack/react-router';
import {
  CheckCircle2,
  ChevronDown,
  ChevronUp,
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

import { MarkdownMessage } from '../../features/chat/components/markdown-message';
import { ResearchComposer } from '../../features/chat/components/research-composer';
import {
  getMockFollowUpResponse,
  getMockResponse,
} from '../../features/chat/mock-response';
import {
  createChatId,
  parseResearchMode,
  type ChatMessage,
  type ResearchMode,
} from '../../features/chat/types';
import { useI18n, type Locale } from '../../shared/i18n/i18n-context';

interface ChatRouteSearch {
  q?: unknown;
  query?: unknown;
  question?: unknown;
  mode?: unknown;
}

type ChatScreen = 'conversation' | 'new';

const defaultQuestions: Record<Locale, string> = {
  ru: 'Какие из новых есть самые производительные web search / deep research подходы?',
  en: 'Which recent web search and deep research approaches perform best?',
};

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
    thinkingDone: 'Размышление завершено',
    thinkingDuration: '42 сек',
    thinkingDetails:
      'Пользователь спрашивает о самых производительных подходах к web search и deep research. Стоит разделить ответ на две парадигмы — тренировочные и инференс-оркестрационные — и подкрепить цифрами с ключевых бенчмарков: BrowseComp, GAIA, DeepResearch Bench…',
    preparing: 'Готовлю ответ…',
    inputLabel: 'Сообщение',
    modeLabel: 'Режим исследования',
    suggestions: [
      'Что интересного вышло за последнее время?',
      'Собери обзор по свежим методам RLHF',
      'Какие статьи стоит прочитать по агентам?',
    ],
    olderChats: [
      'Multilingual Steering Design',
      'URL Content',
      'Llama in English? Llama in…',
      'ArXiv PDF summary',
      'Эмбединги текстов как токены',
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
    thinkingDone: 'Thinking complete',
    thinkingDuration: '42 sec',
    thinkingDetails:
      'The question asks about the strongest web search and deep research approaches. The answer should separate training-based systems from inference orchestration and compare them on BrowseComp, GAIA, and DeepResearch Bench.',
    preparing: 'Preparing an answer…',
    inputLabel: 'Message',
    modeLabel: 'Research mode',
    suggestions: [
      'What interesting work was published recently?',
      'Review the latest RLHF methods',
      'Which papers about agents should I read?',
    ],
    olderChats: [
      'Multilingual Steering Design',
      'URL Content',
      'Llama in English? Llama in…',
      'ArXiv PDF summary',
      'Text embeddings as tokens',
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
    thinkingDone: string;
    thinkingDuration: string;
    thinkingDetails: string;
    preparing: string;
    inputLabel: string;
    modeLabel: string;
    suggestions: string[];
    olderChats: string[];
  }
>;

function readInitialQuestion(search: ChatRouteSearch, locale: Locale) {
  const candidate = [search.q, search.query, search.question].find(
    (value): value is string => typeof value === 'string' && Boolean(value.trim()),
  );

  return candidate?.trim() ?? defaultQuestions[locale];
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

function createInitialMessages(
  question: string,
  locale: Locale,
  mode: ResearchMode,
): ChatMessage[] {
  return [
    {
      id: 'initial-user',
      role: 'user',
      content: question,
    },
    {
      id: 'initial-assistant',
      role: 'assistant',
      content: getMockResponse(locale, mode),
    },
  ];
}

function createMessageId(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function ThinkingStatus({
  locale,
  isOpen,
  onToggle,
}: {
  locale: Locale;
  isOpen: boolean;
  onToggle: () => void;
}) {
  const copy = chatCopy[locale];
  const CollapseIcon = isOpen ? ChevronUp : ChevronDown;

  return (
    <section className="chat-thinking" aria-label={copy.thinkingDone}>
      <button
        className="chat-thinking__toggle"
        type="button"
        aria-expanded={isOpen}
        onClick={onToggle}
      >
        <CheckCircle2 aria-hidden="true" size={17} strokeWidth={2} />
        <span className="chat-thinking__title">{copy.thinkingDone}</span>
        <span className="chat-thinking__duration">{copy.thinkingDuration}</span>
        <span className="chat-thinking__spacer" />
        <CollapseIcon
          className="chat-thinking__chevron"
          aria-hidden="true"
          size={16}
          strokeWidth={2}
        />
      </button>

      {isOpen ? (
        <div className="chat-thinking__details">{copy.thinkingDetails}</div>
      ) : null}
    </section>
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
        <span>{chatCopy[locale].preparing}</span>
      </div>
    );
  }

  if (message.role === 'user') {
    return (
      <article className="chat-message chat-message--user">
        <p>{message.content}</p>
      </article>
    );
  }

  return (
    <article
      className="chat-message chat-message--assistant"
      aria-live="polite"
    >
      <MarkdownMessage content={message.content} />
    </article>
  );
}

export function ChatPage() {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const { chatId } = useParams({ strict: false }) as { chatId?: string };
  const routeSearch = useSearch({ strict: false }) as ChatRouteSearch;
  const initialQuestion = readInitialQuestion(routeSearch, locale);
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
  const [messages, setMessages] = useState<ChatMessage[]>(() =>
    createInitialMessages(initialQuestion, locale, routeMode),
  );
  const [draft, setDraft] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [isThinkingOpen, setIsThinkingOpen] = useState(false);
  const previousRouteConversationKeyRef = useRef(routeConversationKey);
  const pendingTimerRef = useRef<number | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const threadEndRef = useRef<HTMLDivElement | null>(null);

  const title = useMemo(
    () => conversationTitle(activeQuestion, locale),
    [activeQuestion, locale],
  );

  const cancelPendingResponse = () => {
    if (pendingTimerRef.current !== null) {
      window.clearTimeout(pendingTimerRef.current);
      pendingTimerRef.current = null;
      setMessages((current) =>
        current.map((message) =>
          message.pending
            ? {
                ...message,
                content: getMockFollowUpResponse(locale, composerMode),
                pending: false,
              }
            : message,
        ),
      );
    }
    setIsSending(false);
  };

  useEffect(() => {
    if (previousRouteConversationKeyRef.current === routeConversationKey) {
      return;
    }

    previousRouteConversationKeyRef.current = routeConversationKey;
    if (pendingTimerRef.current !== null) {
      window.clearTimeout(pendingTimerRef.current);
      pendingTimerRef.current = null;
    }

    if (chatId === 'new') {
      setDraft('');
      setComposerMode('web');
      setIsSending(false);
      setIsThinkingOpen(false);
      setScreen('new');
      return;
    }

    setActiveQuestion(initialQuestion);
    setComposerMode(routeMode);
    setMessages(createInitialMessages(initialQuestion, locale, routeMode));
    setDraft('');
    setIsSending(false);
    setIsThinkingOpen(false);
    setScreen('conversation');
  }, [chatId, initialQuestion, locale, routeConversationKey, routeMode]);

  useEffect(
    () => () => {
      if (pendingTimerRef.current !== null) {
        window.clearTimeout(pendingTimerRef.current);
      }
    },
    [],
  );

  useEffect(() => {
    if (messages.length <= 2) {
      return;
    }
    threadEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [messages]);

  const openNewChat = () => {
    cancelPendingResponse();
    setDraft('');
    setIsThinkingOpen(false);
    setScreen('new');
    if (window.matchMedia('(max-width: 640px)').matches) {
      setIsHistoryOpen(false);
    }
    window.requestAnimationFrame(() => composerRef.current?.focus());
  };

  const openCurrentConversation = () => {
    setScreen('conversation');
    if (window.matchMedia('(max-width: 640px)').matches) {
      setIsHistoryOpen(false);
    }
  };

  const startNewConversation = (question: string) => {
    const nextChatId = createChatId();
    const nextMode = composerMode;

    setActiveQuestion(question);
    setMessages(createInitialMessages(question, locale, nextMode));
    setDraft('');
    setIsThinkingOpen(false);
    setScreen('conversation');

    void navigate({
      to: '/chat/$chatId',
      params: { chatId: nextChatId },
      search: { q: question, mode: nextMode },
    });
  };

  const sendFollowUp = (content: string) => {
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
    setIsSending(true);

    pendingTimerRef.current = window.setTimeout(() => {
      setMessages((current) =>
        current.map((message) =>
          message.id === assistantId
            ? {
                ...message,
                content: getMockFollowUpResponse(locale, responseMode),
                pending: false,
              }
            : message,
        ),
      );
      setIsSending(false);
      pendingTimerRef.current = null;
    }, 720);
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
              <button
                className={
                  screen === 'conversation'
                    ? 'chat-history__item chat-history__item--active'
                    : 'chat-history__item'
                }
                type="button"
                aria-current={screen === 'conversation' ? 'page' : undefined}
                onClick={openCurrentConversation}
                title={title}
              >
                <span>{title}</span>
              </button>
            </section>

            <section className="chat-history__group">
              <h2>{copy.earlier}</h2>
              <div className="chat-history__items">
                {copy.olderChats.map((item) => (
                  <button
                    className="chat-history__item"
                    type="button"
                    key={item}
                    title={item}
                  >
                    <span>{item}</span>
                  </button>
                ))}
              </div>
            </section>
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
              {messages.map((message, index) => (
                <Fragment key={message.id}>
                  {index === 1 ? (
                    <ThinkingStatus
                      locale={locale}
                      isOpen={isThinkingOpen}
                      onToggle={() => setIsThinkingOpen((current) => !current)}
                    />
                  ) : null}
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
