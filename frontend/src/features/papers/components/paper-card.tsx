import { useNavigate } from '@tanstack/react-router';
import { Bookmark, BookmarkCheck, ExternalLink, LoaderCircle, TrendingUp } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import {
  addByArxiv,
  patchLibraryItem,
  removeFromLibrary,
} from '../../library/api';
import { ApiError } from '../../../shared/api/client';
import { useI18n, type Locale } from '../../../shared/i18n/i18n-context';
import type { Paper } from '../types';

interface PaperCardProps {
  paper: Paper;
  /** paper.id из нашей БД, если уже в библиотеке */
  libraryPaperId?: string | null;
  onLibraryChange?: (arxivId: string, libraryPaperId: string | null) => void;
}

const months: Record<Locale, string[]> = {
  ru: ['янв', 'фев', 'мар', 'апр', 'мая', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек'],
  en: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
};

function formatDate(value: string, locale: Locale) {
  const publishedDate = new Date(value);
  return `${publishedDate.getUTCDate()} ${
    months[locale][publishedDate.getUTCMonth()]
  } ${publishedDate.getUTCFullYear()}`;
}

function formatRelativeDate(value: string, locale: Locale) {
  const publishedDate = new Date(value);
  const now = new Date();
  const diffMs = now.getTime() - publishedDate.getTime();
  const diffDays = Math.max(0, Math.floor(diffMs / 86_400_000));

  if (locale === 'ru') {
    if (diffDays === 0) {
      return 'сегодня';
    }
    if (diffDays === 1) {
      return 'вчера';
    }
    if (diffDays < 7) {
      return `${diffDays} дн. назад`;
    }
    return `${Math.floor(diffDays / 7)} нед. назад`;
  }

  if (diffDays === 0) {
    return 'today';
  }
  if (diffDays === 1) {
    return 'yesterday';
  }
  if (diffDays < 7) {
    return `${diffDays}d ago`;
  }
  return `${Math.floor(diffDays / 7)}w ago`;
}

function formatScore(value: number) {
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1).replace('.0', '')}k`;
  }
  return String(value);
}

function hashHue(input: string): number {
  let hash = 0;
  for (let i = 0; i < input.length; i += 1) {
    hash = (hash * 31 + input.charCodeAt(i)) >>> 0;
  }
  return hash % 360;
}

export function PaperCard({ paper, libraryPaperId = null, onLibraryChange }: PaperCardProps) {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const [savedPaperId, setSavedPaperId] = useState<string | null>(libraryPaperId);
  const [isOpening, setIsOpening] = useState(false);
  const [isBookmarking, setIsBookmarking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setSavedPaperId(libraryPaperId);
  }, [libraryPaperId]);

  const isSaved = Boolean(savedPaperId);
  const SaveIcon = isSaved ? BookmarkCheck : Bookmark;
  const previewHue = useMemo(() => hashHue(paper.arxivId || paper.title), [paper.arxivId, paper.title]);
  const previewYear = useMemo(() => {
    const year = new Date(paper.publishedAt).getUTCFullYear();
    return Number.isFinite(year) ? String(year) : '';
  }, [paper.publishedAt]);
  const previewSnippet = useMemo(() => {
    const text = (paper.description || paper.title).replace(/\s+/g, ' ').trim();
    return text.length > 160 ? `${text.slice(0, 157)}…` : text;
  }, [paper.description, paper.title]);

  const openInReader = async () => {
    if (isOpening) {
      return;
    }
    setIsOpening(true);
    setError(null);
    try {
      const created = await addByArxiv(paper.arxivId);
      setSavedPaperId(created.id);
      onLibraryChange?.(paper.arxivId, created.id);
      await navigate({ to: '/reader/$paperId', params: { paperId: created.id } });
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Ошибка');
    } finally {
      setIsOpening(false);
    }
  };

  const toggleReadingList = async () => {
    if (isBookmarking || isOpening) {
      return;
    }
    setIsBookmarking(true);
    setError(null);
    try {
      if (savedPaperId) {
        await removeFromLibrary(savedPaperId);
        setSavedPaperId(null);
        onLibraryChange?.(paper.arxivId, null);
      } else {
        const created = await addByArxiv(paper.arxivId);
        await patchLibraryItem(created.id, { favorite: true });
        setSavedPaperId(created.id);
        onLibraryChange?.(paper.arxivId, created.id);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Ошибка');
    } finally {
      setIsBookmarking(false);
    }
  };

  return (
    <article className="paper-card">
      <div className="paper-card__content">
        <div className="paper-card__main">
          <button
            type="button"
            className="paper-card__title"
            disabled={isOpening}
            onClick={() => void openInReader()}
            title={locale === 'ru' ? 'Открыть в ридере' : 'Open in reader'}
          >
            {isOpening ? (
              <span className="paper-card__title-loading">
                <LoaderCircle className="spin" size={14} strokeWidth={2} />
                {paper.title}
              </span>
            ) : (
              paper.title
            )}
          </button>
          <div className="paper-card__meta">
            <span>{formatDate(paper.publishedAt, locale)}</span>
            <span aria-hidden="true">·</span>
            <span>{formatRelativeDate(paper.publishedAt, locale)}</span>
            {paper.authors ? (
              <>
                <span aria-hidden="true">·</span>
                <span>{paper.authors}</span>
              </>
            ) : null}
            {paper.category ? (
              <>
                <span aria-hidden="true">·</span>
                <span>{paper.category}</span>
              </>
            ) : null}
          </div>
          {paper.description ? <p>{paper.description}</p> : null}
          {error ? <p className="paper-card__error">{error}</p> : null}
        </div>

        <div className="paper-card__actions">
          <button
            className={isSaved ? 'compact-button compact-button--selected' : 'compact-button'}
            type="button"
            disabled={isBookmarking || isOpening}
            onClick={() => void toggleReadingList()}
            title={
              isSaved
                ? locale === 'ru'
                  ? 'Убрать из списка чтения'
                  : 'Remove from reading list'
                : t('papers.wantToRead')
            }
          >
            {isBookmarking ? (
              <LoaderCircle className="spin" aria-hidden="true" size={15} strokeWidth={2} />
            ) : (
              <SaveIcon aria-hidden="true" size={15} strokeWidth={2} />
            )}
            <span>{isSaved ? t('papers.inList') : t('papers.wantToRead')}</span>
          </button>

          {paper.absUrl ? (
            <a
              className="compact-button compact-button--link"
              href={paper.absUrl}
              target="_blank"
              rel="noreferrer"
              title="arXiv"
              onClick={(event) => event.stopPropagation()}
            >
              <ExternalLink aria-hidden="true" size={15} strokeWidth={2} />
              <span>arXiv</span>
            </a>
          ) : null}

          <div className="compact-button compact-button--score">
            <TrendingUp aria-hidden="true" size={15} strokeWidth={2} />
            <span>{formatScore(paper.popularityScore)}</span>
          </div>
        </div>
      </div>

      <button
        type="button"
        className="paper-card__preview"
        aria-label={t('papers.pdfPreview')}
        disabled={isOpening}
        onClick={() => void openInReader()}
        style={{ ['--preview-hue' as string]: String(previewHue) }}
      >
        <div className="paper-card__preview-top">
          <span className="paper-card__preview-cat">{paper.category || 'arXiv'}</span>
          {previewYear ? <span className="paper-card__preview-year">{previewYear}</span> : null}
        </div>
        <div className="paper-card__preview-title">{paper.title}</div>
        <div className="paper-card__preview-body">{previewSnippet}</div>
        <div className="paper-card__preview-foot">arXiv:{paper.arxivId}</div>
      </button>
    </article>
  );
}
