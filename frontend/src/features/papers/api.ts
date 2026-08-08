import { apiRequest } from '../../shared/api/client';
import { getAccessToken } from '../auth/token-storage';
import type { Paper, TrendingSort } from './types';

interface TrendingApiItem {
  arxiv_id: string;
  title: string;
  abstract: string | null;
  authors: string[];
  published_at: string;
  category: string;
  popularity_score: number;
  citation_count?: number | null;
  citation_source?: string | null;
  pdf_url: string;
  abs_url: string;
}

interface TrendingApiResponse {
  items: TrendingApiItem[];
  category: string;
  sort?: TrendingSort;
  cached: boolean;
}

function formatAuthors(authors: string[]): string {
  if (authors.length === 0) {
    return '';
  }
  if (authors.length <= 3) {
    return authors.join(', ');
  }
  return `${authors.slice(0, 2).join(', ')} +${authors.length - 2}`;
}

export async function fetchTrendingPapers(
  category = 'cs.AI',
  limit = 20,
  sort: TrendingSort = 'new',
): Promise<Paper[]> {
  const params = new URLSearchParams({
    category,
    limit: String(limit),
    sort,
  });
  const data = await apiRequest<TrendingApiResponse>(`/feed/trending?${params}`, {
    token: getAccessToken(),
  });

  return data.items.map((item) => ({
    id: item.arxiv_id,
    arxivId: item.arxiv_id,
    title: item.title,
    description: item.abstract ?? '',
    authors: formatAuthors(item.authors),
    publishedAt: item.published_at,
    popularityScore: item.popularity_score,
    citationCount: item.citation_count ?? null,
    citationSource: item.citation_source ?? null,
    wantToRead: false,
    category: item.category,
    absUrl: item.abs_url,
    pdfUrl: item.pdf_url,
  }));
}
