import { queryOptions } from '@tanstack/react-query';
import { ApiError } from '../../shared/api/client';
import type { Locale } from '../../shared/i18n/i18n-context';

import { fetchTrendingPapers } from './api';
import type { TrendingSort } from './types';

export const trendingPapersQuery = (sort: TrendingSort = 'new', locale: Locale = 'ru') =>
  queryOptions({
    queryKey: ['papers', 'trending', 'brief-v1', sort, locale],
    queryFn: () => fetchTrendingPapers('cs.AI', 20, sort, locale),
    staleTime: 5 * 60 * 1000,
    retry: (count, error) =>
      error instanceof ApiError && error.code === 'feed_preparing' ? count < 12 : count < 2,
    retryDelay: (count, error) =>
      error instanceof ApiError && error.code === 'feed_preparing'
        ? 5000
        : Math.min(1000 * 2 ** count, 10000),
    refetchInterval: (query) =>
      query.state.error instanceof ApiError && query.state.error.code === 'feed_preparing'
        ? 30000
        : false,
  });
