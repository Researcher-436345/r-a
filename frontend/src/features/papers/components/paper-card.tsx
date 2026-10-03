import { MetadataLine } from '../../../shared/ui/metadata-line';
import { useNavigate } from '@tanstack/react-router';
import { Bookmark, BookmarkCheck, Building2, ExternalLink, LoaderCircle, Quote } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import {
  addByArxiv,
  openByArxiv,
  patchLibraryItem,
  removeFromLibrary,
} from '../../library/api';
import { ApiError } from '../../../shared/api/client';
import { useI18n, type Locale } from '../../../shared/i18n/i18n-context';
import type { Paper } from '../types';
import { fetchPaperPreview } from '../api';
import { PaperPreview } from './paper-preview';
import { PaperDescription } from './paper-description';

interface PaperCardProps {
  paper: Paper;
  /** paper.id из нашей БД, если уже в библиотеке */
  libraryPaperId?: string | null;
  onLibraryChange?: (arxivId: string, libraryPaperId: string | null) => void;
  onVisibilityChange?: (arxivId: string, visible: boolean) => void;
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

function formatCitationCount(value: number) {
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1).replace(/\.0$/, '')}k`;
  }
  return String(value);
}

const previewPrefetchMargin = 300;

export function PaperCard({
  paper,
  libraryPaperId = null,
  onLibraryChange,
  onVisibilityChange,
}: PaperCardProps) {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const cardRef = useRef<HTMLElement>(null);
  const [savedPaperId, setSavedPaperId] = useState<string | null>(libraryPaperId);
  const [isOpening, setIsOpening] = useState(false);
  const [isBookmarking, setIsBookmarking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [failedImage, setFailedImage] = useState<string | null>(null);
  const { data: preview } = useQuery({
    queryKey: ['papers', 'preview', 'v4', paper.arxivId],
    queryFn: ({ signal }) => fetchPaperPreview(paper.arxivId, signal),
    enabled: previewVisible,
    staleTime: 24 * 60 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    retry: (count, err) => count < 6 && err instanceof ApiError && err.status === 503,
    retryDelay: (count) => Math.min(3000 * 2 ** count, 10000),
  });
  const affiliations = [...new Set(preview?.affiliations ?? [])].slice(0, 5);

  useEffect(() => {
    const node = cardRef.current;
    if (!node) return;
    if (typeof IntersectionObserver === 'undefined') {
      setPreviewVisible(true);
      return;
    }
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) showPreview();
    }, { rootMargin: `${previewPrefetchMargin}px 0px` });
    const stopObserving = () => {
      observer.disconnect();
      window.removeEventListener('scroll', measureVisibility, true);
      window.removeEventListener('resize', measureVisibility);
    };
    const showPreview = () => {
      setPreviewVisible(true);
      stopObserving();
    };
    const measureVisibility = () => {
      const rect = node.getBoundingClientRect();
      if (rect.bottom > -previewPrefetchMargin && rect.top < window.innerHeight + previewPrefetchMargin) showPreview();
    };
    observer.observe(node);
    window.addEventListener('scroll', measureVisibility, { passive: true, capture: true });
    window.addEventListener('resize', measureVisibility);
    measureVisibility();
    return stopObserving;
  }, [paper.arxivId]);

  useEffect(() => {
    setSavedPaperId(libraryPaperId);
  }, [libraryPaperId]);

  useEffect(() => {
    const node = cardRef.current;
    if (!node || !onVisibilityChange) {
      return;
    }
    if (typeof IntersectionObserver === 'undefined') {
      onVisibilityChange(paper.arxivId, true);
      return () => onVisibilityChange(paper.arxivId, false);
    }

    let visible = false;
    const observer = new IntersectionObserver(
      ([entry]) => {
        const nextVisible = entry.isIntersecting;
        if (nextVisible !== visible) {
          visible = nextVisible;
          onVisibilityChange(paper.arxivId, visible);
        }
      },
      { threshold: 0.01 },
    );
    observer.observe(node);
    return () => {
      observer.disconnect();
      if (visible) {
        onVisibilityChange(paper.arxivId, false);
      }
    };
  }, [onVisibilityChange, paper.arxivId]);

  const isSaved = Boolean(savedPaperId);
  const SaveIcon = isSaved ? BookmarkCheck : Bookmark;
  const previewYear = useMemo(() => {
    const year = new Date(paper.publishedAt).getUTCFullYear();
    return Number.isFinite(year) ? String(year) : '';
  }, [paper.publishedAt]);
  const previewSnippet = useMemo(() => {
    const text = (paper.description || paper.title).replace(/\s+/g, ' ').trim();
    return text;
  }, [paper.description, paper.title]);

  const openInReader = async () => {
    if (isOpening) {
      return;
    }
    setIsOpening(true);
    setError(null);
    try {
      if (savedPaperId) {
        await navigate({ to: '/reader/$paperId', params: { paperId: savedPaperId } });
        return;
      }
      const opened = await openByArxiv(paper.arxivId);
      await navigate({ to: '/reader/$paperId', params: { paperId: opened.id } });
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
    <article ref={cardRef} className={`paper-card${affiliations.length ? ' paper-card--affiliated' : ''}`}>
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
          {affiliations.length > 0 ? (
            <div
              className="paper-card__affiliations"
              role="list"
              aria-label={locale === 'ru' ? 'Организации авторов' : 'Author affiliations'}
            >
              {affiliations.map((name) => (
                <span className="paper-card__affiliation" role="listitem" title={name} key={name}>
                  <Building2 size={12} strokeWidth={1.7} aria-hidden="true" />
                  <span>{name}</span>
                </span>
              ))}
            </div>
          ) : null}
          <div className="paper-card__metadata-row">
            <MetadataLine
              className="paper-card__meta"
              items={[formatDate(paper.publishedAt, locale), paper.authors || '']}
            />
          </div>
          {paper.description ? (
            <PaperDescription key={paper.description} text={paper.description} />
          ) : null}
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

          {typeof paper.citationCount === 'number' && paper.citationCount > 0 ? (
            <div
              className="compact-button compact-button--citations"
              title={
                paper.citationSource
                  ? `${t('papers.citations')} · ${paper.citationSource}`
                  : t('papers.citations')
              }
            >
              <Quote aria-hidden="true" size={15} strokeWidth={2} />
              <span>{formatCitationCount(paper.citationCount)}</span>
            </div>
          ) : null}
        </div>
      </div>

      <PaperPreview
        title={paper.title}
        snippet={previewSnippet}
        year={previewYear}
        arxivId={paper.arxivId}
        label={t('papers.pdfPreview')}
        disabled={isOpening}
        image={preview?.image && failedImage !== preview.image ? preview.image : undefined}
        onOpen={() => void openInReader()}
        onImageError={() => setFailedImage(preview?.image ?? null)}
      />
    </article>
  );
}
