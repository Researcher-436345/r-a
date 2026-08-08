import { queryOptions } from '@tanstack/react-query';

import { fetchTrendingPapers } from './api';
import type { TrendingSort } from './types';

export const trendingPapersQuery = (sort: TrendingSort = 'new') =>
  queryOptions({
    queryKey: ['papers', 'trending', sort],
    queryFn: () => fetchTrendingPapers('cs.AI', 20, sort),
  });
