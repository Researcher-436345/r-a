export const researchModes = ['web', 'deep'] as const;

export type ResearchMode = (typeof researchModes)[number];

export type ChatMessageRole = 'user' | 'assistant';

export interface ChatMessage {
  id: string;
  role: ChatMessageRole;
  content: string;
  sources?: Array<{
    title: string;
    url: string;
    domain: string;
    published_at: string | null;
  }>;
  pending?: boolean;
}

export function parseResearchMode(value: unknown): ResearchMode {
  return value === 'deep' ? 'deep' : 'web';
}

export function createChatId() {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID();
  }

  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (character) => {
    const random = Math.floor(Math.random() * 16);
    const value = character === 'x' ? random : (random & 0x3) | 0x8;
    return value.toString(16);
  });
}
