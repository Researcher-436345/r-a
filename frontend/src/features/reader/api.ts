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
  const data = await apiRequest<PaperAnnotation[] | null>(`/papers/${paperId}/annotations`, {
    token: authToken(),
  });
  return data ?? [];
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

export interface PaperChatMessage {
  id: string;
  paper_id: string;
  user_id: string;
  role: 'user' | 'assistant';
  content: string;
  context_text: string | null;
  created_at: string;
}

export interface ChatContextUsage {
  used_tokens: number;
  limit_tokens: number;
  percent: number;
  paper_tokens: number;
  history_tokens: number;
  has_full_paper: boolean;
  model: string;
}

export interface LLMModelOption {
  id: string;
  label: string;
}

export interface PaperChatReply {
  reply: string;
  message_id?: string;
  user_message_id?: string;
  user_message?: PaperChatMessage;
  assistant_message?: PaperChatMessage;
  context_usage?: ChatContextUsage;
}

export interface PaperChatRequest {
  message: string;
  context_text?: string | null;
  model?: string;
}

export async function fetchChatMessages(paperId: string): Promise<PaperChatMessage[]> {
  const data = await apiRequest<{ items: PaperChatMessage[] }>(`/papers/${paperId}/chat/messages`, {
    token: authToken(),
  });
  return data.items ?? [];
}

export async function fetchChatContext(
  paperId: string,
  model?: string,
): Promise<ChatContextUsage> {
  const q = model ? `?model=${encodeURIComponent(model)}` : '';
  return apiRequest<ChatContextUsage>(`/papers/${paperId}/chat/context${q}`, {
    token: authToken(),
  });
}

export async function fetchAssistantModels(): Promise<{
  default: string;
  items: LLMModelOption[];
}> {
  return apiRequest<{ default: string; items: LLMModelOption[] }>('/assistant/models', {
    token: authToken(),
  });
}

export async function chatPaper(paperId: string, request: PaperChatRequest): Promise<PaperChatReply> {
  return apiRequest<PaperChatReply>(`/papers/${paperId}/chat`, {
    method: 'POST',
    token: authToken(),
    body: request,
  });
}

export interface ExplainReply {
  reply: string;
}

export async function explainPaper(
  paperId: string,
  text: string,
  question?: string,
): Promise<ExplainReply> {
  return apiRequest<ExplainReply>(`/papers/${paperId}/explain`, {
    method: 'POST',
    token: authToken(),
    body: { text, question: question || undefined },
  });
}

export interface TranslateReply {
  translation: string;
  target_lang: string;
}

export const TRANSLATION_MAX_CHARS = 5000;

export async function translateText(
  paperId: string,
  text: string,
  targetLang = 'ru',
): Promise<TranslateReply> {
  const normalized = text.trim();
  if (Array.from(normalized).length > TRANSLATION_MAX_CHARS) {
    throw new Error(`Для перевода можно выделить не более ${TRANSLATION_MAX_CHARS} символов`);
  }
  return apiRequest<TranslateReply>(`/papers/${paperId}/translate`, {
    method: 'POST',
    token: authToken(),
    body: { text: normalized, target_lang: targetLang },
  });
}
