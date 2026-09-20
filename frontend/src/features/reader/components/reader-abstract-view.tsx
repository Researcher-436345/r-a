import '../reader-abstract.css';

import { ArrowUpRight, FileSearch, Info, LoaderCircle, RotateCw } from 'lucide-react';
import { useState } from 'react';

import type { LibraryPaper } from '../../library/api';
import { ApiError } from '../../../shared/api/client';
import { useI18n, type Locale } from '../../../shared/i18n/i18n-context';
import { MetadataLine } from '../../../shared/ui/metadata-line';
import { RichText } from '../../../shared/ui/rich-text';
import { findFullText, paperHasPdfComing } from '../pdf-status';

const MAX_AUTHORS = 12;

// The overview and assistant live in the paper panel: to the right on wide
// screens, below the viewer on phones, so the copy names no side.
const strings: Record<
  Locale,
  {
    notice: string;
    noticeText: string;
    noticeFailed: string;
    find: string;
    findPdf: string;
    finding: string;
    findingPdf: string;
    notFound: string;
    notFoundPdf: string;
    updatedNoPdf: string;
    findFailed: string;
    rateLimited: string;
    openPublisher: string;
    openArxiv: string;
    newTab: string;
    abstract: string;
    noAbstract: string;
    details: string;
    moreAuthors: (count: number) => string;
    processingTitle: string;
    processingText: string;
    timeoutTitle: string;
    timeoutText: string;
    errorTitle: string;
    errorText: string;
    retry: string;
    checkAgain: string;
  }
> = {
  ru: {
    notice: 'Полного текста нет в открытом доступе — обзор и ассистент в панели статьи работают по аннотации.',
    noticeText:
      'PDF нет в открытом доступе, но полный текст загружен из открытых источников — обзор и ассистент в панели статьи работают по нему.',
    noticeFailed:
      'PDF нашёлся, но скачать его не удалось — обзор и ассистент в панели статьи работают по аннотации.',
    find: 'Найти полный текст',
    findPdf: 'Найти PDF',
    finding: 'Ищем полный текст…',
    findingPdf: 'Ищем PDF…',
    notFound: 'В открытых источниках полный текст не нашёлся. Можно попробовать позже.',
    notFoundPdf: 'PDF в открытых источниках не нашёлся. Полный текст уже загружен.',
    updatedNoPdf: 'Полный текст загружен из открытых источников — обзор и ассистент теперь работают по нему. PDF не нашёлся.',
    findFailed: 'Не удалось выполнить поиск. Попробуйте ещё раз.',
    rateLimited: 'Слишком много поисков подряд. Подождите минуту и попробуйте снова.',
    openPublisher: 'Открыть у издателя',
    openArxiv: 'Открыть на arXiv',
    newTab: 'откроется в новой вкладке',
    abstract: 'Аннотация',
    noAbstract: 'Источник не прислал аннотацию для этой статьи.',
    details: 'Подробнее',
    moreAuthors: (count) => `и ещё ${count}`,
    processingTitle: 'Скачиваем и обрабатываем PDF…',
    processingText: 'Обычно это занимает до минуты. Обзор и ассистент в панели статьи уже работают.',
    timeoutTitle: 'PDF всё ещё обрабатывается',
    timeoutText: 'Это заняло дольше обычного. Проверьте ещё раз через минуту.',
    errorTitle: 'Не удалось загрузить PDF',
    errorText: 'Сервер не отдал PDF. Попробуйте ещё раз через минуту.',
    retry: 'Попробовать снова',
    checkAgain: 'Проверить снова',
  },
  en: {
    notice: 'The full text is not in open access — the overview and assistant in the paper panel work from the abstract.',
    noticeText:
      'There is no open-access PDF, but the full text was loaded from open sources — the overview and assistant in the paper panel use it.',
    noticeFailed:
      'A PDF was found but could not be downloaded — the overview and assistant in the paper panel work from the abstract.',
    find: 'Find full text',
    findPdf: 'Find PDF',
    finding: 'Searching for full text…',
    findingPdf: 'Searching for a PDF…',
    notFound: 'No open-access full text turned up. You can try again later.',
    notFoundPdf: 'No open-access PDF turned up. The full text is already loaded.',
    updatedNoPdf: 'The full text was loaded from open sources — the overview and assistant now use it. No PDF turned up.',
    findFailed: 'The search failed. Please try again.',
    rateLimited: 'Too many searches in a row. Wait a minute and try again.',
    openPublisher: 'Open at publisher',
    openArxiv: 'Open on arXiv',
    newTab: 'opens in a new tab',
    abstract: 'Abstract',
    noAbstract: 'The source provided no abstract for this paper.',
    details: 'Details',
    moreAuthors: (count) => `and ${count} more`,
    processingTitle: 'Downloading and processing the PDF…',
    processingText: 'This usually takes under a minute. The overview and assistant in the paper panel already work.',
    timeoutTitle: 'The PDF is still processing',
    timeoutText: 'This is taking longer than usual. Check again in a minute.',
    errorTitle: 'Could not load the PDF',
    errorText: 'The server did not return the PDF. Try again in a minute.',
    retry: 'Try again',
    checkAgain: 'Check again',
  },
};

function encodePath(value: string) {
  return value.split('/').map(encodeURIComponent).join('/');
}

function doiHref(doi: string | null) {
  const bare = (doi ?? '')
    .trim()
    .replace(/^https?:\/\/(dx\.)?doi\.org\//i, '')
    .replace(/^doi:\s*/i, '');
  return bare ? `https://doi.org/${encodePath(bare)}` : null;
}

function arxivHref(arxivId: string | null) {
  const id = (arxivId ?? '').trim();
  return id ? `https://arxiv.org/abs/${encodePath(id)}` : null;
}

// Crossref abstracts arrive wrapped in JATS tags.
function cleanAbstract(abstract: string | null) {
  return (abstract ?? '')
    .replace(/<\/?jats:[^>]*>/gi, '')
    .trim();
}

/** Raw worker/server text is English and technical: tucked away, not the message itself. */
function RawDetail({ detail, locale }: { detail?: string | null; locale: Locale }) {
  if (!detail) {
    return null;
  }
  return (
    <details className="reader-abstract__details">
      <summary>{strings[locale].details}</summary>
      <p>{detail}</p>
    </details>
  );
}

function PaperDetails({ paper, locale }: { paper: LibraryPaper; locale: Locale }) {
  const text = strings[locale];
  const names = paper.authors.map((author) => author.name.trim()).filter(Boolean);
  const shown = names.slice(0, MAX_AUTHORS).join(', ');
  const authorsLine =
    names.length > MAX_AUTHORS ? `${shown} ${text.moreAuthors(names.length - MAX_AUTHORS)}` : shown;
  const meta = [paper.year ? String(paper.year) : '', paper.venue?.trim() ?? ''].filter(Boolean);

  return (
    <header>
      <h1 className="reader-abstract__title">{paper.title}</h1>
      {authorsLine ? <p className="reader-abstract__authors">{authorsLine}</p> : null}
      {meta.length > 0 ? <MetadataLine className="reader-abstract__meta" items={meta} /> : null}
      {paper.arxiv_id || paper.doi ? (
        <div className="reader-abstract__ids">
          {paper.arxiv_id ? (
            <span className="reader-abstract__chip">
              <span className="reader-abstract__chip-label">arXiv</span>
              <span className="reader-abstract__chip-value">{paper.arxiv_id}</span>
            </span>
          ) : null}
          {paper.doi ? (
            <span className="reader-abstract__chip">
              <span className="reader-abstract__chip-label">DOI</span>
              <span className="reader-abstract__chip-value">{paper.doi}</span>
            </span>
          ) : null}
        </div>
      ) : null}
    </header>
  );
}

function AbstractSection({ paper, locale }: { paper: LibraryPaper; locale: Locale }) {
  const text = strings[locale];
  const abstract = cleanAbstract(paper.abstract);
  return (
    <section className="reader-abstract__section" aria-label={text.abstract}>
      <h2 className="reader-abstract__section-label">{text.abstract}</h2>
      {abstract ? (
        <RichText className="reader-abstract__body" allowImages={false}>
          {abstract}
        </RichText>
      ) : (
        <p className="reader-abstract__empty">{text.noAbstract}</p>
      )}
    </section>
  );
}

interface ReaderAbstractViewProps {
  paper: LibraryPaper;
  /** pdf-url detail: the worker's reason when the latest download failed. */
  detail?: string | null;
  /** Refreshed paper after find-fulltext; pdfComing means the parent should resume PDF polling. */
  onPaperChange?: (paper: LibraryPaper, options: { pdfComing: boolean }) => void;
}

/** Viewer body for a paper whose PDF will not appear without user action. */
export function ReaderAbstractView({ paper, detail, onPaperChange }: ReaderAbstractViewProps) {
  const { locale } = useI18n();
  const text = strings[locale];
  const [isFinding, setIsFinding] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const publisherHref = doiHref(paper.doi);
  const externalHref = publisherHref ?? arxivHref(paper.arxiv_id);
  const externalLabel = publisherHref ? text.openPublisher : text.openArxiv;
  const hasText = Boolean(paper.has_full_text);
  const downloadFailed = paper.latest_version?.status === 'failed';
  const notice = hasText ? text.noticeText : downloadFailed ? text.noticeFailed : text.notice;

  const handleFind = async () => {
    setIsFinding(true);
    setMessage(null);
    try {
      const updated = await findFullText(paper.id);
      const pdfComing = paperHasPdfComing(updated);
      // 200 without a PDF means open full text was stored; the notice above switches with the paper.
      if (!pdfComing) {
        setMessage(text.updatedNoPdf);
      }
      onPaperChange?.(updated, { pdfComing });
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        setMessage(text.rateLimited);
      } else if (err instanceof ApiError && err.status === 422) {
        setMessage(hasText ? text.notFoundPdf : text.notFound);
      } else {
        setMessage(text.findFailed);
      }
    } finally {
      setIsFinding(false);
    }
  };

  return (
    <div className="reader-abstract">
      <article className="reader-abstract__inner">
        <PaperDetails paper={paper} locale={locale} />

        <div className="reader-abstract__notice">
          <Info aria-hidden="true" size={16} strokeWidth={2} />
          <div>
            <p>{notice}</p>
            {downloadFailed ? <RawDetail detail={detail} locale={locale} /> : null}
          </div>
        </div>

        <div className="reader-abstract__actions">
          <button
            type="button"
            className="reader-abstract__primary"
            disabled={isFinding}
            aria-busy={isFinding}
            onClick={() => void handleFind()}
          >
            {isFinding ? (
              <LoaderCircle className="spin" aria-hidden="true" size={16} strokeWidth={2} />
            ) : (
              <FileSearch aria-hidden="true" size={16} strokeWidth={2} />
            )}
            {isFinding ? (hasText ? text.findingPdf : text.finding) : hasText ? text.findPdf : text.find}
          </button>
          {externalHref ? (
            <a
              className="reader-abstract__secondary"
              href={externalHref}
              target="_blank"
              rel="noreferrer"
              title={text.newTab}
              aria-label={`${externalLabel} (${text.newTab})`}
            >
              {externalLabel}
              <ArrowUpRight aria-hidden="true" size={14} strokeWidth={2} />
            </a>
          ) : null}
        </div>
        {message ? (
          <p className="reader-abstract__message" role="status">
            {message}
          </p>
        ) : null}

        <AbstractSection paper={paper} locale={locale} />
      </article>
    </div>
  );
}

export type ReaderPdfPendingKind = 'processing' | 'timeout' | 'error';

interface ReaderPdfPendingViewProps {
  kind: ReaderPdfPendingKind;
  paper: LibraryPaper | null;
  detail?: string | null;
  onRetry?: () => void;
}

/** Viewer body while the PDF is on its way, or when waiting for it failed. */
export function ReaderPdfPendingView({ kind, paper, detail, onRetry }: ReaderPdfPendingViewProps) {
  const { locale } = useI18n();
  const text = strings[locale];
  const title =
    kind === 'processing' ? text.processingTitle : kind === 'timeout' ? text.timeoutTitle : text.errorTitle;
  const description =
    kind === 'processing' ? text.processingText : kind === 'timeout' ? text.timeoutText : text.errorText;

  return (
    <div className="reader-abstract">
      <div className="reader-abstract__inner">
        <div
          className={
            kind === 'error'
              ? 'reader-abstract__status reader-abstract__status--error'
              : 'reader-abstract__status'
          }
          role={kind === 'error' ? 'alert' : 'status'}
        >
          {kind === 'processing' ? (
            <LoaderCircle className="spin" aria-hidden="true" size={18} strokeWidth={2} />
          ) : (
            <Info aria-hidden="true" size={18} strokeWidth={2} />
          )}
          <div>
            <p className="reader-abstract__status-title">{title}</p>
            <p className="reader-abstract__status-text">{description}</p>
            {kind === 'error' ? <RawDetail detail={detail} locale={locale} /> : null}
            {kind !== 'processing' && onRetry ? (
              <div className="reader-abstract__actions">
                <button type="button" className="reader-abstract__primary" onClick={onRetry}>
                  <RotateCw aria-hidden="true" size={15} strokeWidth={2} />
                  {kind === 'timeout' ? text.checkAgain : text.retry}
                </button>
              </div>
            ) : null}
          </div>
        </div>

        {paper ? (
          <article className={kind === 'processing' ? 'reader-abstract__deferred' : undefined}>
            <PaperDetails paper={paper} locale={locale} />
            <AbstractSection paper={paper} locale={locale} />
          </article>
        ) : null}
      </div>
    </div>
  );
}
