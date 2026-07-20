import { apiRequest } from '../../shared/api/client';
import { getAccessToken } from '../auth/token-storage';

export interface AnnotationRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface PaperAnnotation {
  id: string;
  paper_id: string;
  page: number;
  rect: AnnotationRect | null;
  selected_text: string;
  note: string;
  color: string;
  created_at: string;
  updated_at: string;
}

export interface CreateAnnotationInput {
  page: number;
  rect?: AnnotationRect;
  selected_text: string;
  note?: string;
  color?: string;
}

function authToken() {
  return getAccessToken();
}

export async function fetchAnnotations(paperId: string): Promise<PaperAnnotation[]> {
  return apiRequest<PaperAnnotation[]>(`/papers/${paperId}/annotations`, {
    token: authToken(),
  });
}

export async function createAnnotation(
  paperId: string,
  input: CreateAnnotationInput,
): Promise<PaperAnnotation> {
  return apiRequest<PaperAnnotation>(`/papers/${paperId}/annotations`, {
    method: 'POST',
    token: authToken(),
    body: input,
  });
}

export async function updateAnnotation(annotationId: string, note: string): Promise<PaperAnnotation> {
  return apiRequest<PaperAnnotation>(`/annotations/${annotationId}`, {
    method: 'PATCH',
    token: authToken(),
    body: { note },
  });
}

export async function deleteAnnotation(annotationId: string): Promise<void> {
  return apiRequest<void>(`/annotations/${annotationId}`, {
    method: 'DELETE',
    token: authToken(),
  });
}

export interface PaperChatReply {
  reply: string;
}

export interface PaperChatRequest {
  message: string;
  context_text?: string | null;
}

export async function chatPaper(paperId: string, request: PaperChatRequest): Promise<PaperChatReply> {
  return apiRequest<PaperChatReply>(`/papers/${paperId}/chat`, {
    method: 'POST',
    token: authToken(),
    body: request,
  });
}

export interface TranslateReply {
  translation: string;
  detected_source?: string | null;
}

export async function translateText(
  paperId: string,
  text: string,
  targetLang = 'ru',
): Promise<TranslateReply> {
  return apiRequest<TranslateReply>(`/papers/${paperId}/translate`, {
    method: 'POST',
    token: authToken(),
    body: { text, target_lang: targetLang },
  });
}
