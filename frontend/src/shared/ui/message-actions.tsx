import { Check, Copy, NotebookPen, ThumbsUp, ThumbsDown } from 'lucide-react';
import { useState } from 'react';
import { trackResultAction, type ResearchFeature } from '../lib/analytics';

interface MessageActionsProps {
  copyLabel: string;
  copied: boolean;
  onCopy: () => void;
  saveLabel?: string;
  onSave?: () => void;
  saving?: boolean;
  saved?: boolean;
  className?: string;
  analytics?: { feature: ResearchFeature; key: string; paperId?: string; resultId?: string; locale: string };
}

export function MessageActions({ copyLabel, copied, onCopy, saveLabel, onSave, saving, saved, analytics, className = '' }: MessageActionsProps) {
  const [vote, setVote] = useState<1 | -1>();
  const feedback = (value: 1 | -1) => {
    if (!analytics) return;
    setVote(value);
    trackResultAction('feedback', analytics.feature, analytics.key, analytics.paperId, analytics.resultId, value);
  };
  return (
    <div className={`message-actions ${className}`.trim()}>
      <button className="message-actions__button" type="button" onClick={onCopy} title={copyLabel} aria-label={copyLabel}>
        {copied ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}
        <span>{copyLabel}</span>
      </button>
      {onSave && <button className="message-actions__button" type="button" onClick={onSave} disabled={saving} title={saveLabel}>
        {saved ? <Check size={14} aria-hidden="true" /> : <NotebookPen size={14} aria-hidden="true" />}
        <span>{saveLabel}</span>
      </button>}
      {analytics ? <>
        <button className="message-actions__button" type="button" onClick={() => feedback(1)} aria-pressed={vote === 1} aria-label={analytics.locale === 'ru' ? 'Ответ помог' : 'Helpful'} title={analytics.locale === 'ru' ? 'Ответ помог' : 'Helpful'}><ThumbsUp size={14} aria-hidden="true" /></button>
        <button className="message-actions__button" type="button" onClick={() => feedback(-1)} aria-pressed={vote === -1} aria-label={analytics.locale === 'ru' ? 'Ответ не помог' : 'Not helpful'} title={analytics.locale === 'ru' ? 'Ответ не помог' : 'Not helpful'}><ThumbsDown size={14} aria-hidden="true" /></button>
      </> : null}
    </div>
  );
}
