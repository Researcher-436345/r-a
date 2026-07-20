import { queryOptions } from '@tanstack/react-query';

import { fetchTrendingPapers } from './api';

export const trendingPapersQuery = () =>
  queryOptions({
    queryKey: ['papers', 'trending'],
    queryFn: () => fetchTrendingPapers(),
  });
