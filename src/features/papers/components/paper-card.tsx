import { useI18n, type Locale } from '../../../shared/i18n/i18n-context';
import type { Paper } from '../types';

interface PaperCardProps {
  paper: Paper;
}

function formatPublishedAt(value: string, locale: Locale) {
  const publishedDate = new Date(value);
  const now = new Date();
  const diffMs = publishedDate.getTime() - now.getTime();
  const diffDays = Math.round(diffMs / (1000 * 60 * 60 * 24));

  if (Math.abs(diffDays) < 30) {
    return new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(
      diffDays,
      'day',
    );
  }

  return new Intl.DateTimeFormat(locale, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).format(publishedDate);
}

export function PaperCard({ paper }: PaperCardProps) {
  const { locale, t } = useI18n();

  return (
    <article className="paper-card">
      <div className="paper-card__content">
        <div>
          <h3>{paper.title}</h3>
          <p>{paper.description}</p>
        </div>

        <div className="paper-card__meta">
          <span>{formatPublishedAt(paper.publishedAt, locale)}</span>
          <span>
            {t('papers.popularity')}: {paper.popularityScore}
          </span>
        </div>

        <div className="paper-card__actions">
          <button
            className={
              paper.wantToRead
                ? 'compact-button compact-button--selected'
                : 'compact-button'
            }
            type="button"
          >
            {paper.wantToRead ? t('papers.inList') : t('papers.wantToRead')}
          </button>

          {paper.githubUrl ? (
            <a
              className="compact-button compact-button--link"
              href={paper.githubUrl}
              target="_blank"
              rel="noreferrer"
            >
              {t('papers.github')}
            </a>
          ) : null}
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
        </div>
      </div>
    </article>
  );
}
