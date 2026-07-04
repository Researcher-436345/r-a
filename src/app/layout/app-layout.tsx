import { Outlet } from '@tanstack/react-router';
import { useEffect, useState } from 'react';

import { I18nProvider } from '../../shared/i18n/i18n-context';
import { SettingsModal } from './settings-modal';
import { Sidebar } from './sidebar';

export type ThemeMode = 'light' | 'dark';

export function AppLayout() {
  return (
    <I18nProvider>
      <AppLayoutContent />
    </I18nProvider>
  );
}

function AppLayoutContent() {
  const [theme, setTheme] = useState<ThemeMode>('light');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  return (
    <div className="app-shell">
      <Sidebar
        theme={theme}
        onThemeChange={setTheme}
        onOpenSettings={() => setIsSettingsOpen(true)}
      />
      <main className="main-content" aria-label="Main content">
        <Outlet />
      </main>
      <SettingsModal
        isOpen={isSettingsOpen}
        theme={theme}
        onThemeChange={setTheme}
        onClose={() => setIsSettingsOpen(false)}
      />
    </div>
  );
}
