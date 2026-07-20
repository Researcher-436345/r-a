import { useQuery } from '@tanstack/react-query';
import { SlidersHorizontal } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';

import { fetchLibrary } from '../../library/api';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { trendingPapersQuery } from '../queries';
import { PaperCard } from './paper-card';

const skeletons = ['one', 'two', 'three'] as const;

export function TrendingPapers() {
  const { t } = useI18n();
  const { data: papers = [], isLoading, isError } = useQuery(trendingPapersQuery());
  const [libraryByArxiv, setLibraryByArxiv] = useState<Record<string, string>>({});

  const refreshLibraryMap = useCallback(async () => {
    try {
      const data = await fetchLibrary(1, 100);
      const map: Record<string, string> = {};
      for (const item of data.items) {
        if (item.paper.arxiv_id) {
          map[item.paper.arxiv_id] = item.paper.id;
        }
      }
      setLibraryByArxiv(map);
    } catch {
      // лента работает и без карты библиотеки
    }
  }, []);

  useEffect(() => {
    void refreshLibraryMap();
  }, [refreshLibraryMap]);

  return (
    <section className="trending-papers" aria-labelledby="trending-papers-title">
      <div className="section-header">
        <div className="section-header__title">
          <h2 id="trending-papers-title">{t('papers.trending')}</h2>
          <span>{t('papers.trendingSub')}</span>
        </div>
        <button className="secondary-button" type="button">
          <SlidersHorizontal aria-hidden="true" size={15} strokeWidth={2} />
          <span>{t('papers.personalize')}</span>
        </button>
      </div>

      {isLoading ? (
        <div className="paper-list">
          {skeletons.map((skeleton) => (
            <article className="paper-card-skeleton" key={skeleton} aria-hidden="true">
              <div className="paper-card-skeleton__content">
                <i />
                <i />
                <i />
                <i />
                <i />
              </div>
              <div className="paper-card-skeleton__preview" />
            </article>
          ))}
        </div>
      ) : null}
      {isError ? <div className="state-panel">{t('papers.error')}</div> : null}

      {!isLoading && !isError ? (
        <div className="paper-list">
          {papers.map((paper) => (
            <PaperCard
              key={paper.id}
              paper={paper}
              libraryPaperId={libraryByArxiv[paper.arxivId] ?? null}
              onLibraryChange={(arxivId, libraryPaperId) => {
                setLibraryByArxiv((current) => {
                  const next = { ...current };
                  if (libraryPaperId) {
                    next[arxivId] = libraryPaperId;
                  } else {
                    delete next[arxivId];
                  }
                  return next;
                });
              }}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}
