import { features } from '../../shared/config/features';
import { useAuthenticated } from '../../features/auth/token-storage';
import { queryClient } from '../query-client';
import { Link, useLocation, useNavigate } from '@tanstack/react-router';
import {
  BookMarked,
  Globe,
  LogOut,
  LogIn,
  MessageSquare,
  MonitorSmartphone,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  Sun,
  Telescope,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useState } from 'react';

import { logout } from '../../features/auth/auth-api';
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
  isLibraryActive: boolean;
  searchLabel: string;
  assistantLabel: string;
}

interface SidebarFooterProps {
  isCollapsed: boolean;
  settingsLabel: string;
  feedbackLabel: string;
  themeLabel: string;
  authenticated: boolean;
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
  const authenticated = useAuthenticated();
  const pathname = useLocation({ select: (location) => location.pathname });
  const [isCollapsed, setIsCollapsed] = useState(defaultCollapsed);

  const nextTheme = theme === 'light' ? 'dark' : 'light';
  const ThemeIcon = theme === 'light' ? Moon : Sun;
  const CollapseIcon = isCollapsed ? PanelLeftOpen : PanelLeftClose;
  const themeLabel = theme === 'light' ? t('nav.darkMode') : t('nav.lightMode');

  const handleLogout = () => {
    // Отзываем серверную сессию (cookie) и чистим access-токен в памяти.
    void logout().finally(() => {
      queryClient.clear();
      void navigate({ to: '/' });
    });
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
        <Link to="/" className="sidebar__logo-link" aria-label="Odyssey — главная">
          <LogoMark compact={isCollapsed} />
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
        isLibraryActive={pathname.startsWith('/library') || pathname.startsWith('/reader')}
        searchLabel={t('nav.search')}
        assistantLabel={t('nav.assistant')}
      />

      <SidebarFooter
        isCollapsed={isCollapsed}
        settingsLabel={t('nav.settings')}
        feedbackLabel={t('nav.feedback')}
        themeLabel={themeLabel}
        authenticated={authenticated}
        logoutLabel={authenticated ? 'Выйти' : 'Войти'}
        ThemeIcon={ThemeIcon}
        onOpenSettings={onOpenSettings}
        onToggleTheme={() => onThemeChange(nextTheme)}
        onLogout={authenticated ? handleLogout : () => { void navigate({ to: '/login' }); }}
      />
    </aside>
  );
}

function SidebarSessionsLink({ isCollapsed }: { isCollapsed: boolean }) {
  return (
    <Link
      to="/settings/sessions"
      className="sidebar__nav-button"
      title="Активные сессии"
      aria-label="Активные сессии"
      activeProps={{ className: 'sidebar__nav-button sidebar__nav-button--active' }}
    >
      <MonitorSmartphone aria-hidden="true" size={18} strokeWidth={2} />
      {!isCollapsed ? <span>Активные сессии</span> : null}
    </Link>
  );
}

function SidebarPrimaryNav({
  isCollapsed,
  isExploreCurrent,
  isAssistantActive,
  isLibraryActive,
  searchLabel,
  assistantLabel,
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
        className={`sidebar__nav-button${isLibraryActive ? ' sidebar__nav-button--active' : ''}`}
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
  authenticated,
  logoutLabel,
  ThemeIcon,
  onOpenSettings,
  onToggleTheme,
  onLogout,
}: SidebarFooterProps) {
  return (
    <div className="sidebar__footer">
      {features.activeSessions && authenticated ? <SidebarSessionsLink isCollapsed={isCollapsed} /> : null}

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
        {authenticated ? <LogOut aria-hidden="true" size={18} strokeWidth={2} /> : <LogIn aria-hidden="true" size={18} strokeWidth={2} />}
        {!isCollapsed ? <span>{logoutLabel}</span> : null}
      </button>
    </div>
  );
}
