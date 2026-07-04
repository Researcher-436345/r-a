import { Link } from '@tanstack/react-router';
import { Bookmark, BookmarkCheck, TrendingUp } from 'lucide-react';
import { useState } from 'react';

import { useI18n, type Locale } from '../../../shared/i18n/i18n-context';
import { GithubIcon } from '../../../shared/ui/github-icon';
import type { Paper } from '../types';

interface PaperCardProps {
  paper: Paper;
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
  const now = new Date('2026-06-14T12:00:00.000Z');
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

export function PaperCard({ paper }: PaperCardProps) {
  const { locale, t } = useI18n();
  const [isSaved, setIsSaved] = useState(paper.wantToRead);
  const SaveIcon = isSaved ? BookmarkCheck : Bookmark;

  return (
    <article className="paper-card">
      <div className="paper-card__content">
        <div className="paper-card__main">
          <Link className="paper-card__title" to="/reader">
            {paper.title}
          </Link>
          <div className="paper-card__meta">
            <span>{formatDate(paper.publishedAt, locale)}</span>
            <span aria-hidden="true">·</span>
            <span>{formatRelativeDate(paper.publishedAt, locale)}</span>
            <span aria-hidden="true">·</span>
            <span>{paper.authors}</span>
          </div>
          <p>{paper.description}</p>
        </div>

        <div className="paper-card__actions">
          <button
            className={isSaved ? 'compact-button compact-button--selected' : 'compact-button'}
            type="button"
            onClick={() => setIsSaved((value) => !value)}
          >
            <SaveIcon aria-hidden="true" size={15} strokeWidth={2} />
            <span>{isSaved ? t('papers.inList') : t('papers.wantToRead')}</span>
          </button>

          {paper.githubUrl ? (
            <a
              className="compact-button compact-button--link"
              href={paper.githubUrl}
              target="_blank"
              rel="noreferrer"
            >
              <GithubIcon aria-hidden="true" width={15} height={15} strokeWidth={2} />
              <span>{formatScore(paper.githubStars ?? 0)}</span>
            </a>
          ) : null}

          <div className="compact-button compact-button--score">
            <TrendingUp aria-hidden="true" size={15} strokeWidth={2} />
            <span>{formatScore(paper.popularityScore)}</span>
          </div>
        </div>
      </div>

      <div className="paper-card__preview" aria-label={t('papers.pdfPreview')}>
        <span>{t('papers.pdfPreview')}</span>
        <div className="pdf-lines" aria-hidden="true">
          <i />
          <i />
          <i />
          <i />
          <i />
          <i />
          <b />
          <i />
          <i />
          <i />
          <i />
        </div>
      </div>
    </article>
  );
}
