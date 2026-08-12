import { useQuery } from '@tanstack/react-query';
import { Flame, Sparkles, Zap } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { fetchLibrary, prefetchArxiv } from '../../library/api';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { SegmentedControl } from '../../../shared/ui/segmented-control';
import { trendingPapersQuery } from '../queries';
import type { TrendingSort } from '../types';
import { PaperCard } from './paper-card';

const skeletons = ['one', 'two', 'three'] as const;
const SORT_STORAGE_KEY = 'researcher.feed.sort';
const PREFETCH_AFTER_VIEWPORT = 3;

function readStoredSort(): TrendingSort {
  if (typeof window === 'undefined') {
    return 'new';
  }
  const raw = window.localStorage.getItem(SORT_STORAGE_KEY);
  if (raw === 'hot' || raw === 'popular' || raw === 'new') {
    return raw;
  }
  return 'new';
}

export function TrendingPapers() {
  const { t } = useI18n();
  const [sort, setSort] = useState<TrendingSort>(readStoredSort);
  const { data: papers = [], isLoading, isError, isFetching } = useQuery(trendingPapersQuery(sort));
  const [libraryByArxiv, setLibraryByArxiv] = useState<Record<string, string>>({});
  const [visibleArxivIds, setVisibleArxivIds] = useState<ReadonlySet<string>>(() => new Set());

  const sortOptions = useMemo(
    () =>
      [
        { value: 'new' as const, label: t('papers.sort.new'), icon: Sparkles },
        { value: 'hot' as const, label: t('papers.sort.hot'), icon: Flame },
        { value: 'popular' as const, label: t('papers.sort.popular'), icon: Zap },
      ] as const,
    [t],
  );

  const subtitleKey =
    sort === 'hot'
      ? 'papers.trendingSub.hot'
      : sort === 'popular'
        ? 'papers.trendingSub.popular'
        : 'papers.trendingSub.new';

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

  useEffect(() => {
    window.localStorage.setItem(SORT_STORAGE_KEY, sort);
  }, [sort]);

  const handleVisibilityChange = useCallback((arxivId: string, visible: boolean) => {
    setVisibleArxivIds((current) => {
      const hasArxivId = current.has(arxivId);
      if (hasArxivId === visible) {
        return current;
      }
      const next = new Set(current);
      if (visible) {
        next.add(arxivId);
      } else {
        next.delete(arxivId);
      }
      return next;
    });
  }, []);

  useEffect(() => {
    const visibleIndexes = papers
      .map((paper, index) => (visibleArxivIds.has(paper.arxivId) ? index : -1))
      .filter((index) => index >= 0);
    if (visibleIndexes.length === 0) {
      return;
    }

    const indexesToPrefetch = new Set(visibleIndexes);
    const lastVisibleIndex = Math.max(...visibleIndexes);
    for (let offset = 1; offset <= PREFETCH_AFTER_VIEWPORT; offset += 1) {
      const index = lastVisibleIndex + offset;
      if (index < papers.length) {
        indexesToPrefetch.add(index);
      }
    }

    [...indexesToPrefetch]
      .sort((left, right) => left - right)
      .forEach((index) => {
        const paper = papers[index];
        if (!libraryByArxiv[paper.arxivId]) {
          prefetchArxiv(paper.arxivId);
        }
      });
  }, [libraryByArxiv, papers, visibleArxivIds]);

  return (
    <section className="trending-papers" aria-labelledby="trending-papers-title">
      <div className="section-header">
        <div className="section-header__title">
          <h2 id="trending-papers-title">{t('papers.trending')}</h2>
          <span>{t(subtitleKey)}</span>
        </div>
        <SegmentedControl
          ariaLabel={t('papers.sort')}
          className="trending-papers__sort"
          options={sortOptions}
          value={sort}
          onChange={setSort}
        />
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
        <div className={`paper-list${isFetching ? ' paper-list--refreshing' : ''}`}>
          {papers.map((paper) => (
            <PaperCard
              key={paper.id}
              paper={paper}
              libraryPaperId={libraryByArxiv[paper.arxivId] ?? null}
              onVisibilityChange={handleVisibilityChange}
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
