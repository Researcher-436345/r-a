import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

export type Locale = 'ru' | 'en';

type TranslationKey =
  | 'app.logo'
  | 'nav.search'
  | 'nav.projects'
  | 'nav.settings'
  | 'nav.feedback'
  | 'nav.theme'
  | 'nav.light'
  | 'nav.dark'
  | 'nav.language'
  | 'projects.multimodal'
  | 'projects.reasoning'
  | 'ask.title'
  | 'ask.placeholder'
  | 'ask.send'
  | 'ask.webSearch'
  | 'ask.deepResearch'
  | 'papers.trending'
  | 'papers.personalize'
  | 'papers.loading'
  | 'papers.error'
  | 'papers.wantToRead'
  | 'papers.inList'
  | 'papers.popularity'
  | 'papers.pdfPreview'
  | 'papers.github';

const translations: Record<Locale, Record<TranslationKey, string>> = {
  ru: {
    'app.logo': 'r-a',
    'nav.search': 'Поиск',
    'nav.projects': 'Проекты',
    'nav.settings': 'Настройки',
    'nav.feedback': 'Обратная связь',
    'nav.theme': 'Тема',
    'nav.light': 'Светлая',
    'nav.dark': 'Темная',
    'nav.language': 'Язык',
    'projects.multimodal': 'Multimodal Papers',
    'projects.reasoning': 'Reasoning Benchmarks',
    'ask.title': 'Спросите AI о статьях, идеях или направлениях исследования...',
    'ask.placeholder': 'Например: какие свежие работы объединяют RAG и мультимодальных агентов?',
    'ask.send': 'Отправить',
    'ask.webSearch': 'Веб-поиск',
    'ask.deepResearch': 'Глубокое исследование',
    'papers.trending': 'Трендовые статьи',
    'papers.personalize': 'Настроить предпочтения',
    'papers.loading': 'Загружаем статьи...',
    'papers.error': 'Не удалось загрузить статьи',
    'papers.wantToRead': 'Хочу прочитать',
    'papers.inList': 'В списке',
    'papers.popularity': 'Популярность',
    'papers.pdfPreview': 'PDF Preview',
    'papers.github': 'GitHub',
  },
  en: {
    'app.logo': 'r-a',
    'nav.search': 'Search',
    'nav.projects': 'Projects',
    'nav.settings': 'Settings',
    'nav.feedback': 'Feedback',
    'nav.theme': 'Theme',
    'nav.light': 'Light',
    'nav.dark': 'Dark',
    'nav.language': 'Language',
    'projects.multimodal': 'Multimodal Papers',
    'projects.reasoning': 'Reasoning Benchmarks',
    'ask.title': 'Ask AI about papers, ideas, or research directions...',
    'ask.placeholder': 'Example: which recent papers connect RAG and multimodal agents?',
    'ask.send': 'Send',
    'ask.webSearch': 'Web Search',
    'ask.deepResearch': 'Deep Research',
    'papers.trending': 'Trending papers',
    'papers.personalize': 'Personalize preferences',
    'papers.loading': 'Loading papers...',
    'papers.error': 'Could not load papers',
    'papers.wantToRead': 'Want to read',
    'papers.inList': 'In list',
    'papers.popularity': 'Popularity',
    'papers.pdfPreview': 'PDF Preview',
    'papers.github': 'GitHub',
  },
};

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>('ru');

  const t = useCallback(
    (key: TranslationKey) => translations[locale][key],
    [locale],
  );

  const value = useMemo(
    () => ({
      locale,
      setLocale,
      t,
    }),
    [locale, t],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);

  if (!context) {
    throw new Error('useI18n must be used inside I18nProvider');
  }

  return context;
}
