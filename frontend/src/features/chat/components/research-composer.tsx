import { forwardRef } from 'react';
import { ArrowUp, Globe, Paperclip, Telescope } from 'lucide-react';

import { IconButton } from '../../../shared/ui/icon-button';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import type { ResearchMode } from '../types';

interface ResearchComposerProps {
  value: string;
  mode: ResearchMode;
  placeholder: string;
  attachLabel: string;
  webSearchLabel: string;
  deepResearchLabel: string;
  sendHint: string;
  sendLabel: string;
  onChange: (value: string) => void;
  onModeChange: (mode: ResearchMode) => void;
  onSubmit: () => void;
  onAttach?: () => void;
  modeAriaLabel?: string;
  inputAriaLabel?: string;
  className?: string;
  disabled?: boolean;
}

export const ResearchComposer = forwardRef<HTMLTextAreaElement, ResearchComposerProps>(
  function ResearchComposer(
    {
      value,
      mode,
      placeholder,
      attachLabel,
      webSearchLabel,
      deepResearchLabel,
      sendHint,
      sendLabel,
      onChange,
      onModeChange,
      onSubmit,
      onAttach,
      modeAriaLabel = 'Research mode',
      inputAriaLabel = placeholder,
      className = '',
      disabled = false,
    },
    ref,
  ) {
    const canSubmit = Boolean(value.trim()) && !disabled;

    const submit = () => {
      if (canSubmit) {
        onSubmit();
      }
    };

    return (
      <form
        className={`ask-box research-composer ${className}`.trim()}
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
      >
        <textarea
          ref={ref}
          className="ask-box__input research-composer__input"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.altKey && event.key === 'Enter' && !event.nativeEvent.isComposing) {
              event.preventDefault();
              submit();
            }
          }}
          placeholder={placeholder}
          aria-label={inputAriaLabel}
          rows={2}
        />

        <div className="ask-box__footer research-composer__footer">
          <IconButton icon={Paperclip} label={attachLabel} onClick={onAttach} />
          <SegmentedControl
            ariaLabel={modeAriaLabel}
            value={mode}
            onChange={onModeChange}
            options={[
              { value: 'web', label: webSearchLabel, icon: Globe },
              { value: 'deep', label: deepResearchLabel, icon: Telescope },
            ]}
          />
          <div className="ask-box__spacer research-composer__spacer" />
          <span className="ask-box__hint research-composer__hint">{sendHint}</span>
          <IconButton
            icon={ArrowUp}
            label={sendLabel}
            variant="send"
            type="submit"
            disabled={disabled}
            aria-disabled={!canSubmit}
          />
        </div>
      </form>
    );
  },
);
