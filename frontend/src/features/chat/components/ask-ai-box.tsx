import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';

import { useI18n } from '../../../shared/i18n/i18n-context';
import { createChatId, type ResearchMode } from '../types';
import { ResearchComposer } from './research-composer';

export function AskAiBox() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [question, setQuestion] = useState('');
  const [mode, setMode] = useState<ResearchMode>('web');

  const submitQuestion = () => {
    const query = question.trim();

    if (!query) {
      return;
    }

    const chatId = createChatId();
    void navigate({
      to: '/chat/$chatId',
      params: { chatId },
      search: { q: query, mode },
    });
  };

  return (
    <section className="ask-section" aria-labelledby="ask-ai-title">
      <div className="ask-box__header">
        <h1 id="ask-ai-title">{t('ask.title')}</h1>
      </div>

      <ResearchComposer
        value={question}
        mode={mode}
        placeholder={t('ask.placeholder')}
        attachLabel={t('ask.attach')}
        webSearchLabel={t('ask.webSearch')}
        deepResearchLabel={t('ask.deepResearch')}
        sendHint={t('ask.sendHint')}
        sendLabel={t('ask.send')}
        onChange={setQuestion}
        onModeChange={setMode}
        onSubmit={submitQuestion}
      />
    </section>
  );
}
