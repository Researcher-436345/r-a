import { Outlet, useLocation } from '@tanstack/react-router';
import { useRef, useState } from 'react';

import { useMobileViewport } from '../../shared/lib/use-mobile-layout';
import { useTheme } from '../../shared/theme/theme-context';
import { MobileNavigation } from './mobile-navigation';
import { SettingsModal } from './settings-modal';
import { Sidebar } from './sidebar';

export type { ThemeMode } from '../../shared/theme/theme-context';

export function AppLayout() {
  const pathname = useLocation({ select: (location) => location.pathname });
  const isReader = pathname === '/reader' || pathname.startsWith('/reader/');
  const isChat = pathname.startsWith('/chat/');
  const isLibrary = pathname === '/library' || pathname.startsWith('/library/');
  const isWorkspace = isReader || isChat || isLibrary;
  const { theme, setTheme } = useTheme();
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const shellRef = useRef<HTMLDivElement>(null);
  useMobileViewport(shellRef);

  return (
    <div ref={shellRef} className={isWorkspace ? 'app-shell app-shell--workspace' : 'app-shell'}>
      <Sidebar
        theme={theme}
        onThemeChange={setTheme}
        onOpenSettings={() => setIsSettingsOpen(true)}
        defaultCollapsed={isReader}
      />
      <main
        className={isWorkspace ? 'main-content main-content--workspace' : 'main-content'}
        aria-label="Main content"
      >
        <Outlet />
      </main>
      <MobileNavigation onOpenSettings={() => setIsSettingsOpen(true)} />
      <SettingsModal isOpen={isSettingsOpen} onClose={() => setIsSettingsOpen(false)} />
    </div>
  );
}
