export interface Paper {
  id: string;
  arxivId: string;
  title: string;
  description: string;
  authors: string;
  publishedAt: string;
  popularityScore: number;
  wantToRead: boolean;
  category?: string;
  absUrl?: string;
  pdfUrl?: string;
  githubUrl?: string;
  githubStars?: number;
  pdfPreviewUrl?: string;
}
