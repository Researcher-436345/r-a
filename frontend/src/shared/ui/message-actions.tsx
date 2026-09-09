import { Check, Copy, NotebookPen } from 'lucide-react';

interface MessageActionsProps {
  copyLabel: string;
  copied: boolean;
  onCopy: () => void;
  saveLabel?: string;
  onSave?: () => void;
  saving?: boolean;
  saved?: boolean;
  className?: string;
}

export function MessageActions({ copyLabel, copied, onCopy, saveLabel, onSave, saving, saved, className = '' }: MessageActionsProps) {
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
    </div>
  );
}
