import { useState } from 'react';
import { ArrowUp, Globe, Paperclip, Telescope } from 'lucide-react';

import { useI18n } from '../../../shared/i18n/i18n-context';
import { IconButton } from '../../../shared/ui/icon-button';
import { SegmentedControl } from '../../../shared/ui/segmented-control';

type ResearchMode = 'web' | 'deep';

export function AskAiBox() {
  const { t } = useI18n();
  const [question, setQuestion] = useState('');
  const [mode, setMode] = useState<ResearchMode>('web');

  return (
    <section className="ask-section" aria-labelledby="ask-ai-title">
      <div className="ask-box__header">
        <h1 id="ask-ai-title">{t('ask.title')}</h1>
      </div>

      <form
        className="ask-box"
        onSubmit={(event) => {
          event.preventDefault();
        }}
      >
        <textarea
          className="ask-box__input"
          value={question}
          onChange={(event) => setQuestion(event.target.value)}
          placeholder={t('ask.placeholder')}
          rows={3}
        />

        <div className="ask-box__footer">
          <IconButton icon={Paperclip} label={t('ask.attach')} />
          <SegmentedControl
            ariaLabel="Research mode"
            value={mode}
            onChange={setMode}
            options={[
              { value: 'web', label: t('ask.webSearch'), icon: Globe },
              { value: 'deep', label: t('ask.deepResearch'), icon: Telescope },
            ]}
          />
          <div className="ask-box__spacer" />
          <span className="ask-box__hint">{t('ask.sendHint')}</span>
          <IconButton icon={ArrowUp} label={t('ask.send')} variant="send" type="submit" />
        </div>
      </form>
    </section>
  );
}
