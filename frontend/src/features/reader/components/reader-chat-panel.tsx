import {
  ArrowUp,
  CornerDownRight,
  GitCompare,
  Highlighter,
  Layers,
  NotebookPen,
  Paperclip,
  Quote,
  Sparkles,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { streamAnswer } from '../../chat/api';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import {
  readerNotes,
  readerPaper,
  readerPrompts,
  readerSimilar,
  readerStrings,
  type ReaderTab,
} from '../reader-data';

const readerTabs = [
  { value: 'assistant', icon: Sparkles },
  { value: 'notes', icon: NotebookPen },
  { value: 'similar', icon: Layers },
] as const;

interface ChatTurn {
  role: 'user' | 'assistant';
  content: string;
  streaming?: boolean;
  error?: boolean;
}

export function ReaderChatPanel() {
  const { locale } = useI18n();
  const text = readerStrings[locale];
  const [activeTab, setActiveTab] = useState<ReaderTab>('assistant');
  const [chatInput, setChatInput] = useState('');
  const [messages, setMessages] = useState<ChatTurn[]>([]);
  const [busy, setBusy] = useState(false);

  // Stable chat id for this reader session.
  const chatIdRef = useRef<string>(
    typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `chat-${Date.now()}`,
  );
  const threadRef = useRef<HTMLDivElement>(null);

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
    threadRef.current?.scrollTo({ top: threadRef.current.scrollHeight });
  }, [messages]);

  const send = async () => {
    const question = chatInput.trim();
    if (!question || busy) return;

    setChatInput('');
    setBusy(true);
    setMessages((prev) => [
      ...prev,
      { role: 'user', content: question },
      { role: 'assistant', content: '', streaming: true },
    ]);

    const updateAssistant = (fn: (turn: ChatTurn) => ChatTurn) =>
      setMessages((prev) => {
        const next = [...prev];
        const last = next[next.length - 1];
        if (last && last.role === 'assistant') next[next.length - 1] = fn(last);
        return next;
      });

    await streamAnswer(
      { chatId: chatIdRef.current, articleId: readerPaper.id, content: question },
      {
        onDelta: (delta) =>
          updateAssistant((turn) => ({ ...turn, content: turn.content + delta })),
        onDone: (payload) =>
          updateAssistant((turn) => ({
            ...turn,
            content: payload.content || turn.content,
            streaming: false,
          })),
        onError: (message) =>
          updateAssistant((turn) => ({
            ...turn,
            content: `${text.errorPrefix}: ${message}`,
            streaming: false,
            error: true,
          })),
      },
    );

    setBusy(false);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void send();
    }
  };

  const hasConversation = messages.length > 0;

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

      {activeTab === 'assistant' ? (
        <>
          {hasConversation ? (
            <div className="reader-thread" ref={threadRef}>
              {messages.map((msg, i) => (
                <div
                  key={i}
                  className={`reader-msg reader-msg--${msg.role}${
                    msg.error ? ' reader-msg--error' : ''
                  }`}
                >
                  <div className="reader-msg__bubble">
                    {msg.content}
                    {msg.streaming && !msg.content ? (
                      <span className="reader-msg__thinking">{text.thinking}</span>
                    ) : null}
                    {msg.streaming && msg.content ? (
                      <span className="reader-msg__caret" aria-hidden="true" />
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="reader-assistant">
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
                      onClick={() => setChatInput(prompt)}
                    >
                      <CornerDownRight aria-hidden="true" size={15} strokeWidth={2} />
                      <span>{prompt}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="reader-assistant__hint">{text.tryHint}</div>
            </div>
          )}

          <div className="reader-chat-input-wrap">
            <div className="reader-chat-input">
              <textarea
                value={chatInput}
                rows={2}
                placeholder={text.chatPlaceholder}
                onChange={(event) => setChatInput(event.target.value)}
                onKeyDown={handleKeyDown}
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
                  disabled={busy || !chatInput.trim()}
                  onClick={() => void send()}
                >
                  <ArrowUp aria-hidden="true" size={17} strokeWidth={2} />
                </button>
              </div>
            </div>
          </div>
        </>
      ) : null}

      {activeTab === 'notes' ? (
        <div className="reader-notes">
          {readerNotes[locale].map((note) => (
            <article className="reader-note-card" key={`${note.loc}-${note.quote}`}>
              <div className="reader-note-card__quote">
                <Quote aria-hidden="true" size={15} strokeWidth={2} />
                <span>{note.quote}</span>
              </div>
              <div className="reader-note-card__body">
                <p>{note.note}</p>
                <div>{note.loc}</div>
              </div>
            </article>
          ))}
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
