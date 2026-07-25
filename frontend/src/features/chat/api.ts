import { API_URL } from '../../shared/config';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  createdAt: string;
}

export interface StreamHandlers {
  onDelta: (text: string) => void;
  onDone: (payload: { messageId: string; content: string }) => void;
  onError: (message: string) => void;
}

/**
 * Sends a question about an article and streams the assistant's answer via SSE.
 * Matches the backend contract: POST /v1/chats/{chatId}/messages with events
 * `delta` / `done` / `error`.
 */
export async function streamAnswer(
  params: { chatId: string; articleId: string; content: string },
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch(`${API_URL}/v1/chats/${encodeURIComponent(params.chatId)}/messages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ articleId: params.articleId, content: params.content }),
      signal,
    });
  } catch (err) {
    handlers.onError(err instanceof Error ? err.message : 'Network error');
    return;
  }

  // Errors before the stream starts arrive as a JSON body with an HTTP status.
  if (!res.ok) {
    let message = `Request failed (${res.status})`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      /* keep default */
    }
    handlers.onError(message);
    return;
  }

  if (!res.body) {
    handlers.onError('Empty response body');
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let sep: number;
    while ((sep = buffer.indexOf('\n\n')) !== -1) {
      const frame = buffer.slice(0, sep);
      buffer = buffer.slice(sep + 2);
      handleFrame(frame, handlers);
    }
  }
}

/** Fetches stored chat history. Returns [] if the chat has no messages yet. */
export async function fetchHistory(chatId: string): Promise<ChatMessage[]> {
  const res = await fetch(`${API_URL}/v1/chats/${encodeURIComponent(chatId)}/messages`);
  if (!res.ok) throw new Error(`Failed to load history (${res.status})`);
  return (await res.json()) as ChatMessage[];
}

function handleFrame(frame: string, handlers: StreamHandlers): void {
  let event = 'message';
  const dataLines: string[] = [];

  for (const line of frame.split('\n')) {
    if (line.startsWith('event:')) {
      event = line.slice('event:'.length).trim();
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length).trim());
    }
  }

  const data = dataLines.join('\n');
  if (!data) return;

  try {
    const parsed = JSON.parse(data);
    if (event === 'delta') handlers.onDelta(parsed.text ?? '');
    else if (event === 'done') handlers.onDone(parsed);
    else if (event === 'error') handlers.onError(parsed.error ?? 'Unknown error');
  } catch {
    /* ignore malformed frame */
  }
}
