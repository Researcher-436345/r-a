import { useLayoutEffect, useRef, type UIEvent, type WheelEvent } from 'react';

/** Follow new content only until the reader scrolls away from the bottom. */
export function useFollowScroll(conversationId: string | undefined, content: unknown, visible = true) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const following = useRef(true);
  const previousConversation = useRef(conversationId);

  const onThreadScroll = (event: UIEvent<HTMLDivElement>) => {
    const node = event.currentTarget;
    // A tiny rounding tolerance, not a "near the bottom" zone that traps the reader.
    following.current = node.scrollHeight - node.clientHeight - node.scrollTop <= 2;
  };

  const onThreadWheel = (event: WheelEvent<HTMLDivElement>) => {
    // Stop before the browser's scroll event, so an incoming token cannot race
    // an upward wheel/trackpad gesture. Touch, keyboard and dragging use onScroll.
    if (event.deltaY < 0) following.current = false;
  };

  useLayoutEffect(() => {
    if (previousConversation.current !== conversationId) {
      previousConversation.current = conversationId;
      following.current = true;
    }
    const node = scrollRef.current;
    if (visible && node && following.current) {
      // Never scroll ancestors or queue smooth animations for every token.
      node.scrollTop = node.scrollHeight;
    }
  }, [conversationId, content, visible]);

  return { scrollRef, onThreadScroll, onThreadWheel };
}
