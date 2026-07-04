import type { Locale } from '../../shared/i18n/i18n-context';

export type ReaderTab = 'assistant' | 'notes' | 'similar';

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
    tabAssistant: string;
    tabNotes: string;
    tabSimilar: string;
    cardTitle: string;
    cardSub: string;
    tryHint: string;
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
    tabAssistant: 'Ассистент',
    tabNotes: 'Заметки',
    tabSimilar: 'Похожие',
    cardTitle: 'С чего начать',
    cardSub:
      'Спросите что-нибудь о статье или выделите фрагмент в тексте, чтобы задать точный вопрос.',
    tryHint: 'Попробуйте спросить: «В чём интуиция за разделом 3.2?»',
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
    tabAssistant: 'Assistant',
    tabNotes: 'Notes',
    tabSimilar: 'Similar',
    cardTitle: 'Where to start',
    cardSub:
      'Ask anything about the paper, or highlight a passage in the text to ask a precise question.',
    tryHint: 'Try asking: “What’s the intuition behind section 3.2?”',
    chatPlaceholder: 'Ask about this paper or highlight text...',
    attach: 'Attach',
    sendHint: 'Alt + Enter',
  },
};

export const readerPrompts: Record<Locale, string[]> = {
  ru: [
    'Объясни основную идею простыми словами',
    'Чем QGF отличается от backprop-through-time?',
    'Какие бенчмарки использовали в экспериментах?',
  ],
  en: [
    'Explain the core idea in simple terms',
    'How does QGF differ from backprop-through-time?',
    'Which benchmarks were used in the experiments?',
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
