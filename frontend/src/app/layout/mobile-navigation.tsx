import { Link, useLocation, useNavigate } from '@tanstack/react-router';
import { BookMarked, Globe, LogIn, LogOut, Menu, MonitorSmartphone, Moon, Settings, Sun, Telescope, X } from 'lucide-react';
import { useEffect, useState } from 'react';

import { logout } from '../../features/auth/auth-api';
import { loginHref } from '../../features/auth/require-auth';
import { useAuthenticated } from '../../features/auth/token-storage';
import { features } from '../../shared/config/features';
import { useI18n } from '../../shared/i18n/i18n-context';
import { useMobileLayout } from '../../shared/lib/use-mobile-layout';
import { useTheme } from '../../shared/theme/theme-context';
import { IconButton } from '../../shared/ui/icon-button';
import { LogoMark } from '../../shared/ui/logo-mark';
import { ResponsivePanel } from '../../shared/ui/responsive-panel';
import { queryClient } from '../query-client';

export function MobileNavigation({ onOpenSettings }: { onOpenSettings: () => void }) {
  const { t, locale } = useI18n();
  const { theme, setTheme } = useTheme();
  const authenticated = useAuthenticated();
  const navigate = useNavigate();
  const pathname = useLocation({ select: (location) => location.pathname });
  const isMobile = useMobileLayout();
  const [menuOpen, setMenuOpen] = useState(false);
  const libraryActive = pathname.startsWith('/library') || pathname.startsWith('/reader');
  const assistantActive = pathname.startsWith('/chat/');
  const ThemeIcon = theme === 'light' ? Moon : Sun;

  useEffect(() => setMenuOpen(false), [pathname, isMobile]);

  if (!isMobile) return null;

  const accountAction = () => {
    setMenuOpen(false);
    if (!authenticated) {
      void navigate({ to: loginHref() });
      return;
    }
    void logout().finally(() => {
      queryClient.clear();
      void navigate({ to: '/' });
    });
  };

  return (
    <>
      <nav className="mobile-nav" aria-label={locale === 'ru' ? 'Основная навигация' : 'Primary navigation'}>
        <Link to="/" className={`mobile-nav__item${pathname === '/' ? ' mobile-nav__item--active' : ''}`} aria-current={pathname === '/' ? 'page' : undefined}>
          <Globe size={18} aria-hidden="true" />
          <span>{t('nav.search')}</span>
        </Link>
        <Link to="/chat/$chatId" params={{ chatId: 'new' }} search={{ q: '', mode: 'web' }} className={`mobile-nav__item${assistantActive ? ' mobile-nav__item--active' : ''}`} aria-current={assistantActive ? 'page' : undefined}>
          <Telescope size={18} aria-hidden="true" />
          <span>{t('nav.assistant')}</span>
        </Link>
        <Link to="/library" search={{ folder: '' }} className={`mobile-nav__item${libraryActive ? ' mobile-nav__item--active' : ''}`} aria-current={libraryActive ? 'page' : undefined}>
          <BookMarked size={18} aria-hidden="true" />
          <span>{locale === 'ru' ? 'Библиотека' : 'Library'}</span>
        </Link>
        <button className="mobile-nav__item" type="button" onClick={() => setMenuOpen(true)} aria-haspopup="dialog" aria-expanded={menuOpen}>
          <Menu size={18} aria-hidden="true" />
          <span>{locale === 'ru' ? 'Ещё' : 'More'}</span>
        </button>
      </nav>
      {menuOpen ? (
        <ResponsivePanel className="mobile-menu" label={locale === 'ru' ? 'Меню приложения' : 'Application menu'} onClose={() => setMenuOpen(false)}>
          <div className="mobile-menu__header">
            <LogoMark />
            <IconButton icon={X} label={locale === 'ru' ? 'Закрыть меню' : 'Close menu'} onClick={() => setMenuOpen(false)} />
          </div>
          <button className="mobile-menu__action" type="button" onClick={() => { setMenuOpen(false); onOpenSettings(); }}>
            <Settings size={20} aria-hidden="true" /><span>{t('nav.settings')}</span>
          </button>
          <button className="mobile-menu__action" type="button" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}>
            <ThemeIcon size={20} aria-hidden="true" /><span>{theme === 'light' ? t('nav.darkMode') : t('nav.lightMode')}</span>
          </button>
          {features.activeSessions && authenticated ? (
            <Link className="mobile-menu__action" to="/settings/sessions" onClick={() => setMenuOpen(false)}>
              <MonitorSmartphone size={20} aria-hidden="true" /><span>{locale === 'ru' ? 'Активные сессии' : 'Active sessions'}</span>
            </Link>
          ) : null}
          <button className="mobile-menu__action" type="button" onClick={accountAction}>
            {authenticated ? <LogOut size={20} aria-hidden="true" /> : <LogIn size={20} aria-hidden="true" />}
            <span>{authenticated ? (locale === 'ru' ? 'Выйти' : 'Sign out') : (locale === 'ru' ? 'Войти' : 'Sign in')}</span>
          </button>
        </ResponsivePanel>
      ) : null}
    </>
  );
}
