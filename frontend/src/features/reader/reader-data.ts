import type { Locale } from '../../shared/i18n/i18n-context';

export type ReaderTab = 'summary' | 'assistant' | 'notes' | 'similar';

export const readerPaper = {
  title: 'Test-Time Gradient Guidance of Flow Policies in Reinforcement Learning',
  meta: 'Zhou et al. · arXiv:2606.11087 · 2026',
  authors:
    'Zhiyuan Zhou, Andy Peng, Charles Xu, Qiyang Li, Tobias Springenberg, Kevin Frans, Sergey Levine',
  affiliation: 'UC Berkeley  ·  Physical Intelligence',
};

export const readerStrings: Record<
  Locale,
  {
    zoomIn: string;
    zoomOut: string;
    download: string;
    bookmarkAdd: string;
    bookmarkRemove: string;
    chooseFolder: string;
    foldersLoading: string;
    tabSummary: string;
    tabAssistant: string;
    tabNotes: string;
    tabSimilar: string;
    summaryLoading: string;
    summaryGenerating: string;
    summaryQueued: string;
    summaryRefresh: string;
    summaryStale: string;
    summaryRetry: string;
    summaryFailed: string;
    summaryParsing: string;
    summaryBadge: string;
    summaryDeepDive: string;
    summaryPending: string;
    summaryCopy: string;
    summaryCopied: string;
    summaryTabs: Record<
      'tldr' | 'problem' | 'method' | 'results' | 'takeaways' | 'limitations',
      string
    >;
    cardTitle: string;
    cardSub: string;
    /** No PDF, but open-access full text is stored. */
    cardSubText: string;
    /** No PDF and no full text: the assistant reads the abstract. */
    cardSubAbstract: string;
    tryHint: string;
    tryHintNoPdf: string;
    chatPlaceholder: string;
    attach: string;
    sendHint: string;
  }
> = {
  ru: {
    zoomIn: 'Увеличить',
    zoomOut: 'Уменьшить',
    download: 'Скачать PDF',
    bookmarkAdd: 'Добавить в «Хочу прочитать»',
    bookmarkRemove: 'Убрать из «Хочу прочитать»',
    chooseFolder: 'Добавить в папку',
    foldersLoading: 'Загружаем папки…',
    tabSummary: 'Обзор',
    tabAssistant: 'Ассистент',
    tabNotes: 'Заметки',
    tabSimilar: 'Похожие',
    summaryLoading: 'Загружаем обзор…',
    summaryGenerating: 'Читаем статью и собираем обзор…',
    summaryQueued: 'Обзор уже готовится в другой вкладке — ждём…',
    summaryRefresh: 'Пересобрать обзор',
    summaryStale: 'Статья обновилась — обзор пересобирается',
    summaryRetry: 'Попробовать снова',
    summaryFailed: 'Не удалось собрать обзор',
    summaryParsing: 'Извлекаем текст статьи — обзор начнётся, как только он будет готов…',
    summaryBadge: 'AI-обзор',
    summaryDeepDive: 'Разбор',
    summaryPending: 'Эта часть ещё пишется…',
    summaryCopy: 'Копировать',
    summaryCopied: 'Скопировано',
    summaryTabs: {
      tldr: 'Резюме',
      problem: 'Проблема',
      method: 'Метод',
      results: 'Результаты',
      takeaways: 'Выводы',
      limitations: 'Ограничения',
    },
    cardTitle: 'С чего начать',
    cardSub:
      'Спросите что-нибудь о статье или выделите фрагмент в тексте, чтобы задать точный вопрос.',
    cardSubText:
      'Спросите что-нибудь о статье — ассистент отвечает по её полному тексту из открытых источников.',
    cardSubAbstract:
      'Спросите что-нибудь о статье — полного текста нет, поэтому ассистент отвечает по аннотации.',
    tryHint: 'Попробуйте спросить: «В чём интуиция за разделом 2?»',
    tryHintNoPdf: 'Попробуйте спросить: «Какую задачу решают авторы?»',
    chatPlaceholder: 'Спросите об этой статье или выделите текст...',
    attach: 'Прикрепить',
    sendHint: 'Alt + Enter',
  },
  en: {
    zoomIn: 'Zoom in',
    zoomOut: 'Zoom out',
    download: 'Download PDF',
    bookmarkAdd: 'Add to “Want to read”',
    bookmarkRemove: 'Remove from “Want to read”',
    chooseFolder: 'Add to folder',
    foldersLoading: 'Loading folders…',
    tabSummary: 'Overview',
    tabAssistant: 'Assistant',
    tabNotes: 'Notes',
    tabSimilar: 'Similar',
    summaryLoading: 'Loading the overview…',
    summaryGenerating: 'Reading the paper and drafting the overview…',
    summaryQueued: 'The overview is already being generated elsewhere — waiting…',
    summaryRefresh: 'Regenerate overview',
    summaryStale: 'The paper changed — the overview is being rebuilt',
    summaryRetry: 'Try again',
    summaryFailed: 'Could not build the overview',
    summaryParsing: 'Extracting the paper text — the overview starts as soon as it is ready…',
    summaryBadge: 'AI overview',
    summaryDeepDive: 'Deep dive',
    summaryPending: 'This part is still being written…',
    summaryCopy: 'Copy',
    summaryCopied: 'Copied',
    summaryTabs: {
      tldr: 'Summary',
      problem: 'Problem',
      method: 'Method',
      results: 'Results',
      takeaways: 'Takeaways',
      limitations: 'Limitations',
    },
    cardTitle: 'Where to start',
    cardSub:
      'Ask anything about the paper, or highlight a passage in the text to ask a precise question.',
    cardSubText:
      'Ask anything about the paper — the assistant answers from its full text from open sources.',
    cardSubAbstract:
      'Ask anything about the paper — there is no full text, so the assistant answers from the abstract.',
    tryHint: 'Try asking: “What’s the intuition behind section 2?”',
    tryHintNoPdf: 'Try asking: “What problem do the authors solve?”',
    chatPlaceholder: 'Ask about this paper or highlight text...',
    attach: 'Attach',
    sendHint: 'Alt + Enter',
  },
};

export const readerPrompts: Record<Locale, string[]> = {
  ru: [
    'Объясни основную идею простыми словами',
    'Какие основные результаты получили авторы?',
    'Какие связанные работы упоминаются в статье?',
  ],
  en: [
    'Explain the core idea in simple terms',
    'What were the authors’ main results?',
    'Which related works are discussed in the paper?',
  ],
};

export const readerNotes = {
  ru: [
    {
      quote: '...incorporating them into RL pipelines for policy improvement has proven more difficult.',
      note:
        'Ключевая мотивация: flow/diffusion-политики плохо встраиваются в RL из-за нестабильности обучения актора.',
      loc: 'стр. 1 · Введение',
    },
    {
      quote: 'QGF works by pre-training both a reference flow policy and a value function critic...',
      note:
        'Главная идея метода — разнести обучение политики (BC) и критика (TD), а оптимизацию делать на test-time.',
      loc: 'стр. 1 · Аннотация',
    },
    {
      quote: '...using the value gradient to guide the reference policy to generate higher-value actions.',
      note: 'Проверить вывод оценки градиента критика в разделе 5 — кажется, тут вся новизна.',
      loc: 'стр. 1 · Введение',
    },
  ],
  en: [
    {
      quote: '...incorporating them into RL pipelines for policy improvement has proven more difficult.',
      note: 'Key motivation: flow/diffusion policies are hard to fold into RL due to actor-training instability.',
      loc: 'p. 1 · Introduction',
    },
    {
      quote: 'QGF works by pre-training both a reference flow policy and a value function critic...',
      note: 'Core idea — decouple policy (BC) and critic (TD) training, do the optimization at test time.',
      loc: 'p. 1 · Abstract',
    },
    {
      quote: '...using the value gradient to guide the reference policy to generate higher-value actions.',
      note: 'Check the critic-gradient estimator derivation in section 5 — looks like the main novelty.',
      loc: 'p. 1 · Introduction',
    },
  ],
};

export const readerSimilar = {
  ru: [
    {
      title: 'Diffusion Policy: Visuomotor Policy Learning via Action Diffusion',
      authors: 'Chi, Feng, Du +4',
      sim: '94%',
      tag: 'flow / diffusion политики',
    },
    {
      title: 'IDQL: Implicit Q-Learning as an Actor-Critic Method',
      authors: 'Hansen-Estruch +3',
      sim: '89%',
      tag: 'offline RL критик',
    },
    {
      title: 'Consistency Models for Fast Policy Generation',
      authors: 'Song, Dhariwal',
      sim: '82%',
      tag: 'test-time sampling',
    },
    {
      title: 'Best-of-N Sampling for Reward-Guided Generation',
      authors: 'Lightman +2',
      sim: '78%',
      tag: 'test-time compute',
    },
  ],
  en: [
    {
      title: 'Diffusion Policy: Visuomotor Policy Learning via Action Diffusion',
      authors: 'Chi, Feng, Du +4',
      sim: '94%',
      tag: 'flow / diffusion policies',
    },
    {
      title: 'IDQL: Implicit Q-Learning as an Actor-Critic Method',
      authors: 'Hansen-Estruch +3',
      sim: '89%',
      tag: 'offline RL critic',
    },
    {
      title: 'Consistency Models for Fast Policy Generation',
      authors: 'Song, Dhariwal',
      sim: '82%',
      tag: 'test-time sampling',
    },
    {
      title: 'Best-of-N Sampling for Reward-Guided Generation',
      authors: 'Lightman +2',
      sim: '78%',
      tag: 'test-time compute',
    },
  ],
};
