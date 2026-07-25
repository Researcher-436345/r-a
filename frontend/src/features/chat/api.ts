import { apiFetch, apiRequest } from '../../shared/api/client';
import type { ChatMessageRole, ResearchMode } from './types';

export interface StoredChatMessage {
  id: string;
  chat_id: string;
  role: ChatMessageRole;
  content: string;
  created_at: string;
}

export interface ResearchChatSummary {
  id: string;
  title: string;
  mode: ResearchMode;
  created_at: string;
  updated_at: string;
}

export interface ResearchChat extends ResearchChatSummary {
  messages: StoredChatMessage[];
}

export function listResearchChats() {
  return apiRequest<ResearchChatSummary[]>('/research/chats');
}

export function getResearchChat(chatId: string) {
  return apiRequest<ResearchChat>(`/research/chats/${chatId}`);
}

interface StreamResearchMessageOptions {
  chatId: string;
  message: string;
  mode: ResearchMode;
  signal?: AbortSignal;
  onDelta: (content: string) => void;
}

export async function streamResearchMessage({
  chatId,
  message,
  mode,
  signal,
  onDelta,
}: StreamResearchMessageOptions): Promise<StoredChatMessage> {
  const response = await apiFetch(`/research/chats/${chatId}/messages`, {
    method: 'POST',
    body: { message, mode },
    signal,
  });
  if (!response.body) {
    throw new Error('Streaming response body is unavailable');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let completedMessage: StoredChatMessage | null = null;

  const processEvent = (rawEvent: string) => {
    let event = 'message';
    const dataLines: string[] = [];
    for (const line of rawEvent.split('\n')) {
      if (line.startsWith('event:')) {
        event = line.slice('event:'.length).trim();
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trimStart());
      }
    }
    if (dataLines.length === 0) {
      return;
    }

    const data = JSON.parse(dataLines.join('\n')) as Record<string, unknown>;
    if (event === 'delta' && typeof data.content === 'string') {
      onDelta(data.content);
    } else if (event === 'error') {
      throw new Error(
        typeof data.detail === 'string' ? data.detail : 'Research request failed',
      );
    } else if (event === 'done') {
      completedMessage = data as unknown as StoredChatMessage;
    }
  };

  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value, { stream: !done }).replace(/\r\n/g, '\n');

    let boundary = buffer.indexOf('\n\n');
    while (boundary !== -1) {
      const rawEvent = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      processEvent(rawEvent);
      boundary = buffer.indexOf('\n\n');
    }
    if (done) {
      break;
    }
  }
  if (buffer.trim()) {
    processEvent(buffer);
  }
  if (!completedMessage) {
    throw new Error('Research stream ended before completion');
  }
  return completedMessage;
}
