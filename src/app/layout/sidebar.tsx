import { Link } from '@tanstack/react-router';
import { useState } from 'react';

import { useI18n, type Locale } from '../../shared/i18n/i18n-context';
import type { ThemeMode } from './app-layout';

interface SidebarProps {
  theme: ThemeMode;
  onThemeChange: (theme: ThemeMode) => void;
}

const projects = [
  {
    id: 'multimodal-papers',
    labelKey: 'projects.multimodal',
  },
  {
    id: 'reasoning-benchmarks',
    labelKey: 'projects.reasoning',
  },
] as const;

export function Sidebar({ theme, onThemeChange }: SidebarProps) {
  const { locale, setLocale, t } = useI18n();
  const [areProjectsOpen, setAreProjectsOpen] = useState(true);
  const [areSettingsOpen, setAreSettingsOpen] = useState(false);

  const nextTheme = theme === 'light' ? 'dark' : 'light';

  return (
    <aside className="sidebar" aria-label="Application navigation">
      <div className="sidebar__top">
        <Link to="/" className="sidebar__logo" aria-label="r-a home">
          {t('app.logo')}
        </Link>

        <nav className="sidebar__nav" aria-label="Primary navigation">
          <Link
            to="/"
            className="sidebar__link sidebar__link--active"
            activeProps={{ className: 'sidebar__link sidebar__link--active' }}
          >
            <span className="sidebar__dot" aria-hidden="true" />
            {t('nav.search')}
          </Link>

          <div className="sidebar__group">
            <button
              className="sidebar__link sidebar__link--button"
              type="button"
              onClick={() => setAreProjectsOpen((value) => !value)}
              aria-expanded={areProjectsOpen}
            >
              <span className="sidebar__chevron" aria-hidden="true">
                {areProjectsOpen ? 'v' : '>'}
              </span>
              {t('nav.projects')}
            </button>

            {areProjectsOpen ? (
              <div className="sidebar__project-list">
                {projects.map((project) => (
                  <button
                    key={project.id}
                    className="sidebar__project"
                    type="button"
                  >
                    {t(project.labelKey)}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        </nav>
      </div>

      <div className="sidebar__bottom">
        <button
          className="sidebar__utility"
          type="button"
          onClick={() => setAreSettingsOpen((value) => !value)}
          aria-expanded={areSettingsOpen}
        >
          {t('nav.settings')}
        </button>

        {areSettingsOpen ? (
          <div className="settings-panel">
            <span className="settings-panel__label">{t('nav.language')}</span>
            <div className="segmented-control" role="group" aria-label="Language">
              {(['ru', 'en'] as const).map((language) => (
                <button
                  key={language}
                  className={
                    locale === language
                      ? 'segmented-control__item segmented-control__item--active'
                      : 'segmented-control__item'
                  }
                  type="button"
                  onClick={() => setLocale(language as Locale)}
                >
                  {language.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
        ) : null}

        <button className="sidebar__utility" type="button">
          {t('nav.feedback')}
        </button>

        <button
          className="theme-toggle"
          type="button"
          onClick={() => onThemeChange(nextTheme)}
          aria-label={`${t('nav.theme')}: ${theme === 'light' ? t('nav.light') : t('nav.dark')}`}
        >
          <span>{t('nav.theme')}</span>
          <span className="theme-toggle__pill">
            {theme === 'light' ? t('nav.light') : t('nav.dark')}
          </span>
        </button>
      </div>
    </aside>
  );
}
