import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

export type ThemeMode = 'light' | 'dark';

/** with-ui — тема ридера следует интерфейсу; ui-only — меняется только chrome, PDF светлый */
export type ReaderThemeScope = 'with-ui' | 'ui-only';

const THEME_KEY = 'researcher.theme';
const READER_SCOPE_KEY = 'researcher.readerThemeScope';

interface ThemeContextValue {
  theme: ThemeMode;
  setTheme: (theme: ThemeMode) => void;
  toggleTheme: () => void;
  readerThemeScope: ReaderThemeScope;
  setReaderThemeScope: (scope: ReaderThemeScope) => void;
  /** Применять тёмную «бумагу» PDF */
  readerDark: boolean;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readStoredTheme(): ThemeMode {
  try {
    return window.localStorage.getItem(THEME_KEY) === 'dark' ? 'dark' : 'light';
  } catch {
    return 'light';
  }
}

function readStoredScope(): ReaderThemeScope {
  try {
    return window.localStorage.getItem(READER_SCOPE_KEY) === 'ui-only' ? 'ui-only' : 'with-ui';
  } catch {
    return 'with-ui';
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeMode>(() => readStoredTheme());
  const [readerThemeScope, setReaderThemeScopeState] = useState<ReaderThemeScope>(() =>
    readStoredScope(),
  );

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try {
      window.localStorage.setItem(THEME_KEY, theme);
    } catch {
      // ignore
    }
  }, [theme]);

  useEffect(() => {
    try {
      window.localStorage.setItem(READER_SCOPE_KEY, readerThemeScope);
    } catch {
      // ignore
    }
    // clean up legacy per-reader toggle
    try {
      window.localStorage.removeItem('researcher.reader.darkMode');
    } catch {
      // ignore
    }
  }, [readerThemeScope]);

  const setTheme = useCallback((next: ThemeMode) => {
    setThemeState(next);
  }, []);

  const toggleTheme = useCallback(() => {
    setThemeState((current) => (current === 'light' ? 'dark' : 'light'));
  }, []);

  const setReaderThemeScope = useCallback((scope: ReaderThemeScope) => {
    setReaderThemeScopeState(scope);
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      setTheme,
      toggleTheme,
      readerThemeScope,
      setReaderThemeScope,
      readerDark: theme === 'dark' && readerThemeScope === 'with-ui',
    }),
    [theme, setTheme, toggleTheme, readerThemeScope, setReaderThemeScope],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within ThemeProvider');
  }
  return ctx;
}
