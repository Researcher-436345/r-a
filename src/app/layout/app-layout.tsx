import { Outlet } from '@tanstack/react-router';
import { useEffect, useState } from 'react';

import { I18nProvider } from '../../shared/i18n/i18n-context';
import { Sidebar } from './sidebar';

export type ThemeMode = 'light' | 'dark';

export function AppLayout() {
  const [theme, setTheme] = useState<ThemeMode>('light');

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  return (
    <I18nProvider>
      <div className="app-shell">
        <Sidebar theme={theme} onThemeChange={setTheme} />
        <main className="main-content" aria-label="Main content">
          <Outlet />
        </main>
      </div>
    </I18nProvider>
  );
}
