import { Link } from '@tanstack/react-router';
import {
  ArrowRight,
  BookMarked,
  BookOpenText,
  FolderTree,
  Globe,
  Highlighter,
  MessageSquareText,
  Moon,
  ScrollText,
  Sparkles,
  Sun,
  TrendingUp,
} from 'lucide-react';
import type { ComponentType } from 'react';
import type { LucideProps } from 'lucide-react';

import { useI18n, type Locale } from '../../shared/i18n/i18n-context';
import { useTheme } from '../../shared/theme/theme-context';
import { LogoMark } from '../../shared/ui/logo-mark';
import { SleepingCat } from '../../shared/ui/sleeping-cat';

import './landing.css';

interface Feature {
  icon: ComponentType<LucideProps>;
  title: string;
  text: string;
}

interface Copy {
  login: string;
  start: string;
  eyebrow: string;
  title: string;
  subtitle: string;
  heroNote: string;
  painEyebrow: string;
  painTitle: string;
  painText: string;
  painItems: { label: string; tools: string }[];
  painResult: string;
  howTitle: string;
  steps: { title: string; text: string }[];
  featuresTitle: string;
  featuresSub: string;
  features: Feature[];
  overviewTitle: string;
  overviewText: string;
  overviewTabs: string[];
  overviewBullets: string[];
  audienceTitle: string;
  audience: { title: string; text: string }[];
  whyTitle: string;
  why: { title: string; text: string }[];
  ctaTitle: string;
  ctaText: string;
  ctaButton: string;
  ctaLogin: string;
  footer: string;
  mock: {
    paperTitle: string;
    pageLabel: string;
    tabs: string[];
    tldr: string;
    cite: string;
    ask: string;
  };
}

const copy: Record<Locale, Copy> = {
  ru: {
    login: 'Войти',
    start: 'Начать бесплатно',
    eyebrow: 'Odyssey · рабочее место исследователя',
    title: 'Читайте статьи глубже, а находите быстрее',
    subtitle:
      'Odyssey собирает научные статьи в вашу библиотеку, читает их вместе с вами и отвечает по полному тексту — с цитатами, которые ведут на нужную страницу PDF.',
    heroNote: 'Бесплатно на время беты. Нужен только email.',
    painEyebrow: 'Главная боль',
    painTitle: 'Исследование рассыпается по десяткам вкладок',
    painText:
      'Поиск в одном месте, перевод в другом, PDF в третьем, заметки в четвёртом. Материалы, ссылки и выводы теряются, а путь от вопроса до ответа растягивается.',
    painItems: [
      { label: 'Поиск', tools: 'Google Scholar, arXiv' },
      { label: 'Перевод', tools: 'Google Translate, DeepL' },
      { label: 'Чтение', tools: 'PDF-просмотрщики, браузер' },
      { label: 'Заметки', tools: 'Notion, Obsidian' },
    ],
    painResult: 'В Odyssey это один интерфейс: нашли статью, открыли, поняли, сохранили вывод и вернулись к нему позже.',
    howTitle: 'Как это работает',
    steps: [
      {
        title: 'Добавьте статью',
        text: 'По ссылке arXiv, DOI или загрузкой PDF. Дубликаты отсеиваются автоматически.',
      },
      {
        title: 'Откройте ридер',
        text: 'Текст извлекается из исходников статьи, а обзор собирается ещё до того, как вы дочитали аннотацию.',
      },
      {
        title: 'Спросите ассистента',
        text: 'Выделите фрагмент или задайте вопрос — ответ придёт со ссылками на страницы.',
      },
    ],
    featuresTitle: 'Что умеет Odyssey',
    featuresSub: 'Один инструмент на весь путь: от ленты свежих работ до заметок по прочитанному.',
    features: [
      {
        icon: TrendingUp,
        title: 'Лента трендов',
        text: 'Свежие статьи arXiv по категориям с цитированиями из OpenAlex: новое, горячее и популярное.',
      },
      {
        icon: FolderTree,
        title: 'Библиотека и папки',
        text: 'Список «Хочу прочитать», папки с подпапками, статус загрузки и дедупликация по DOI и arXiv ID.',
      },
      {
        icon: Highlighter,
        title: 'Ридер PDF',
        text: 'Цветные выделения, заметки к фрагментам, мгновенный перевод выделенного и прыжки по цитатам.',
      },
      {
        icon: ScrollText,
        title: 'AI-обзор статьи',
        text: 'Резюме, проблема, метод, результаты, выводы и ограничения — и подробный разбор в стиле блога.',
      },
      {
        icon: MessageSquareText,
        title: 'Ассистент по статье',
        text: 'Отвечает по полному тексту, а не по аннотации. Каждая ссылка [стр. N] открывает нужную страницу.',
      },
      {
        icon: Globe,
        title: 'Исследовательский поиск',
        text: 'Веб-поиск по научным источникам и режим глубокого исследования с отчётом и списком работ.',
      },
    ],
    overviewTitle: 'Обзор вместо чтения аннотации',
    overviewText:
      'Для каждой статьи Odyssey собирает структурированную карточку и длинный разбор с формулами и ссылками на страницы. Обзор кэшируется и пересобирается, когда статья обновляется.',
    overviewTabs: ['Резюме', 'Проблема', 'Метод', 'Результаты', 'Выводы', 'Ограничения'],
    overviewBullets: [
      'Каждый тезис опирается на текст статьи: без выдуманных чисел и бенчмарков.',
      'Цитаты вида «стр. 4 · joint scaling law» ведут в PDF одним кликом.',
      'Обзор на русском или английском — по языку интерфейса.',
    ],
    audienceTitle: 'Для кого',
    audience: [
      { title: 'Исследователи', text: 'Следить за областью и быстро разбирать новые работы по своей теме.' },
      { title: 'ML-инженеры и data scientists', text: 'Вытаскивать метод и результаты из статьи, не читая её целиком.' },
      { title: 'Студенты и аспиранты', text: 'Входить в новую тему и собирать литературу к курсовой или диссертации.' },
      { title: 'Университетские лаборатории', text: 'Держать статьи и заметки группы в одной библиотеке.' },
    ],
    whyTitle: 'Почему Odyssey',
    why: [
      { title: 'Всё в одном месте', text: 'Поиск, библиотека, ридер, обзор, перевод и заметки — без переключений между сервисами.' },
      { title: 'Работает из России', text: 'Без VPN и зарубежных карт: сервис и модели доступны напрямую.' },
      { title: 'По полному тексту, с проверкой', text: 'Ассистент и обзор опираются на текст статьи и ссылаются на страницы, а не пересказывают аннотацию.' },
      { title: 'На русском и английском', text: 'Интерфейс, обзоры и ответы на языке, на котором вам удобно думать.' },
    ],
    ctaTitle: 'Готовы начать?',
    ctaText: 'Зарегистрируйтесь, добавьте первую статью и откройте обзор — на это уйдёт минута.',
    ctaButton: 'Создать аккаунт',
    ctaLogin: 'У меня уже есть аккаунт',
    footer: 'Odyssey — библиотека, ридер и ассистент для научных статей.',
    mock: {
      paperTitle: 'Understanding Reasoning from Pretraining to Post-Training',
      pageLabel: 'стр. 4 из 32',
      tabs: ['Резюме', 'Проблема', 'Метод', 'Результаты'],
      tldr:
        'Авторы используют шахматы как контролируемый полигон и показывают, что потери предобучения предсказывают результат RL.',
      cite: 'стр. 4 · joint scaling law',
      ask: 'Почему RL не просто «заостряет» политику?',
    },
  },
  en: {
    login: 'Sign in',
    start: 'Get started',
    eyebrow: 'Odyssey · a workspace for researchers',
    title: 'Read papers deeper, find them faster',
    subtitle:
      'Odyssey collects papers into your library, reads them with you and answers from the full text — with citations that jump to the right PDF page.',
    heroNote: 'Free during the beta. All you need is an email.',
    painEyebrow: 'The problem',
    painTitle: 'Research falls apart across dozens of tabs',
    painText:
      'Search in one place, translation in another, the PDF in a third, notes in a fourth. Materials, links and conclusions get lost, and the path from question to answer stretches out.',
    painItems: [
      { label: 'Search', tools: 'Google Scholar, arXiv' },
      { label: 'Translation', tools: 'Google Translate, DeepL' },
      { label: 'Reading', tools: 'PDF viewers, the browser' },
      { label: 'Notes', tools: 'Notion, Obsidian' },
    ],
    painResult: 'In Odyssey it is one interface: find a paper, open it, understand it, save the takeaway and come back to it later.',
    howTitle: 'How it works',
    steps: [
      {
        title: 'Add a paper',
        text: 'By arXiv link, DOI or a PDF upload. Duplicates are filtered out automatically.',
      },
      {
        title: 'Open the reader',
        text: 'The text is extracted from the paper source and the overview is ready before you finish the abstract.',
      },
      {
        title: 'Ask the assistant',
        text: 'Select a passage or ask a question — the answer comes with page references.',
      },
    ],
    featuresTitle: 'What Odyssey does',
    featuresSub: 'One tool for the whole path: from a feed of fresh work to notes on what you read.',
    features: [
      {
        icon: TrendingUp,
        title: 'Trending feed',
        text: 'Fresh arXiv papers by category with OpenAlex citation counts: new, hot and popular.',
      },
      {
        icon: FolderTree,
        title: 'Library and folders',
        text: 'A “Want to read” list, nested folders, processing status and deduplication by DOI and arXiv ID.',
      },
      {
        icon: Highlighter,
        title: 'PDF reader',
        text: 'Colored highlights, notes on passages, instant translation of a selection and citation jumps.',
      },
      {
        icon: ScrollText,
        title: 'AI paper overview',
        text: 'Summary, problem, method, results, takeaways and limitations — plus a blog-style deep dive.',
      },
      {
        icon: MessageSquareText,
        title: 'Paper assistant',
        text: 'Answers from the full text, not the abstract. Every [p. N] reference opens the exact page.',
      },
      {
        icon: Globe,
        title: 'Research search',
        text: 'Web search over scholarly sources and a deep-research mode that returns a report with a reading list.',
      },
    ],
    overviewTitle: 'An overview instead of skimming the abstract',
    overviewText:
      'For every paper Odyssey builds a structured card and a long deep dive with formulas and page references. The overview is cached and rebuilt when the paper changes.',
    overviewTabs: ['Summary', 'Problem', 'Method', 'Results', 'Takeaways', 'Limitations'],
    overviewBullets: [
      'Every claim is grounded in the paper text: no invented numbers or benchmarks.',
      'Citations like “p. 4 · joint scaling law” open the PDF in one click.',
      'Overview in Russian or English — it follows the interface language.',
    ],
    audienceTitle: 'Who it is for',
    audience: [
      { title: 'Researchers', text: 'Keep up with a field and quickly digest new work on your topic.' },
      { title: 'ML engineers and data scientists', text: 'Pull the method and results out of a paper without reading all of it.' },
      { title: 'Students and PhD candidates', text: 'Get into a new topic and collect literature for a thesis.' },
      { title: 'University labs', text: 'Keep the group’s papers and notes in one library.' },
    ],
    whyTitle: 'Why Odyssey',
    why: [
      { title: 'Everything in one place', text: 'Search, library, reader, overview, translation and notes — no switching between services.' },
      { title: 'Works from Russia', text: 'No VPN or foreign cards: the service and the models are reachable directly.' },
      { title: 'Full text, verifiable', text: 'The assistant and the overview rely on the paper text and cite pages instead of retelling the abstract.' },
      { title: 'Russian and English', text: 'Interface, overviews and answers in the language you think in.' },
    ],
    ctaTitle: 'Ready to start?',
    ctaText: 'Sign up, add your first paper and open its overview — it takes a minute.',
    ctaButton: 'Create an account',
    ctaLogin: 'I already have an account',
    footer: 'Odyssey — a library, reader and assistant for research papers.',
    mock: {
      paperTitle: 'Understanding Reasoning from Pretraining to Post-Training',
      pageLabel: 'p. 4 of 32',
      tabs: ['Summary', 'Problem', 'Method', 'Results'],
      tldr:
        'The authors use chess as a controlled testbed and show that pretraining loss predicts post-RL performance.',
      cite: 'p. 4 · joint scaling law',
      ask: 'Why does RL not simply sharpen the policy?',
    },
  },
};

/** CSS-only preview of the reader with the overview card: honest, no screenshots to go stale. */
function ReaderMock({ text }: { text: Copy['mock'] }) {
  return (
    <div className="landing-mock" aria-hidden="true">
      <div className="landing-mock__toolbar">
        <span className="landing-mock__title">{text.paperTitle}</span>
        <span className="landing-mock__page">{text.pageLabel}</span>
      </div>
      <div className="landing-mock__body">
        <div className="landing-mock__pdf">
          <div className="landing-mock__line landing-mock__line--head" />
          <div className="landing-mock__line" />
          <div className="landing-mock__line landing-mock__line--short" />
          <div className="landing-mock__line landing-mock__line--mark" />
          <div className="landing-mock__line" />
          <div className="landing-mock__line landing-mock__line--short" />
          <div className="landing-mock__formula">R(C) ≈ a + b · log C</div>
          <div className="landing-mock__line" />
          <div className="landing-mock__line landing-mock__line--short" />
          <div className="landing-mock__line" />
        </div>
        <div className="landing-mock__panel">
          <div className="landing-mock__tabs">
            {text.tabs.map((tab, index) => (
              <span
                key={tab}
                className={index === 0 ? 'landing-mock__tab landing-mock__tab--active' : 'landing-mock__tab'}
              >
                {tab}
              </span>
            ))}
          </div>
          <p className="landing-mock__tldr">
            {text.tldr} <span className="landing-mock__cite">{text.cite}</span>
          </p>
          <div className="landing-mock__ask">
            <Sparkles size={13} strokeWidth={2} aria-hidden="true" />
            <span>{text.ask}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

export function LandingPage() {
  const { locale, setLocale } = useI18n();
  const { theme, toggleTheme } = useTheme();
  const text = copy[locale];

  return (
    <div className="landing">
      <header className="landing__bar">
        <LogoMark />
        <nav className="landing__nav" aria-label="Landing navigation">
          <button
            type="button"
            className="landing__icon-button"
            onClick={() => setLocale(locale === 'ru' ? 'en' : 'ru')}
            aria-label={locale === 'ru' ? 'Switch to English' : 'Переключить на русский'}
          >
            {locale === 'ru' ? 'EN' : 'RU'}
          </button>
          <button
            type="button"
            className="landing__icon-button"
            onClick={toggleTheme}
            aria-label={theme === 'dark' ? 'Light theme' : 'Dark theme'}
          >
            {theme === 'dark' ? <Sun size={15} strokeWidth={2} /> : <Moon size={15} strokeWidth={2} />}
          </button>
          <Link to="/login" className="landing__ghost">
            {text.login}
          </Link>
          <Link to="/register" className="landing__pill">
            {text.start}
          </Link>
        </nav>
      </header>

      <main>
        <section className="landing__hero">
          <div className="landing__hero-copy">
            <span className="landing__eyebrow">{text.eyebrow}</span>
            <h1 className="landing__title">{text.title}</h1>
            <p className="landing__subtitle">{text.subtitle}</p>
            <div className="landing__actions">
              <Link to="/register" className="landing__pill landing__pill--large">
                {text.start}
                <ArrowRight size={16} strokeWidth={2} aria-hidden="true" />
              </Link>
              <Link to="/login" className="landing__ghost landing__ghost--large">
                {text.login}
              </Link>
            </div>
            <p className="landing__note">{text.heroNote}</p>
          </div>
          <ReaderMock text={text.mock} />
        </section>

        <section className="landing__section landing__pain">
          <div className="landing__pain-copy">
            <span className="landing__eyebrow">{text.painEyebrow}</span>
            <h2 className="landing__h2">{text.painTitle}</h2>
            <p>{text.painText}</p>
          </div>
          <div className="landing__pain-board" aria-hidden="true">
            <ul className="landing__pain-list">
              {text.painItems.map((item) => (
                <li key={item.label} className="landing__pain-item">
                  <span className="landing__pain-label">{item.label}</span>
                  <span className="landing__pain-tools">{item.tools}</span>
                </li>
              ))}
            </ul>
            <div className="landing__pain-arrow">
              <ArrowRight size={18} strokeWidth={2} />
            </div>
            <div className="landing__pain-result">
              <LogoMark compact />
              <p>{text.painResult}</p>
            </div>
          </div>
        </section>

        <section className="landing__section">
          <h2 className="landing__h2">{text.howTitle}</h2>
          <ol className="landing__steps">
            {text.steps.map((step, index) => (
              <li key={step.title} className="landing__step">
                <span className="landing__step-index">{index + 1}</span>
                <div>
                  <h3>{step.title}</h3>
                  <p>{step.text}</p>
                </div>
              </li>
            ))}
          </ol>
        </section>

        <section className="landing__section">
          <div className="landing__section-head">
            <h2 className="landing__h2">{text.featuresTitle}</h2>
            <p>{text.featuresSub}</p>
          </div>
          <ul className="landing__features">
            {text.features.map((feature) => {
              const Icon = feature.icon;
              return (
                <li key={feature.title} className="landing__feature">
                  <span className="landing__feature-icon">
                    <Icon size={18} strokeWidth={2} aria-hidden="true" />
                  </span>
                  <div>
                    <h3>{feature.title}</h3>
                    <p>{feature.text}</p>
                  </div>
                </li>
              );
            })}
          </ul>
        </section>

        <section className="landing__section landing__overview">
          <div className="landing__overview-copy">
            <span className="landing__eyebrow">
              <BookOpenText size={14} strokeWidth={2} aria-hidden="true" />
              AI overview
            </span>
            <h2 className="landing__h2">{text.overviewTitle}</h2>
            <p>{text.overviewText}</p>
            <ul className="landing__checklist">
              {text.overviewBullets.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
          <div className="landing__overview-card" aria-hidden="true">
            <div className="landing__overview-tabs">
              {text.overviewTabs.map((tab, index) => (
                <span
                  key={tab}
                  className={
                    index === 0
                      ? 'landing__overview-tab landing__overview-tab--active'
                      : 'landing__overview-tab'
                  }
                >
                  {tab}
                </span>
              ))}
            </div>
            <div className="landing__overview-body">
              <div className="landing-mock__line landing-mock__line--head" />
              <div className="landing-mock__line" />
              <div className="landing-mock__line" />
              <div className="landing-mock__line landing-mock__line--short" />
            </div>
            <div className="landing__overview-foot">
              <BookMarked size={13} strokeWidth={2} aria-hidden="true" />
              <span>{locale === 'ru' ? 'Разбор' : 'Deep dive'}</span>
              <span className="landing__overview-words">
                {locale === 'ru' ? '≈ 1 200 слов' : '≈ 1,200 words'}
              </span>
            </div>
          </div>
        </section>

        <section className="landing__section landing__why">
          <h2 className="landing__h2">{text.whyTitle}</h2>
          <ul className="landing__why-list">
            {text.why.map((item) => (
              <li key={item.title}>
                <h3>{item.title}</h3>
                <p>{item.text}</p>
              </li>
            ))}
          </ul>
        </section>

        <section className="landing__section">
          <h2 className="landing__h2">{text.audienceTitle}</h2>
          <ul className="landing__audience">
            {text.audience.map((item) => (
              <li key={item.title}>
                <h3>{item.title}</h3>
                <p>{item.text}</p>
              </li>
            ))}
          </ul>
        </section>

        <section className="landing__cta">
          <SleepingCat variant="home" />
          <h2 className="landing__h2">{text.ctaTitle}</h2>
          <p>{text.ctaText}</p>
          <div className="landing__actions landing__actions--center">
            <Link to="/register" className="landing__pill landing__pill--large">
              {text.ctaButton}
              <ArrowRight size={16} strokeWidth={2} aria-hidden="true" />
            </Link>
            <Link to="/login" className="landing__ghost landing__ghost--large">
              {text.ctaLogin}
            </Link>
          </div>
        </section>
      </main>

      <footer className="landing__footer">
        <span>{text.footer}</span>
        <span className="meta-divider" aria-hidden="true" />
        <span>{new Date().getFullYear()}</span>
      </footer>
    </div>
  );
}
