import { useEffect, useRef } from 'react';
import { FileText, LayoutTemplate, Moon, Sun, X } from 'lucide-react';

import { useI18n } from '../../shared/i18n/i18n-context';
import { useTheme } from '../../shared/theme/theme-context';
import { IconButton } from '../../shared/ui/icon-button';
import { SegmentedControl } from '../../shared/ui/segmented-control';

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export function SettingsModal({ isOpen, onClose }: SettingsModalProps) {
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme, readerThemeScope, setReaderThemeScope } = useTheme();

  const panelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!isOpen) return;
    const previous = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    panel?.querySelector<HTMLButtonElement>('button')?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); onClose(); }
      if (event.key !== 'Tab' || !panel) return;
      const controls = [...panel.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input, [tabindex="0"]')];
      const first = controls[0], last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('keydown', onKeyDown); previous?.focus(); };
  }, [isOpen, onClose]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="settings-modal" role="presentation" onClick={onClose}>
      <div
        ref={panelRef}
        className="settings-modal__panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-modal-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="settings-modal__header">
          <h3 id="settings-modal-title">{t('settings.title')}</h3>
          <IconButton icon={X} label={t('settings.close')} variant="modal" onClick={onClose} />
        </div>

        <div className="settings-modal__row">
          <div className="settings-modal__copy">
            <div>{t('settings.languageLabel')}</div>
            <span>{t('settings.languageHint')}</span>
          </div>
          <SegmentedControl
            ariaLabel={t('settings.languageLabel')}
            value={locale}
            onChange={setLocale}
            options={[
              { value: 'ru', label: 'RU' },
              { value: 'en', label: 'EN' },
            ]}
          />
        </div>

        <div className="settings-modal__row">
          <div className="settings-modal__copy">
            <div>{t('settings.themeSetting')}</div>
            <span>{t('settings.themeHint')}</span>
          </div>
          <SegmentedControl
            ariaLabel={t('settings.themeSetting')}
            value={theme}
            onChange={setTheme}
            options={[
              { value: 'light', label: t('nav.light'), icon: Sun },
              { value: 'dark', label: t('nav.dark'), icon: Moon },
            ]}
          />
        </div>

        <div className="settings-modal__row">
          <div className="settings-modal__copy">
            <div>{t('settings.readerTheme')}</div>
            <span>{t('settings.readerThemeHint')}</span>
          </div>
          <SegmentedControl
            ariaLabel={t('settings.readerTheme')}
            value={readerThemeScope}
            onChange={setReaderThemeScope}
            options={[
              {
                value: 'with-ui' as const,
                label: t('settings.readerThemeBoth'),
                icon: LayoutTemplate,
              },
              {
                value: 'ui-only' as const,
                label: t('settings.readerThemeUiOnly'),
                icon: FileText,
              },
            ]}
          />
        </div>
      </div>
    </div>
  );
}
