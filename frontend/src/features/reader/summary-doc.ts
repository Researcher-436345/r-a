/**
 * Разбор markdown-обзора статьи на структуру alphaXiv:
 * заголовок → карточка (TL;DR + 5 секций из буллетов) → длинный «разбор».
 *
 * Модель отдаёт фиксированные английские `## `-маркеры (см. summaryHeadings
 * на бэкенде). Парсер терпим к самодеятельности модели — русским/иным
 * вариантам заголовков — и работает на частичном тексте во время стриминга,
 * так что карточка заполняется по мере генерации.
 */

export type SummaryCardKey =
  | 'tldr'
  | 'problem'
  | 'method'
  | 'results'
  | 'takeaways'
  | 'limitations';

export const SUMMARY_CARD_KEYS: readonly SummaryCardKey[] = [
  'tldr',
  'problem',
  'method',
  'results',
  'takeaways',
  'limitations',
];

type BucketKey = SummaryCardKey | 'deep';

export interface SummaryDoc {
  /** Заголовок статьи на языке обзора (пусто, если модель его не вернула) */
  title: string;
  card: Record<SummaryCardKey, string>;
  /** Длинный блог-разбор: markdown с `###`-подзаголовками */
  deepDive: string;
  /** Нашёлся хотя бы один известный маркер; иначе рендерим сырой markdown */
  structured: boolean;
}

const HEADING_ALIASES: Record<string, BucketKey> = {
  'tl;dr': 'tldr',
  tldr: 'tldr',
  summary: 'tldr',
  overview: 'tldr',
  'резюме': 'tldr',
  'кратко': 'tldr',
  'суть': 'tldr',
  problem: 'problem',
  'problem statement': 'problem',
  motivation: 'problem',
  'задача': 'problem',
  'проблема': 'problem',
  'постановка задачи': 'problem',
  method: 'method',
  methods: 'method',
  methodology: 'method',
  approach: 'method',
  'метод': 'method',
  'методы': 'method',
  'подход': 'method',
  results: 'results',
  'key results': 'results',
  findings: 'results',
  'результаты': 'results',
  'ключевые результаты': 'results',
  takeaways: 'takeaways',
  'key takeaways': 'takeaways',
  conclusions: 'takeaways',
  conclusion: 'takeaways',
  implications: 'takeaways',
  'выводы': 'takeaways',
  'значение': 'takeaways',
  limitations: 'limitations',
  'limitations and future work': 'limitations',
  'ограничения': 'limitations',
  'deep dive': 'deep',
  'deep-dive': 'deep',
  deepdive: 'deep',
  blog: 'deep',
  'разбор': 'deep',
  'подробный разбор': 'deep',
  'детальный разбор': 'deep',
};

function stripInlineMarkdown(value: string): string {
  return value
    .replace(/[*_`]+/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

function normalizeHeading(raw: string): string {
  return stripInlineMarkdown(raw)
    .toLowerCase()
    .replace(/[:.\s]+$/, '')
    .trim();
}

function joinBucket(lines: string[] | undefined): string {
  if (!lines || lines.length === 0) {
    return '';
  }
  return lines.join('\n').trim();
}

export function emptySummaryDoc(): SummaryDoc {
  return {
    title: '',
    card: { tldr: '', problem: '', method: '', results: '', takeaways: '', limitations: '' },
    deepDive: '',
    structured: false,
  };
}

export function parseSummaryDoc(markdown: string): SummaryDoc {
  const doc = emptySummaryDoc();
  const lines = (markdown || '').replace(/\r\n/g, '\n').split('\n');
  const buckets: Partial<Record<BucketKey, string[]>> = {};
  const seen = new Set<BucketKey>();
  let current: BucketKey | null = null;
  let inFence = false;

  for (const line of lines) {
    if (/^\s*```/.test(line)) {
      inFence = !inFence;
    }
    if (!inFence) {
      const h1 = line.match(/^#\s+(.+?)\s*$/);
      if (h1 && current === null && !doc.title) {
        doc.title = stripInlineMarkdown(h1[1]);
        continue;
      }
      const h2 = line.match(/^##\s+(.+?)\s*$/);
      if (h2) {
        const key = HEADING_ALIASES[normalizeHeading(h2[1])];
        if (key) {
          current = key;
          seen.add(key);
          buckets[key] ??= [];
          continue;
        }
        // Неизвестный `##`: внутри разбора это просто подраздел, до него —
        // модель, скорее всего, переименовала секцию; оставляем текст в текущей.
        if (current === 'deep' || seen.has('deep') || seen.size >= SUMMARY_CARD_KEYS.length) {
          current = 'deep';
          seen.add('deep');
          (buckets.deep ??= []).push(`### ${h2[1]}`);
          continue;
        }
        if (current) {
          buckets[current]?.push(`**${stripInlineMarkdown(h2[1])}**`);
          continue;
        }
      }
    }
    if (current === null) {
      // Преамбула до первого маркера — модели велено её не писать; игнорируем.
      continue;
    }
    buckets[current]?.push(line);
  }

  doc.structured = seen.size > 0;
  for (const key of SUMMARY_CARD_KEYS) {
    doc.card[key] = joinBucket(buckets[key]);
  }
  doc.deepDive = joinBucket(buckets.deep);
  return doc;
}
