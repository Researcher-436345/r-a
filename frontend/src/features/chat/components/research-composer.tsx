import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { SleepingCat } from '../../../shared/ui/sleeping-cat';
import { ArrowUp, Globe, Paperclip, Telescope } from 'lucide-react';

import { IconButton } from '../../../shared/ui/icon-button';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import { useMobileLayout } from '../../../shared/lib/use-mobile-layout';
import { useI18n } from '../../../shared/i18n/i18n-context';
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
    const navigate = useNavigate();
    const isMobile = useMobileLayout();
    const { locale } = useI18n();
    const inputRef = useRef<HTMLTextAreaElement>(null);
    useImperativeHandle(ref, () => inputRef.current!, []);
    useEffect(() => {
      const input = inputRef.current;
      if (!input) return;
      input.style.height = 'auto';
      input.style.height = `${Math.min(input.scrollHeight, 160)}px`;
    }, [value]);
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
        <SleepingCat variant={className.includes('chat-composer') ? 'chat' : 'home'} />
        <textarea
          ref={inputRef}
          className="ask-box__input research-composer__input"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (
              event.key === 'Enter' &&
              !isMobile &&
              !event.shiftKey &&
              !event.nativeEvent.isComposing
            ) {
              event.preventDefault();
              submit();
            }
          }}
          placeholder={placeholder}
          aria-label={inputAriaLabel}
          rows={className.includes('chat-composer') ? 1 : 2}
          autoCapitalize="sentences"
        />

        <div className="ask-box__footer research-composer__footer">
          <IconButton icon={Paperclip} label={attachLabel} onClick={onAttach ?? (() => void navigate({ to: '/library/add', search: { folder: '' } }))} />
          <SegmentedControl
            ariaLabel={modeAriaLabel}
            value={mode}
            onChange={onModeChange}
            options={[
              { value: 'web', label: webSearchLabel, mobileLabel: locale === 'ru' ? 'Поиск' : 'Search', icon: Globe },
              { value: 'deep', label: deepResearchLabel, mobileLabel: locale === 'ru' ? 'Глубокий' : 'Deep', icon: Telescope },
            ]}
          />
          <div className="ask-box__spacer research-composer__spacer" />
          {!isMobile ? <span className="sr-only">{sendHint}</span> : null}
          <IconButton
            icon={ArrowUp}
            label={sendLabel}
            variant="send"
            type="submit"
            disabled={!canSubmit}
          />
        </div>
      </form>
    );
  },
);
