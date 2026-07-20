import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  type KeyboardEvent,
} from 'react';

import type { ChatContextAttachment } from '../chat-context';

export type ComposerSegment =
  | { type: 'text'; value: string }
  | { type: 'chip'; attachment: ChatContextAttachment };

export interface ComposerSnapshot {
  displayText: string;
  plainText: string;
  attachments: ChatContextAttachment[];
  modelText: string;
  segments: ComposerSegment[];
}

export interface ChatComposerHandle {
  focus: () => void;
  insertAttachment: (attachment: ChatContextAttachment) => void;
  setPlainText: (text: string) => void;
  clear: () => void;
  getSnapshot: () => ComposerSnapshot;
  isEmpty: () => boolean;
}

interface ChatComposerProps {
  placeholder: string;
  onChange?: () => void;
  onSubmit?: () => void;
  onChipClick?: (attachment: ChatContextAttachment) => void;
}

const CHIP_SELECTOR = '[data-chat-chip="true"]';

function readChip(el: HTMLElement): ChatContextAttachment {
  let rect: ChatContextAttachment['rect'] = null;
  const rectRaw = el.dataset.rect;
  if (rectRaw) {
    try {
      rect = JSON.parse(rectRaw) as ChatContextAttachment['rect'];
    } catch {
      rect = null;
    }
  }
  return {
    id: el.dataset.chipId ?? `${Date.now()}`,
    page: Number(el.dataset.page ?? 0),
    rect,
    locationLabel: el.dataset.location ?? el.textContent?.trim() ?? '',
    preview: el.dataset.preview ?? '',
    text: el.dataset.text ?? '',
  };
}

function createChipElement(
  attachment: ChatContextAttachment,
  onChipClick?: (attachment: ChatContextAttachment) => void,
): HTMLSpanElement {
  const chip = document.createElement('span');
  chip.className = 'reader-chat-token reader-chat-token--clickable';
  chip.contentEditable = 'false';
  chip.dataset.chatChip = 'true';
  chip.dataset.chipId = attachment.id;
  chip.dataset.page = String(attachment.page);
  chip.dataset.location = attachment.locationLabel;
  chip.dataset.preview = attachment.preview;
  chip.dataset.text = attachment.text;
  if (attachment.rect) {
    chip.dataset.rect = JSON.stringify(attachment.rect);
  }
  chip.title = `${attachment.locationLabel} — ${attachment.preview || attachment.text}`;
  chip.textContent = attachment.locationLabel;
  chip.setAttribute('role', 'button');
  chip.tabIndex = 0;
  chip.addEventListener('mousedown', (event) => {
    // не давать contentEditable съесть клик / сбросить выделение
    event.preventDefault();
    event.stopPropagation();
  });
  chip.addEventListener('click', (event) => {
    event.preventDefault();
    event.stopPropagation();
    onChipClick?.(attachment);
  });
  return chip;
}

function placeCaretAfter(node: Node) {
  const selection = window.getSelection();
  if (!selection) {
    return;
  }
  const range = document.createRange();
  range.setStartAfter(node);
  range.collapse(true);
  selection.removeAllRanges();
  selection.addRange(range);
}

function rangeStillInEditor(editor: HTMLElement, range: Range | null): range is Range {
  if (!range) {
    return false;
  }
  try {
    return editor.contains(range.startContainer) && editor.contains(range.endContainer);
  } catch {
    return false;
  }
}

function pushTextSegment(segments: ComposerSegment[], value: string) {
  if (!value) {
    return;
  }
  const last = segments[segments.length - 1];
  if (last?.type === 'text') {
    last.value += value;
    return;
  }
  segments.push({ type: 'text', value });
}

function serialize(editor: HTMLElement): ComposerSnapshot {
  const attachments: ChatContextAttachment[] = [];
  const segments: ComposerSegment[] = [];
  let displayText = '';
  let plainText = '';
  let modelText = '';

  const walk = (node: Node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      const value = (node.textContent ?? '').replace(/\u200B/g, '');
      displayText += value;
      plainText += value;
      modelText += value;
      pushTextSegment(segments, value);
      return;
    }
    if (!(node instanceof HTMLElement)) {
      return;
    }
    if (node.dataset.chatChip === 'true') {
      const attachment = readChip(node);
      attachments.push(attachment);
      segments.push({ type: 'chip', attachment });
      displayText += attachment.locationLabel;
      modelText += `\n\n[${attachment.locationLabel} · стр. ${attachment.page}]\n${attachment.text}\n`;
      return;
    }
    if (node.tagName === 'BR') {
      displayText += '\n';
      plainText += '\n';
      modelText += '\n';
      pushTextSegment(segments, '\n');
      return;
    }
    node.childNodes.forEach(walk);
  };

  editor.childNodes.forEach(walk);

  return {
    displayText: displayText.replace(/\u200B/g, '').trim(),
    plainText: plainText.replace(/\u200B/g, '').trim(),
    attachments,
    modelText: modelText.replace(/\u200B/g, '').trim(),
    segments,
  };
}

function isVisuallyEmpty(editor: HTMLElement) {
  const snap = serialize(editor);
  return !snap.plainText && snap.attachments.length === 0;
}

export const ChatComposer = forwardRef<ChatComposerHandle, ChatComposerProps>(function ChatComposer(
  { placeholder, onChange, onSubmit, onChipClick },
  ref,
) {
  const editorRef = useRef<HTMLDivElement | null>(null);
  /** Курсор до ухода в PDF — вставка фрагмента сюда, а не в конец */
  const savedRangeRef = useRef<Range | null>(null);
  const onChipClickRef = useRef(onChipClick);
  onChipClickRef.current = onChipClick;

  const emitChange = () => {
    const editor = editorRef.current;
    if (!editor) {
      return;
    }
    editor.dataset.empty = isVisuallyEmpty(editor) ? 'true' : 'false';
    onChange?.();
  };

  const rememberCaret = () => {
    const editor = editorRef.current;
    const selection = window.getSelection();
    if (!editor || !selection || selection.rangeCount === 0) {
      return;
    }
    if (!editor.contains(selection.anchorNode)) {
      return;
    }
    savedRangeRef.current = selection.getRangeAt(0).cloneRange();
  };

  const restoreCaret = () => {
    const editor = editorRef.current;
    const selection = window.getSelection();
    if (!editor || !selection) {
      return false;
    }
    if (!rangeStillInEditor(editor, savedRangeRef.current)) {
      return false;
    }
    selection.removeAllRanges();
    selection.addRange(savedRangeRef.current);
    return true;
  };

  useImperativeHandle(ref, () => ({
    focus: () => {
      const editor = editorRef.current;
      if (!editor) {
        return;
      }
      editor.focus();
      if (!restoreCaret()) {
        placeCaretAfter(editor.lastChild ?? editor);
      }
    },
    insertAttachment: (attachment) => {
      const editor = editorRef.current;
      if (!editor) {
        return;
      }

      // Берём сохранённый курсор ДО focus(): focus() часто ставит каретку в начало.
      const insertAt =
        rangeStillInEditor(editor, savedRangeRef.current)
          ? savedRangeRef.current.cloneRange()
          : null;

      editor.focus();

      const chip = createChipElement(attachment, (item) => onChipClickRef.current?.(item));
      const spacer = document.createTextNode('\u200B');

      if (insertAt) {
        insertAt.collapse(true);
        insertAt.insertNode(spacer);
        insertAt.insertNode(chip);
        placeCaretAfter(spacer);
      } else {
        editor.appendChild(chip);
        editor.appendChild(spacer);
        placeCaretAfter(spacer);
      }

      rememberCaret();
      emitChange();
    },
    setPlainText: (text) => {
      const editor = editorRef.current;
      if (!editor) {
        return;
      }
      editor.textContent = text;
      emitChange();
      placeCaretAfter(editor.lastChild ?? editor);
      rememberCaret();
    },
    clear: () => {
      const editor = editorRef.current;
      if (!editor) {
        return;
      }
      editor.replaceChildren();
      savedRangeRef.current = null;
      emitChange();
    },
    getSnapshot: () => {
      const editor = editorRef.current;
      if (!editor) {
        return { displayText: '', plainText: '', attachments: [], modelText: '', segments: [] };
      }
      return serialize(editor);
    },
    isEmpty: () => {
      const editor = editorRef.current;
      return editor ? isVisuallyEmpty(editor) : true;
    },
  }));

  useEffect(() => {
    emitChange();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount only
  }, []);

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      onSubmit?.();
      return;
    }

    if (event.key !== 'Backspace' && event.key !== 'Delete') {
      return;
    }

    const editor = editorRef.current;
    const selection = window.getSelection();
    if (!editor || !selection || !selection.isCollapsed || selection.rangeCount === 0) {
      return;
    }

    const range = selection.getRangeAt(0);
    const { startContainer, startOffset } = range;

    const deleteChip = (chip: HTMLElement) => {
      event.preventDefault();
      const next = chip.nextSibling;
      chip.remove();
      if (next?.nodeType === Node.TEXT_NODE && (next.textContent === '\u200B' || next.textContent === '')) {
        next.remove();
      }
      rememberCaret();
      emitChange();
    };

    if (event.key === 'Backspace') {
      if (startContainer.nodeType === Node.TEXT_NODE) {
        const text = startContainer.textContent ?? '';
        const before = text.slice(0, startOffset).replace(/\u200B/g, '');
        if (before.length === 0) {
          const prev = startContainer.previousSibling;
          if (prev instanceof HTMLElement && prev.matches(CHIP_SELECTOR)) {
            deleteChip(prev);
          }
        }
        return;
      }

      if (startContainer === editor && startOffset > 0) {
        const prev = editor.childNodes[startOffset - 1];
        if (prev instanceof HTMLElement && prev.matches(CHIP_SELECTOR)) {
          deleteChip(prev);
        } else if (
          prev?.nodeType === Node.TEXT_NODE &&
          !(prev.textContent ?? '').replace(/\u200B/g, '')
        ) {
          const beforePrev = prev.previousSibling;
          if (beforePrev instanceof HTMLElement && beforePrev.matches(CHIP_SELECTOR)) {
            deleteChip(beforePrev);
          }
        }
      }
      return;
    }

    if (startContainer.nodeType === Node.TEXT_NODE) {
      const text = startContainer.textContent ?? '';
      const after = text.slice(startOffset).replace(/\u200B/g, '');
      if (after.length === 0) {
        const next = startContainer.nextSibling;
        if (next instanceof HTMLElement && next.matches(CHIP_SELECTOR)) {
          deleteChip(next);
        }
      }
      return;
    }

    if (startContainer === editor) {
      const next = editor.childNodes[startOffset];
      if (next instanceof HTMLElement && next.matches(CHIP_SELECTOR)) {
        deleteChip(next);
      }
    }
  };

  return (
    <div
      ref={editorRef}
      className="reader-chat-composer"
      contentEditable
      role="textbox"
      aria-multiline="true"
      aria-label="Сообщение ассистенту"
      data-empty="true"
      data-placeholder={placeholder}
      suppressContentEditableWarning
      onInput={() => {
        rememberCaret();
        emitChange();
      }}
      onKeyUp={rememberCaret}
      onMouseUp={rememberCaret}
      onBlur={rememberCaret}
      onKeyDown={handleKeyDown}
    />
  );
});
