import {
  BookOpenText,
  ChartColumn,
  FileText,
  FlaskConical,
  Lightbulb,
  LoaderCircle,
  RefreshCw,
  Target,
  TriangleAlert,
} from 'lucide-react';
import type { ComponentType } from 'react';
import type { LucideProps } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { ApiError } from '../../../shared/api/client';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { copyText } from '../../../shared/lib/clipboard';
import { MessageActions } from '../../../shared/ui/message-actions';
import { RichText } from '../../../shared/ui/rich-text';
import {
  fetchPaperSummary,
  generatePaperSummaryStream,
  type PaperSummary,
} from '../api';
import { readerStrings } from '../reader-data';
import {
  SUMMARY_CARD_KEYS,
  parseSummaryDoc,
  type SummaryCardKey,
  type SummaryDoc,
} from '../summary-doc';

/** Сколько раз опрашиваем бэкенд, пока обзор генерирует другой клиент. */
const QUEUE_POLL_ATTEMPTS = 40;
const QUEUE_POLL_DELAY_MS = 3_000;
/** Сколько ждём парсер PDF (425 Too Early), прежде чем сдаться. */
const PARSE_WAIT_ATTEMPTS = 60;
const PARSE_POLL_DELAY_MS = 5_000;

/**
 * Готовые обзоры по (paper, lang) — панель размонтируется при смене вкладки,
 * а повторный GET на каждое переключение заметен как мигание.
 */
const summaryCache = new Map<string, PaperSummary>();

/** Drops cached overviews once the paper's text changed (full text found, PDF parsed). */
export function invalidatePaperSummary(paperId: string) {
  for (const key of summaryCache.keys()) {
    if (key.startsWith(`${paperId}:`)) {
      summaryCache.delete(key);
    }
  }
}

const CARD_ICONS: Record<SummaryCardKey, ComponentType<LucideProps>> = {
  tldr: FileText,
  problem: Target,
  method: FlaskConical,
  results: ChartColumn,
  takeaways: Lightbulb,
  limitations: TriangleAlert,
};

interface ReaderSummaryPanelProps {
  paperId?: string;
  /** Клик по [стр. N] — прыжок на страницу PDF */
  onPageCite?: (page: number, quote?: string) => void;
}

type PanelState =
  | 'idle'
  | 'loading'
  | 'generating'
  | 'queued'
  | 'parsing'
  | 'ready'
  | 'error';

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(resolve, ms);
    signal.addEventListener(
      'abort',
      () => {
        window.clearTimeout(timer);
        reject(new DOMException('Aborted', 'AbortError'));
      },
      { once: true },
    );
  });
}

function isAbort(err: unknown, signal: AbortSignal): boolean {
  return signal.aborted || (err instanceof DOMException && err.name === 'AbortError');
}

export function ReaderSummaryPanel({ paperId, onPageCite }: ReaderSummaryPanelProps) {
  const { locale } = useI18n();
  const text = readerStrings[locale];
  const cacheKey = paperId ? `${paperId}:${locale}` : '';
  const [summary, setSummary] = useState<PaperSummary | null>(() =>
    cacheKey ? (summaryCache.get(cacheKey) ?? null) : null,
  );
  const [streamed, setStreamed] = useState('');
  const [state, setState] = useState<PanelState>(() => (summary ? 'ready' : 'idle'));
  const [error, setError] = useState<string | null>(null);
  const [activeCard, setActiveCard] = useState<SummaryCardKey>('tldr');
  const [copied, setCopied] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const finish = useCallback(
    (generated: PaperSummary) => {
      summaryCache.set(`${generated.paper_id}:${generated.lang}`, generated);
      setSummary(generated);
      setStreamed('');
      setState('ready');
    },
    [],
  );

  const run = useCallback(
    async (id: string, force: boolean, signal: AbortSignal) => {
      setError(null);
      setStreamed('');
      setState('generating');

      // Парсер ещё не отдал текст: бэкенд отвечает 425, повторяем POST.
      let queued = false;
      for (let attempt = 0; attempt < PARSE_WAIT_ATTEMPTS; attempt += 1) {
        try {
          const generated = await generatePaperSummaryStream(id, locale, {
            force,
            signal,
            onDelta: (delta) => {
              setState('generating');
              setStreamed((current) => current + delta);
            },
          });
          if (signal.aborted) {
            return;
          }
          finish(generated);
          return;
        } catch (err) {
          if (isAbort(err, signal)) {
            return;
          }
          if (err instanceof ApiError && err.status === 425) {
            setState('parsing');
            try {
              await sleep(PARSE_POLL_DELAY_MS, signal);
            } catch {
              return;
            }
            continue;
          }
          // 409 — обзор уже генерирует другая вкладка или соседний запрос.
          if (err instanceof ApiError && err.status === 409) {
            queued = true;
            break;
          }
          setError(err instanceof ApiError ? err.detail : text.summaryFailed);
          setState('error');
          return;
        }
      }
      if (!queued) {
        setError(text.summaryFailed);
        setState('error');
        return;
      }

      setState('queued');
      setStreamed('');
      for (let attempt = 0; attempt < QUEUE_POLL_ATTEMPTS; attempt += 1) {
        try {
          await sleep(QUEUE_POLL_DELAY_MS, signal);
          const polled = await fetchPaperSummary(id, locale);
          if (signal.aborted) {
            return;
          }
          if (polled?.status === 'ready') {
            finish(polled);
            return;
          }
          if (polled?.status === 'failed') {
            setError(polled.error_message || text.summaryFailed);
            setState('error');
            return;
          }
        } catch (err) {
          if (isAbort(err, signal)) {
            return;
          }
          setError(err instanceof ApiError ? err.detail : text.summaryFailed);
          setState('error');
          return;
        }
      }
      setError(text.summaryFailed);
      setState('error');
    },
    [finish, locale, text.summaryFailed],
  );

  useEffect(() => {
    if (!paperId) {
      setSummary(null);
      setState('idle');
      return;
    }

    const controller = new AbortController();
    abortRef.current = controller;

    const cached = summaryCache.get(`${paperId}:${locale}`);
    if (cached) {
      setSummary(cached);
      setStreamed('');
      setError(null);
      setState('ready');
      return () => {
        controller.abort();
        abortRef.current = null;
      };
    }

    const load = async () => {
      setState('loading');
      setError(null);
      setSummary(null);
      setStreamed('');
      try {
        const stored = await fetchPaperSummary(paperId, locale);
        if (controller.signal.aborted) {
          return;
        }
        if (stored?.status === 'ready' && !stored.stale) {
          finish(stored);
          return;
        }
        // Отсутствует, устарел или упал прошлый прогон — генерируем лениво, один раз.
        await run(paperId, false, controller.signal);
      } catch (err) {
        if (controller.signal.aborted) {
          return;
        }
        setError(err instanceof ApiError ? err.detail : text.summaryFailed);
        setState('error');
      }
    };

    void load();
    return () => {
      controller.abort();
      abortRef.current = null;
    };
  }, [paperId, locale, run, finish, text.summaryFailed]);

  const regenerate = () => {
    if (!paperId) {
      return;
    }
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    void run(paperId, true, controller.signal);
  };

  const isBusy =
    state === 'loading' || state === 'generating' || state === 'queued' || state === 'parsing';
  const isStreaming = state === 'generating' && streamed.length > 0;
  const body = isStreaming ? streamed : (summary?.content ?? '');
  const doc: SummaryDoc = useMemo(() => parseSummaryDoc(body), [body]);
  const cardText = doc.card[activeCard];

  const statusLabel =
    state === 'loading'
      ? text.summaryLoading
      : state === 'queued'
        ? text.summaryQueued
        : state === 'parsing'
          ? text.summaryParsing
          : text.summaryGenerating;

  const handleCopy = () => {
    const source = body.trim();
    if (!source) {
      return;
    }
    void copyText(source)
      .then(() => setCopied(true))
      .catch(() => setError(locale === 'ru' ? 'Не удалось скопировать' : 'Could not copy'));
  };

  return (
    <div className="reader-summary">
      {isBusy ? (
        <div className="reader-summary__status" role="status" aria-live="polite">
          <LoaderCircle className="reader-summary__loader" aria-hidden="true" size={15} />
          <span>{statusLabel}</span>
        </div>
      ) : null}

      {state === 'error' ? (
        <div className="reader-summary__error">
          <p>{error ?? text.summaryFailed}</p>
          <button type="button" onClick={regenerate} disabled={!paperId}>
            {text.summaryRetry}
          </button>
        </div>
      ) : null}

      {doc.title ? <h1 className="reader-summary__title">{doc.title}</h1> : null}

      {doc.structured ? (
        <>
          <section className="reader-summary__card" aria-label={text.summaryBadge}>
            <div className="reader-summary__tabs" role="tablist">
              {SUMMARY_CARD_KEYS.map((key) => {
                const Icon = CARD_ICONS[key];
                const isActive = key === activeCard;
                const isEmpty = !doc.card[key];
                return (
                  <button
                    key={key}
                    type="button"
                    role="tab"
                    aria-selected={isActive}
                    className={[
                      'reader-summary__tab',
                      isActive ? 'reader-summary__tab--active' : '',
                      isEmpty ? 'reader-summary__tab--empty' : '',
                    ]
                      .filter(Boolean)
                      .join(' ')}
                    onClick={() => setActiveCard(key)}
                  >
                    <Icon aria-hidden="true" size={14} strokeWidth={2} />
                    <span>{text.summaryTabs[key]}</span>
                  </button>
                );
              })}
            </div>
            <div className="reader-summary__card-body" role="tabpanel">
              {cardText ? (
                <RichText openInReader onPageCite={onPageCite}>{cardText}</RichText>
              ) : (
                <p className="reader-summary__card-pending">
                  {isBusy ? text.summaryPending : '—'}
                </p>
              )}
            </div>
          </section>

          {doc.deepDive ? (
            <section className="reader-summary__deep" aria-label={text.summaryDeepDive}>
              <div className="reader-summary__deep-label">
                <BookOpenText aria-hidden="true" size={15} strokeWidth={2} />
                <span>{text.summaryDeepDive}</span>
              </div>
              <RichText openInReader className="reader-summary__body" onPageCite={onPageCite}>
                {doc.deepDive}
              </RichText>
            </section>
          ) : null}
        </>
      ) : body ? (
        // Модель проигнорировала скелет — показываем как есть, без карточки.
        <RichText openInReader className="reader-summary__body" onPageCite={onPageCite}>
          {body}
        </RichText>
      ) : null}

      {state === 'ready' && body ? (
        <div className="reader-summary__actions">
          <MessageActions
            copied={copied}
            copyLabel={copied ? text.summaryCopied : text.summaryCopy}
            onCopy={handleCopy}
          />
          <button
            type="button"
            className="reader-summary__refresh"
            onClick={regenerate}
            disabled={!paperId || isBusy}
            title={text.summaryRefresh}
            aria-label={text.summaryRefresh}
          >
            <RefreshCw aria-hidden="true" size={15} strokeWidth={2} />
          </button>
        </div>
      ) : null}
    </div>
  );
}
