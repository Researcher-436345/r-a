export interface Paper {
  id: string;
  title: string;
  description: string;
  authors: string;
  publishedAt: string;
  popularityScore: number;
  wantToRead: boolean;
  githubUrl?: string;
  githubStars?: number;
  pdfPreviewUrl?: string;
}
