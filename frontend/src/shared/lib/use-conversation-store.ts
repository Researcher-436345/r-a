import { useSyncExternalStore } from 'react';
import { getConversationStore } from './conversation-store';

export function useConversationStore<T>(key: string) {
  const store = getConversationStore<T>(key);
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot, store.getSnapshot);
  return { ...snapshot, store, setMessages: store.setMessages, setIsSending: store.setIsSending };
}
