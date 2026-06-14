import { useState } from 'react';

import { useI18n } from '../../../shared/i18n/i18n-context';

type ResearchMode = 'web' | 'deep';

export function AskAiBox() {
  const { t } = useI18n();
  const [question, setQuestion] = useState('');
  const [mode, setMode] = useState<ResearchMode>('web');

  return (
    <section className="ask-box" aria-labelledby="ask-ai-title">
      <div className="ask-box__header">
        <h1 id="ask-ai-title">{t('ask.title')}</h1>
      </div>

      <form
        className="ask-box__form"
        onSubmit={(event) => {
          event.preventDefault();
        }}
      >
        <textarea
          className="ask-box__input"
          value={question}
          onChange={(event) => setQuestion(event.target.value)}
          placeholder={t('ask.placeholder')}
          rows={4}
        />

        <div className="ask-box__footer">
          <div className="segmented-control" role="group" aria-label="Research mode">
            <button
              className={
                mode === 'web'
                  ? 'segmented-control__item segmented-control__item--active'
                  : 'segmented-control__item'
              }
              type="button"
              onClick={() => setMode('web')}
            >
              {t('ask.webSearch')}
            </button>
            <button
              className={
                mode === 'deep'
                  ? 'segmented-control__item segmented-control__item--active'
                  : 'segmented-control__item'
              }
              type="button"
              onClick={() => setMode('deep')}
            >
              {t('ask.deepResearch')}
            </button>
          </div>

          <button className="primary-button" type="submit">
            {t('ask.send')}
          </button>
        </div>
      </form>
    </section>
  );
}
