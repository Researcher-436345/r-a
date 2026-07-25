export const researchModes = ['web', 'deep'] as const;

export type ResearchMode = (typeof researchModes)[number];

export type ChatMessageRole = 'user' | 'assistant';

export interface ChatMessage {
  id: string;
  role: ChatMessageRole;
  content: string;
  pending?: boolean;
}

export function parseResearchMode(value: unknown): ResearchMode {
  return value === 'deep' ? 'deep' : 'web';
}

export function createChatId() {
  const randomPart =
    typeof globalThis.crypto?.randomUUID === 'function'
      ? globalThis.crypto.randomUUID().slice(0, 8)
      : Math.random().toString(36).slice(2, 10);

  return `${Date.now().toString(36)}-${randomPart}`;
}
