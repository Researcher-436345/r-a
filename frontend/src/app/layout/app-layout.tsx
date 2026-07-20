import { Outlet, useLocation } from '@tanstack/react-router';
import { useState } from 'react';

import { useTheme } from '../../shared/theme/theme-context';
import { SettingsModal } from './settings-modal';
import { readerSidebarProjects, Sidebar } from './sidebar';

export type { ThemeMode } from '../../shared/theme/theme-context';

export function AppLayout() {
  const pathname = useLocation({ select: (location) => location.pathname });
  const isReader = pathname === '/reader' || pathname.startsWith('/reader/');
  const { theme, setTheme } = useTheme();
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);

  return (
    <div className={isReader ? 'app-shell app-shell--reader' : 'app-shell'}>
      <Sidebar
        theme={theme}
        onThemeChange={setTheme}
        onOpenSettings={() => setIsSettingsOpen(true)}
        defaultCollapsed={isReader}
        defaultProjectsOpen={!isReader}
        projects={isReader ? readerSidebarProjects : undefined}
      />
      <main
        className={isReader ? 'main-content main-content--reader' : 'main-content'}
        aria-label="Main content"
      >
        <Outlet />
      </main>
      <SettingsModal isOpen={isSettingsOpen} onClose={() => setIsSettingsOpen(false)} />
    </div>
  );
}
