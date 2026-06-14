export interface Paper {
  id: string;
  title: string;
  description: string;
  publishedAt: string;
  popularityScore: number;
  wantToRead: boolean;
  githubUrl?: string;
  pdfPreviewUrl?: string;
}
