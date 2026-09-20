import { apiRequest, apiFetch, ApiError } from '../../shared/api/client';
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
  source_chat_message_id?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateAnnotationInput {
  page: number;
  rect?: AnnotationRect;
  selected_text: string;
  note?: string;
  color?: string;
  source_chat_message_id?: string | null;
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

type ChatStreamEvent =
  | { type: 'delta'; text?: string }
  | {
      type: 'done';
      reply: string;
      message_id?: string;
      user_message_id?: string;
      user_message?: PaperChatMessage;
      assistant_message?: PaperChatMessage;
      context_usage?: ChatContextUsage;
    }
  | { type: 'error'; detail?: string };

export async function chatPaperStream(
  paperId: string,
  request: PaperChatRequest,
  handlers: {
    onDelta?: (text: string) => void;
  } = {},
): Promise<PaperChatReply> {
  const response = await apiFetch(`/papers/${paperId}/chat?stream=1`, {
    method: 'POST',
    headers: { Accept: 'text/event-stream' },
    body: request,
  });

  if (!response.body) {
    throw new ApiError(502, 'Empty stream body');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let donePayload: Extract<ChatStreamEvent, { type: 'done' }> | null = null;

  const handleDataLine = (raw: string) => {
    const payload = raw.trim();
    if (!payload || payload === '[DONE]') {
      return;
    }
    let event: ChatStreamEvent;
    try {
      event = JSON.parse(payload) as ChatStreamEvent;
    } catch {
      return;
    }
    if (event.type === 'delta' && event.text) {
      handlers.onDelta?.(event.text);
      return;
    }
    if (event.type === 'error') {
      throw new ApiError(502, event.detail || 'LLM stream error');
    }
    if (event.type === 'done') {
      donePayload = event;
    }
  };

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split('\n');
    buffer = parts.pop() ?? '';
    for (const line of parts) {
      if (line.startsWith('data:')) {
        handleDataLine(line.slice(5));
      }
    }
  }
  if (buffer.startsWith('data:')) {
    handleDataLine(buffer.slice(5));
  }

  // TypeScript cannot see assignments performed inside the SSE line handler.
  const completed = donePayload as Extract<ChatStreamEvent, { type: 'done' }> | null;
  if (!completed) {
    throw new ApiError(502, 'Stream ended without completion');
  }

  return {
    reply: completed.reply,
    message_id: completed.message_id,
    user_message_id: completed.user_message_id,
    user_message: completed.user_message,
    assistant_message: completed.assistant_message,
    context_usage: completed.context_usage,
  };
}

export interface PaperSummary {
  paper_id: string;
  lang: string;
  version_id: string | null;
  model: string;
  content: string;
  status: 'pending' | 'ready' | 'failed';
  error_message: string | null;
  updated_at: string;
  /** Статья была перепарсена после генерации обзора */
  stale: boolean;
}

/** Возвращает закэшированный обзор или null, если он ещё не сгенерирован. */
export async function fetchPaperSummary(
  paperId: string,
  lang: string,
): Promise<PaperSummary | null> {
  try {
    return await apiRequest<PaperSummary>(
      `/papers/${paperId}/summary?lang=${encodeURIComponent(lang)}`,
      { token: authToken() },
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null;
    }
    throw err;
  }
}

type SummaryStreamEvent =
  | { type: 'delta'; text?: string }
  | { type: 'done'; summary: PaperSummary }
  | { type: 'error'; detail?: string };

export async function generatePaperSummaryStream(
  paperId: string,
  lang: string,
  options: {
    force?: boolean;
    model?: string;
    onDelta?: (text: string) => void;
    signal?: AbortSignal;
  } = {},
): Promise<PaperSummary> {
  const params = new URLSearchParams({ lang, stream: '1' });
  if (options.force) params.set('force', '1');
  if (options.model) params.set('model', options.model);
  const response = await apiFetch(`/papers/${paperId}/summary?${params.toString()}`, {
    method: 'POST',
    headers: { Accept: 'text/event-stream' },
    signal: options.signal,
  });
  if (!response.body) {
    throw new ApiError(502, 'Empty summary stream');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let result: PaperSummary | null = null;

  const handleLine = (line: string) => {
    if (!line.startsWith('data:')) {
      return;
    }
    const payload = line.slice(5).trim();
    if (!payload) {
      return;
    }
    let event: SummaryStreamEvent;
    try {
      event = JSON.parse(payload) as SummaryStreamEvent;
    } catch {
      return;
    }
    if (event.type === 'delta' && event.text) {
      options.onDelta?.(event.text);
    } else if (event.type === 'error') {
      throw new ApiError(502, event.detail || 'Summary stream error');
    } else if (event.type === 'done') {
      result = event.summary;
    }
  };

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() ?? '';
    for (const line of lines) {
      handleLine(line);
    }
  }
  buffer += decoder.decode();
  if (buffer) {
    handleLine(buffer);
  }

  const completed = result as PaperSummary | null;
  if (!completed) {
    throw new ApiError(502, 'Summary stream ended without completion');
  }
  return completed;
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
  signal?: AbortSignal,
): Promise<TranslateReply> {
  const normalized = text.trim();
  if (Array.from(normalized).length > TRANSLATION_MAX_CHARS) {
    throw new Error(`Для перевода можно выделить не более ${TRANSLATION_MAX_CHARS} символов`);
  }
  return apiRequest<TranslateReply>(`/papers/${paperId}/translate`, {
    public: true,
    method: 'POST',
    token: authToken(),
    signal,
    body: { text: normalized, target_lang: targetLang },
  });
}

type TranslationStreamEvent =
  | { type: 'delta'; text?: string }
  | { type: 'done'; translation?: string; target_lang?: string }
  | { type: 'error'; detail?: string };

export async function translateTextStream(
  paperId: string,
  text: string,
  targetLang = 'ru',
  handlers: { onDelta?: (text: string) => void } = {},
  signal?: AbortSignal,
): Promise<TranslateReply> {
  const normalized = text.trim();
  if (Array.from(normalized).length > TRANSLATION_MAX_CHARS) {
    throw new Error(`Для перевода можно выделить не более ${TRANSLATION_MAX_CHARS} символов`);
  }

  const response = await apiFetch(`/papers/${paperId}/translate?stream=1`, {
    method: 'POST',
    public: true,
    headers: { Accept: 'text/event-stream' },
    body: { text: normalized, target_lang: targetLang },
    signal,
  });

  if (!response.body) {
    throw new ApiError(502, 'Empty translation stream');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let result: TranslateReply | null = null;
  const handleLine = (line: string) => {
    if (!line.startsWith('data:')) {
      return;
    }
    const payload = line.slice(5).trim();
    if (!payload) {
      return;
    }
    let event: TranslationStreamEvent;
    try {
      event = JSON.parse(payload) as TranslationStreamEvent;
    } catch {
      return;
    }
    if (event.type === 'delta' && event.text) {
      handlers.onDelta?.(event.text);
    } else if (event.type === 'error') {
      throw new ApiError(502, event.detail || 'Translation stream error');
    } else if (event.type === 'done' && event.translation) {
      result = { translation: event.translation, target_lang: event.target_lang || targetLang };
    }
  };

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() ?? '';
    for (const line of lines) {
      handleLine(line);
    }
  }
  buffer += decoder.decode();
  if (buffer) {
    handleLine(buffer);
  }
  const completed = result as TranslateReply | null;
  if (!completed) {
    throw new ApiError(502, 'Translation stream ended without completion');
  }
  return completed;
}
