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
import { useMemo, useState } from 'react';

import { useI18n } from '../../../shared/i18n/i18n-context';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import {
  readerNotes,
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

export function ReaderChatPanel() {
  const { locale } = useI18n();
  const text = readerStrings[locale];
  const [activeTab, setActiveTab] = useState<ReaderTab>('assistant');
  const [chatInput, setChatInput] = useState('');

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

          <div className="reader-chat-input-wrap">
            <div className="reader-chat-input">
              <textarea
                value={chatInput}
                rows={2}
                placeholder={text.chatPlaceholder}
                onChange={(event) => setChatInput(event.target.value)}
              />
              <div className="reader-chat-input__footer">
                <button className="reader-attach-button" type="button" title={text.attach}>
                  <Paperclip aria-hidden="true" size={16} strokeWidth={2} />
                </button>
                <div className="reader-chat-input__spacer" />
                <span>{text.sendHint}</span>
                <button className="reader-send-button" type="button" aria-label="Send">
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
