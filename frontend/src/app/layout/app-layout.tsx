import { Outlet, useLocation } from '@tanstack/react-router';
import { useState } from 'react';

import { useTheme } from '../../shared/theme/theme-context';
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

  return (
    <div className={isWorkspace ? 'app-shell app-shell--workspace' : 'app-shell'}>
      <Sidebar
        theme={theme}
        onThemeChange={setTheme}
        onOpenSettings={() => setIsSettingsOpen(true)}
        defaultCollapsed={isWorkspace}
      />
      <main
        className={isWorkspace ? 'main-content main-content--workspace' : 'main-content'}
        aria-label="Main content"
      >
        <Outlet />
      </main>
      <SettingsModal isOpen={isSettingsOpen} onClose={() => setIsSettingsOpen(false)} />
    </div>
  );
}
