import { Link, useLocation, useNavigate } from '@tanstack/react-router';
import {
  BookMarked,
  CalendarDays,
  Globe,
  LogOut,
  MessageSquare,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  Sun,
  Telescope,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useState } from 'react';

import { clearTokens } from '../../features/auth/token-storage';
import { useI18n } from '../../shared/i18n/i18n-context';
import { LogoMark } from '../../shared/ui/logo-mark';
import type { ThemeMode } from '../../shared/theme/theme-context';

interface SidebarProps {
  theme: ThemeMode;
  onThemeChange: (theme: ThemeMode) => void;
  onOpenSettings: () => void;
  defaultCollapsed?: boolean;
}

interface SidebarPrimaryNavProps {
  isCollapsed: boolean;
  isExploreCurrent: boolean;
  isAssistantActive: boolean;
  isEventsActive: boolean;
  searchLabel: string;
  assistantLabel: string;
  eventsLabel: string;
}

interface SidebarFooterProps {
  isCollapsed: boolean;
  settingsLabel: string;
  feedbackLabel: string;
  themeLabel: string;
  logoutLabel: string;
  ThemeIcon: LucideIcon;
  onOpenSettings: () => void;
  onToggleTheme: () => void;
  onLogout: () => void;
}

export function Sidebar({
  theme,
  onThemeChange,
  onOpenSettings,
  defaultCollapsed = false,
}: SidebarProps) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const pathname = useLocation({ select: (location) => location.pathname });
  const [isCollapsed, setIsCollapsed] = useState(defaultCollapsed);

  const nextTheme = theme === 'light' ? 'dark' : 'light';
  const ThemeIcon = theme === 'light' ? Moon : Sun;
  const CollapseIcon = isCollapsed ? PanelLeftOpen : PanelLeftClose;
  const themeLabel = theme === 'light' ? t('nav.darkMode') : t('nav.lightMode');

  const handleLogout = () => {
    clearTokens();
    void navigate({ to: '/login' });
  };

  useEffect(() => {
    setIsCollapsed(defaultCollapsed);
  }, [defaultCollapsed]);

  return (
    <aside
      className={isCollapsed ? 'sidebar sidebar--collapsed' : 'sidebar'}
      aria-label="Application navigation"
    >
      <div className="sidebar__header">
        <Link to="/" className="sidebar__logo-link" aria-label="r-a home">
          <LogoMark />
        </Link>
        {!isCollapsed ? <div className="sidebar__header-spacer" /> : null}
        <button
          className="sidebar__icon-button"
          type="button"
          title={isCollapsed ? t('nav.expand') : t('nav.collapse')}
          aria-label={isCollapsed ? t('nav.expand') : t('nav.collapse')}
          onClick={() => setIsCollapsed((value) => !value)}
        >
          <CollapseIcon aria-hidden="true" size={18} strokeWidth={2} />
        </button>
      </div>

      <SidebarPrimaryNav
        isCollapsed={isCollapsed}
        isExploreCurrent={pathname === '/'}
        isAssistantActive={pathname.startsWith('/chat/')}
        isEventsActive={pathname === '/events'}
        searchLabel={t('nav.search')}
        assistantLabel={t('nav.assistant')}
        eventsLabel={t('nav.events')}
      />

      <SidebarFooter
        isCollapsed={isCollapsed}
        settingsLabel={t('nav.settings')}
        feedbackLabel={t('nav.feedback')}
        themeLabel={themeLabel}
        logoutLabel="Выйти"
        ThemeIcon={ThemeIcon}
        onOpenSettings={onOpenSettings}
        onToggleTheme={() => onThemeChange(nextTheme)}
        onLogout={handleLogout}
      />
    </aside>
  );
}

function SidebarPrimaryNav({
  isCollapsed,
  isExploreCurrent,
  isAssistantActive,
  isEventsActive,
  searchLabel,
  assistantLabel,
  eventsLabel,
}: SidebarPrimaryNavProps) {
  return (
    <nav className="sidebar__nav" aria-label="Primary navigation">
      <Link
        to="/"
        className={
          isExploreCurrent
            ? 'sidebar__nav-button sidebar__nav-button--active'
            : 'sidebar__nav-button'
        }
        title={searchLabel}
        aria-label={searchLabel}
        aria-current={isExploreCurrent ? 'page' : undefined}
      >
        <Globe aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{searchLabel}</span> : null}
      </Link>

      <Link
        to="/events"
        className={
          isEventsActive
            ? 'sidebar__nav-button sidebar__nav-button--active'
            : 'sidebar__nav-button'
        }
        title={eventsLabel}
        aria-label={eventsLabel}
        aria-current={isEventsActive ? 'page' : undefined}
      >
        <CalendarDays aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{eventsLabel}</span> : null}
      </Link>

      <Link
        to="/chat/$chatId"
        params={{ chatId: 'new' }}
        search={{ q: '', mode: 'web' }}
        className={
          isAssistantActive
            ? 'sidebar__nav-button sidebar__nav-button--active'
            : 'sidebar__nav-button'
        }
        title={assistantLabel}
        aria-label={assistantLabel}
        aria-current={isAssistantActive ? 'page' : undefined}
      >
        <Telescope aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{assistantLabel}</span> : null}
      </Link>

      <Link
        to="/library"
        search={{ folder: '' }}
        className="sidebar__nav-button"
        title="Библиотека"
        aria-label="Библиотека"
        activeProps={{ className: 'sidebar__nav-button sidebar__nav-button--active' }}
      >
        <BookMarked aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>Библиотека</span> : null}
      </Link>
    </nav>
  );
}

function SidebarFooter({
  isCollapsed,
  settingsLabel,
  feedbackLabel,
  themeLabel,
  logoutLabel,
  ThemeIcon,
  onOpenSettings,
  onToggleTheme,
  onLogout,
}: SidebarFooterProps) {
  return (
    <div className="sidebar__footer">
      <button
        className="sidebar__nav-button"
        type="button"
        title={settingsLabel}
        onClick={onOpenSettings}
      >
        <Settings aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{settingsLabel}</span> : null}
      </button>

      <button className="sidebar__nav-button" type="button" title={feedbackLabel}>
        <MessageSquare aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{feedbackLabel}</span> : null}
      </button>

      <button
        className="sidebar__nav-button"
        type="button"
        title={themeLabel}
        onClick={onToggleTheme}
      >
        <ThemeIcon aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{themeLabel}</span> : null}
      </button>

      <button
        className="sidebar__nav-button"
        type="button"
        title={logoutLabel}
        onClick={onLogout}
      >
        <LogOut aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{logoutLabel}</span> : null}
      </button>
    </div>
  );
}
