import { Link, useNavigate, useRouter, useSearch } from '@tanstack/react-router';
import {
  ArrowLeft,
  ArrowUpRight,
  Globe,
  Link2Off,
  Loader2,
  RotateCw,
  Search,
  SearchX,
  TriangleAlert,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import { createChatId } from '../../features/chat/types';
import { cachedOpenFailure, openByUrl } from '../../features/library/api';
import { cleanPaperTitle, isPrefetchablePaperUrl } from '../../features/papers/link-kind';
import { ApiError } from '../../shared/api/client';
import { useI18n, type Locale } from '../../shared/i18n/i18n-context';

import './open-page.css';

export interface OpenSearch {
  url: string;
  title: string;
}

const MAX_URL_LENGTH = 4096;
const MAX_TITLE_LENGTH = 300;
const SLOW_HINT_MS = 5_000;
const VERY_SLOW_HINT_MS = 20_000;

function searchString(value: unknown) {
  // The router parses query values as JSON, so a numeric title arrives as a number.
  if (typeof value === 'string') {
    return value;
  }
  return typeof value === 'number' ? String(value) : '';
}

export function validateOpenSearch(search: Record<string, unknown>): OpenSearch {
  const url = searchString(search.url).trim();
  const title = searchString(search.title).replace(/\s+/g, ' ').trim();
  return {
    url: url.length > MAX_URL_LENGTH ? '' : url,
    title: title.slice(0, MAX_TITLE_LENGTH),
  };
}

function parseHttpUrl(value: string): URL | null {
  if (!value) {
    return null;
  }
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed : null;
  } catch {
    return null;
  }
}

type OpenState =
  | { kind: 'resolving' }
  | { kind: 'not_found' }
  | { kind: 'not_a_paper' }
  | { kind: 'invalid'; detail?: string }
  | { kind: 'error'; detail: string; rateLimited?: boolean };

function stateFromError(error: unknown): OpenState {
  if (error instanceof ApiError) {
    if (error.status === 422) {
      return error.code === 'not_a_paper' ? { kind: 'not_a_paper' } : { kind: 'not_found' };
    }
    if (error.status === 400) {
      return { kind: 'invalid', detail: error.detail };
    }
    return { kind: 'error', detail: error.detail, rateLimited: error.status === 429 };
  }
  return { kind: 'error', detail: error instanceof Error ? error.message : String(error) };
}

const openCopy = {
  ru: {
    back: 'Назад',
    untitled: 'Статья по ссылке',
    resolvingEyebrow: 'Открываем в Odyssey',
    resolvingBody:
      'Загружаем статью в приложение: метаданные, аннотацию и PDF, если он есть в открытом доступе.',
    slowHint: 'Ищем открытую версию в arXiv и OpenAlex…',
    verySlowHint:
      'Сайт источника отвечает медленно — проверяем открытые копии PDF. Обычно это занимает до минуты.',
    notFoundEyebrow: 'Нет в открытых источниках',
    notFoundBody:
      'Не нашли эту работу ни по ссылке, ни в arXiv и OpenAlex. Попробуйте поиск по названию — ассистент подберёт доступные версии и близкие статьи.',
    notFoundBodyNoTitle:
      'Не удалось распознать статью по этой ссылке и найти её в arXiv и OpenAlex.',
    webPageEyebrow: 'Похоже, это не статья',
    webPageBody:
      'По ссылке обычная веб-страница без данных о публикации, и в arXiv и OpenAlex такой работы нет. Можно поискать статьи по её названию или открыть страницу на сайте.',
    notAPaperEyebrow: 'Не статья',
    notAPaperTitle: 'Ссылка ведёт не на статью',
    notAPaperBody:
      'Это список, главная или служебная страница сайта, а не конкретная работа. Её можно открыть на сайте источника.',
    invalidEyebrow: 'Ссылка не распознана',
    invalidTitle: 'Эту ссылку не получается открыть',
    invalidBody: 'В Odyssey открываются публичные ссылки http и https на статьи.',
    errorEyebrow: 'Не удалось открыть',
    errorBody: 'Что-то пошло не так при загрузке статьи. Попробуйте ещё раз.',
    rateLimitedBody: 'Слишком много статей открывается подряд. Подождите минуту и попробуйте снова.',
    details: 'Подробнее',
    retry: 'Повторить',
    searchByTitle: 'Искать по названию',
    openOriginal: 'Открыть оригинал',
    openSite: 'Открыть сайт',
    newTab: 'откроется в новой вкладке',
    home: 'На главную',
  },
  en: {
    back: 'Back',
    untitled: 'Paper from link',
    resolvingEyebrow: 'Opening in Odyssey',
    resolvingBody:
      'Bringing the paper into the app: metadata, abstract and the PDF when it is openly available.',
    slowHint: 'Looking for an open version on arXiv and OpenAlex…',
    verySlowHint:
      'The source site is slow — checking open PDF copies. This usually takes up to a minute.',
    notFoundEyebrow: 'Not in open sources',
    notFoundBody:
      'We could not find this work from the link, on arXiv or on OpenAlex. Try searching by title — the assistant will look for available versions and related papers.',
    notFoundBodyNoTitle:
      'We could not recognise a paper at this link or find it on arXiv and OpenAlex.',
    webPageEyebrow: 'Probably not a paper',
    webPageBody:
      'The link is an ordinary web page with no publication data, and arXiv and OpenAlex have no such work. You can search for papers by its title or open the page on its site.',
    notAPaperEyebrow: 'Not a paper',
    notAPaperTitle: 'This link is not a paper',
    notAPaperBody:
      'It points to a listing, home or service page rather than a specific work. You can open it on the source site.',
    invalidEyebrow: 'Unrecognised link',
    invalidTitle: 'This link cannot be opened',
    invalidBody: 'Odyssey opens public http and https links to papers.',
    errorEyebrow: 'Could not open',
    errorBody: 'Something went wrong while loading the paper. Please try again.',
    rateLimitedBody: 'Too many papers opened in a row. Wait a minute and try again.',
    details: 'Details',
    retry: 'Try again',
    searchByTitle: 'Search by title',
    openOriginal: 'Open original',
    openSite: 'Open site',
    newTab: 'opens in a new tab',
    home: 'Go home',
  },
} satisfies Record<Locale, Record<string, string>>;

export function OpenPage() {
  const { locale } = useI18n();
  const copy = openCopy[locale];
  const navigate = useNavigate();
  const router = useRouter();
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const { url, title: rawTitle } = validateOpenSearch(rawSearch);
  const parsedUrl = useMemo(() => parseHttpUrl(url), [url]);
  const title = useMemo(() => cleanPaperTitle(rawTitle), [rawTitle]);
  const host = parsedUrl ? parsedUrl.hostname.replace(/^www\./i, '') : '';

  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<OpenState>(() => {
    if (!parsedUrl) {
      return { kind: 'invalid' };
    }
    // A link whose prefetch already came back empty shows that at once.
    const failure = cachedOpenFailure(url, title);
    return failure ? stateFromError(failure) : { kind: 'resolving' };
  });
  const [waitStage, setWaitStage] = useState<0 | 1 | 2>(0);

  useEffect(() => {
    if (!parsedUrl) {
      setState({ kind: 'invalid' });
      return;
    }
    // "Try again" must really ask the server again.
    const force = attempt > 0;
    if (!force) {
      const failure = cachedOpenFailure(url, title);
      if (failure) {
        setState(stateFromError(failure));
        return;
      }
    }
    let cancelled = false;
    setState({ kind: 'resolving' });
    setWaitStage(0);
    const timers = [
      window.setTimeout(() => setWaitStage(1), SLOW_HINT_MS),
      window.setTimeout(() => setWaitStage(2), VERY_SLOW_HINT_MS),
    ];
    const clearTimers = () => timers.forEach((timer) => window.clearTimeout(timer));

    // openByUrl dedupes, so a chat prefetch in flight or already done is reused.
    openByUrl(url, title, { force })
      .then((paper) => {
        if (!cancelled) {
          void navigate({
            to: '/reader/$paperId',
            params: { paperId: paper.id },
            replace: true,
          });
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setState(stateFromError(error));
        }
      })
      .finally(clearTimers);

    return () => {
      cancelled = true;
      clearTimers();
    };
  }, [attempt, navigate, parsedUrl, title, url]);

  const goBack = () => {
    if (window.history.length > 1) {
      router.history.back();
      return;
    }
    void navigate({ to: '/' });
  };

  const retry = () => setAttempt((current) => current + 1);

  const searchByTitle = () => {
    void navigate({
      to: '/chat/$chatId',
      params: { chatId: createChatId() },
      search: { q: title, mode: 'web' },
    });
  };

  const externalLink = (label: string) =>
    parsedUrl ? (
      <a
        className="open-card__external"
        href={parsedUrl.href}
        target="_blank"
        rel="noreferrer noopener"
        title={`${label} — ${copy.newTab}`}
      >
        {label}
        <ArrowUpRight className="external-link-icon" aria-hidden="true" size={14} strokeWidth={2} />
        <span className="sr-only">({copy.newTab})</span>
      </a>
    ) : null;

  const searchButton = (variant: 'primary' | 'secondary') =>
    title ? (
      <button
        className={variant === 'primary' ? 'open-card__primary' : 'open-card__secondary'}
        type="button"
        onClick={searchByTitle}
      >
        <Search aria-hidden="true" size={15} strokeWidth={2} />
        {copy.searchByTitle}
      </button>
    ) : null;

  const retryButton = (variant: 'primary' | 'secondary') => (
    <button
      className={variant === 'primary' ? 'open-card__primary' : 'open-card__secondary'}
      type="button"
      onClick={retry}
    >
      <RotateCw aria-hidden="true" size={15} strokeWidth={2} />
      {copy.retry}
    </button>
  );

  let eyebrowIcon: React.ReactNode;
  let eyebrow: string;
  let heading = title || copy.untitled;
  let body: string | null = null;
  let detail: string | null = null;
  let actions: React.ReactNode = null;

  switch (state.kind) {
    case 'resolving':
      eyebrowIcon = <Loader2 className="spin" aria-hidden="true" size={14} strokeWidth={2.2} />;
      eyebrow = copy.resolvingEyebrow;
      body = copy.resolvingBody;
      break;
    case 'not_found': {
      // Blogs, wikis and course pages: say what they are instead of "a missing paper".
      const webPage = !isPrefetchablePaperUrl(url);
      eyebrowIcon = <SearchX aria-hidden="true" size={14} strokeWidth={2.2} />;
      eyebrow = webPage ? copy.webPageEyebrow : copy.notFoundEyebrow;
      body = webPage ? copy.webPageBody : title ? copy.notFoundBody : copy.notFoundBodyNoTitle;
      actions = (
        <>
          {title ? searchButton('primary') : retryButton('primary')}
          {title ? retryButton('secondary') : null}
          {externalLink(webPage ? copy.openSite : copy.openOriginal)}
        </>
      );
      break;
    }
    case 'not_a_paper':
      eyebrowIcon = <Link2Off aria-hidden="true" size={14} strokeWidth={2.2} />;
      eyebrow = copy.notAPaperEyebrow;
      heading = copy.notAPaperTitle;
      body = copy.notAPaperBody;
      actions = (
        <>
          {searchButton('primary')}
          {externalLink(copy.openSite)}
        </>
      );
      break;
    case 'invalid':
      eyebrowIcon = <Link2Off aria-hidden="true" size={14} strokeWidth={2.2} />;
      eyebrow = copy.invalidEyebrow;
      heading = copy.invalidTitle;
      body = copy.invalidBody;
      detail = state.detail ?? null;
      actions = (
        <Link className="open-card__primary" to="/">
          {copy.home}
        </Link>
      );
      break;
    case 'error':
      eyebrowIcon = <TriangleAlert aria-hidden="true" size={14} strokeWidth={2.2} />;
      eyebrow = copy.errorEyebrow;
      body = state.rateLimited ? copy.rateLimitedBody : copy.errorBody;
      detail = state.rateLimited ? null : state.detail;
      actions = (
        <>
          {retryButton('primary')}
          {searchButton('secondary')}
          {externalLink(copy.openOriginal)}
        </>
      );
      break;
  }

  const isResolving = state.kind === 'resolving';
  const cardClass = `open-card open-card--${state.kind}${isResolving && attempt === 0 ? ' open-card--enter' : ''}`;

  return (
    <div className="open-page">
      <div className="open-page__inner">
        <button className="open-page__back" type="button" onClick={goBack}>
          <ArrowLeft aria-hidden="true" size={15} strokeWidth={2} />
          {copy.back}
        </button>

        <section className={cardClass} aria-labelledby="open-card-title" aria-busy={isResolving || undefined}>
          <p className="open-card__eyebrow" role="status" aria-live="polite">
            {eyebrowIcon}
            <span>{eyebrow}</span>
          </p>
          <h1 className="open-card__title" id="open-card-title">
            {heading}
          </h1>
          {host ? (
            <p className="open-card__meta">
              <Globe aria-hidden="true" size={13} strokeWidth={2} />
              <span className="open-card__host" title={parsedUrl?.href}>
                {host}
              </span>
            </p>
          ) : null}
          {isResolving ? <div className="open-card__progress" aria-hidden="true" /> : null}
          {body ? <p className="open-card__body">{body}</p> : null}
          {isResolving && waitStage > 0 ? (
            <p className="open-card__hint" aria-live="polite">
              <Search aria-hidden="true" size={14} strokeWidth={2} />
              <span>{waitStage === 2 ? copy.verySlowHint : copy.slowHint}</span>
            </p>
          ) : null}
          {/* Server and network text is English and technical: available, not the message. */}
          {detail ? (
            <details className="open-card__detail">
              <summary>{copy.details}</summary>
              <p>{detail}</p>
            </details>
          ) : null}
          {actions ? <div className="open-card__actions">{actions}</div> : null}
        </section>
      </div>
    </div>
  );
}
