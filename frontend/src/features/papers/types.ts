export type TrendingSort = 'new' | 'hot' | 'popular';

export interface Paper {
  id: string;
  arxivId: string;
  title: string;
  description: string;
  authors: string;
  publishedAt: string;
  popularityScore: number;
  /** Реальные цитирования; null если источник ещё не знает статью */
  citationCount?: number | null;
  citationSource?: string | null;
  wantToRead: boolean;
  category?: string;
  absUrl?: string;
  pdfUrl?: string;
  githubUrl?: string;
  githubStars?: number;
  pdfPreviewUrl?: string;
}
