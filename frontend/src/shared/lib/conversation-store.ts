type Update<T> = T | ((previous: T) => T);
export interface ConversationSnapshot<T> {
  messages: T[];
  isSending: boolean;
  loaded: boolean;
}

/** A stream belongs to a conversation, not to the mounted page that started it. */
export class ConversationStore<T> {
  private snapshot: ConversationSnapshot<T> = { messages: [], isSending: false, loaded: false };
  private listeners = new Set<() => void>();
  revision = 0;
  controller: AbortController | null = null;
  getSnapshot = () => this.snapshot;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };
  private update(patch: Partial<ConversationSnapshot<T>>) {
    this.snapshot = { ...this.snapshot, ...patch };
    this.revision += 1;
    this.listeners.forEach((listener) => listener());
  }
  setMessages = (value: Update<T[]>) => {
    this.update({ messages: typeof value === 'function' ? value(this.snapshot.messages) : value, loaded: true });
  };
  setIsSending = (isSending: boolean) => { this.update({ isSending }); };
  /** Do not let a slow history response overwrite a newer streaming update. */
  hydrate(messages: T[], revision: number) {
    if (this.revision === revision && !this.snapshot.isSending) this.setMessages(messages);
  }
}

const conversations = new Map<string, ConversationStore<unknown>>();
export function getConversationStore<T>(key: string): ConversationStore<T> {
  let store = conversations.get(key);
  if (!store) {
    store = new ConversationStore<unknown>();
    conversations.set(key, store);
  }
  return store as ConversationStore<T>;
}
export function clearConversationStores() {
  // Old callbacks can finish against their detached objects, never another user's store.
  conversations.clear();
}
