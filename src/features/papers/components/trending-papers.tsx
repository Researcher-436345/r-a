import { useQuery } from '@tanstack/react-query';

import { useI18n } from '../../../shared/i18n/i18n-context';
import { trendingPapersQuery } from '../queries';
import { PaperCard } from './paper-card';

export function TrendingPapers() {
  const { t } = useI18n();
  const { data: papers = [], isLoading, isError } = useQuery(trendingPapersQuery());

  return (
    <section className="trending-papers" aria-labelledby="trending-papers-title">
      <div className="section-header">
        <h2 id="trending-papers-title">{t('papers.trending')}</h2>
        <button className="secondary-button" type="button">
          {t('papers.personalize')}
        </button>
      </div>

      {isLoading ? <div className="state-panel">{t('papers.loading')}</div> : null}
      {isError ? <div className="state-panel">{t('papers.error')}</div> : null}

      {!isLoading && !isError ? (
        <div className="paper-list">
          {papers.map((paper) => (
            <PaperCard key={paper.id} paper={paper} />
          ))}
        </div>
      ) : null}
    </section>
  );
}
