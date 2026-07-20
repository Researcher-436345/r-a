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
  | 'nav.collapse'
  | 'nav.expand'
  | 'nav.darkMode'
  | 'nav.lightMode'
  | 'projects.multimodal'
  | 'projects.reasoning'
  | 'ask.title'
  | 'ask.placeholder'
  | 'ask.send'
  | 'ask.attach'
  | 'ask.webSearch'
  | 'ask.deepResearch'
  | 'ask.sendHint'
  | 'papers.trending'
  | 'papers.trendingSub'
  | 'papers.personalize'
  | 'papers.loading'
  | 'papers.error'
  | 'papers.wantToRead'
  | 'papers.inList'
  | 'papers.popularity'
  | 'papers.pdfPreview'
  | 'papers.github'
  | 'settings.title'
  | 'settings.close'
  | 'settings.languageLabel'
  | 'settings.languageHint'
  | 'settings.themeSetting'
  | 'settings.themeHint'
  | 'settings.readerTheme'
  | 'settings.readerThemeHint'
  | 'settings.readerThemeBoth'
  | 'settings.readerThemeUiOnly';

const translations: Record<Locale, Record<TranslationKey, string>> = {
  ru: {
    'app.logo': 'r-a',
    'nav.search': 'Исследовать',
    'nav.projects': 'Проекты',
    'nav.settings': 'Настройки',
    'nav.feedback': 'Обратная связь',
    'nav.theme': 'Тема',
    'nav.light': 'Светлая',
    'nav.dark': 'Тёмная',
    'nav.language': 'Язык',
    'nav.collapse': 'Свернуть панель',
    'nav.expand': 'Развернуть панель',
    'nav.darkMode': 'Тёмная тема',
    'nav.lightMode': 'Светлая тема',
    'projects.multimodal': 'Multimodal Papers',
    'projects.reasoning': 'Reasoning Benchmarks',
    'ask.title': 'Что исследуем сегодня?',
    'ask.placeholder': 'Спросите о статьях, идеях или направлениях исследований...',
    'ask.send': 'Отправить',
    'ask.attach': 'Прикрепить файл',
    'ask.webSearch': 'Веб-поиск',
    'ask.deepResearch': 'Глубокое исследование',
    'ask.sendHint': 'Alt + Enter',
    'papers.trending': 'Трендовые статьи',
    'papers.trendingSub': 'свежие с arXiv · cs.AI',
    'papers.personalize': 'Настроить предпочтения',
    'papers.loading': 'Загружаем статьи...',
    'papers.error': 'Не удалось загрузить статьи',
    'papers.wantToRead': 'Хочу прочитать',
    'papers.inList': 'В списке чтения',
    'papers.popularity': 'Популярность',
    'papers.pdfPreview': 'Превью PDF',
    'papers.github': 'GitHub',
    'settings.title': 'Настройки',
    'settings.close': 'Закрыть настройки',
    'settings.languageLabel': 'Язык интерфейса',
    'settings.languageHint': 'Названия статей остаются на английском',
    'settings.themeSetting': 'Тема оформления',
    'settings.themeHint': 'Светлое или тёмное оформление',
    'settings.readerTheme': 'Тема PDF-ридера',
    'settings.readerThemeHint': 'Менять вместе с интерфейсом или оставить светлую бумагу',
    'settings.readerThemeBoth': 'С интерфейсом',
    'settings.readerThemeUiOnly': 'Только UI',
  },
  en: {
    'app.logo': 'r-a',
    'nav.search': 'Explore',
    'nav.projects': 'Projects',
    'nav.settings': 'Settings',
    'nav.feedback': 'Feedback',
    'nav.theme': 'Theme',
    'nav.light': 'Light',
    'nav.dark': 'Dark',
    'nav.language': 'Language',
    'nav.collapse': 'Collapse sidebar',
    'nav.expand': 'Expand sidebar',
    'nav.darkMode': 'Dark mode',
    'nav.lightMode': 'Light mode',
    'projects.multimodal': 'Multimodal Papers',
    'projects.reasoning': 'Reasoning Benchmarks',
    'ask.title': 'What are we researching today?',
    'ask.placeholder': 'Ask about papers, ideas, or research directions...',
    'ask.send': 'Send',
    'ask.attach': 'Attach file',
    'ask.webSearch': 'Web Search',
    'ask.deepResearch': 'Deep Research',
    'ask.sendHint': 'Alt + Enter',
    'papers.trending': 'Trending papers',
    'papers.trendingSub': 'fresh from arXiv · cs.AI',
    'papers.personalize': 'Personalize preferences',
    'papers.loading': 'Loading papers',
    'papers.error': 'Could not load papers',
    'papers.wantToRead': 'Want to read',
    'papers.inList': 'In reading list',
    'papers.popularity': 'Popularity',
    'papers.pdfPreview': 'PDF Preview',
    'papers.github': 'GitHub',
    'settings.title': 'Settings',
    'settings.close': 'Close settings',
    'settings.languageLabel': 'Interface language',
    'settings.languageHint': 'Paper titles stay in English',
    'settings.themeSetting': 'Appearance',
    'settings.themeHint': 'Light or dark interface',
    'settings.readerTheme': 'PDF reader theme',
    'settings.readerThemeHint': 'Follow the UI theme or keep paper light',
    'settings.readerThemeBoth': 'With UI',
    'settings.readerThemeUiOnly': 'UI only',
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
