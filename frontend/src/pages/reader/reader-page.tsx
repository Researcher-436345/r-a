import { useParams } from '@tanstack/react-router';
import { useCallback, useEffect, useRef, useState } from 'react';

import {
  fetchAnnotations,
  createAnnotation,
  explainPaper,
  translateText,
  type PaperAnnotation,
} from '../../features/reader/api';
import type { ChatContextAttachment } from '../../features/reader/chat-context';
import {
  DEFAULT_HIGHLIGHT_COLOR,
  toPagePixelRect,
} from '../../features/reader/highlight-colors';
import {
  fetchPaper,
  waitForPdfUrl,
  type LibraryPaper,
} from '../../features/library/api';
import { ReaderChatPanel } from '../../features/reader/components/reader-chat-panel';
import { ReaderPdfViewer } from '../../features/reader/components/reader-pdf-viewer';
import {
  ReaderSelectionPopup,
  type ReaderSelection,
} from '../../features/reader/components/reader-selection-popup';
import type {
  ReaderAnnotationFocus,
  ReaderTextSelection,
} from '../../features/reader/components/reader-pdf-canvas-viewer';
import { ApiError } from '../../shared/api/client';

export function ReaderPage() {
  const { paperId } = useParams({ strict: false }) as { paperId?: string };
  const [paper, setPaper] = useState<LibraryPaper | null>(null);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [pdfStatus, setPdfStatus] = useState<'loading' | 'ready' | 'failed' | 'idle'>('idle');
  const [annotations, setAnnotations] = useState<PaperAnnotation[]>([]);
  const [selection, setSelection] = useState<ReaderSelection | null>(null);
  const [highlightColor, setHighlightColor] = useState<string>(DEFAULT_HIGHLIGHT_COLOR);
  const [activeNoteId, setActiveNoteId] = useState<string | null>(null);
  const [flashFocus, setFlashFocus] = useState<ReaderAnnotationFocus | null>(null);
  const [chatAttachment, setChatAttachment] = useState<ChatContextAttachment | null>(null);
  const [focusAssistantToken, setFocusAssistantToken] = useState(0);
  const [focusNotesToken, setFocusNotesToken] = useState(0);
  const [focusChatMessageId, setFocusChatMessageId] = useState<string | null>(null);
  const [focusChatMessageToken, setFocusChatMessageToken] = useState(0);
  const [isSavingNote, setIsSavingNote] = useState(false);
  const [isTranslating, setIsTranslating] = useState(false);
  const [translation, setTranslation] = useState<string | null>(null);
  const [isExplaining, setIsExplaining] = useState(false);
  const [explanation, setExplanation] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(Boolean(paperId));
  const pdfObjectUrlRef = useRef<string | null>(null);
  const readerPageRef = useRef<HTMLDivElement | null>(null);
  const [chatWidth, setChatWidth] = useState(() => {
    if (typeof window === 'undefined') {
      return 420;
    }
    const raw = Number(window.localStorage.getItem('researcher.reader.chatWidth'));
    return Number.isFinite(raw) && raw >= 280 ? raw : 420;
  });
  const [isResizingChat, setIsResizingChat] = useState(false);

  const loadAnnotations = useCallback(async (id: string) => {
    const items = await fetchAnnotations(id);
    setAnnotations(items);
  }, []);

  useEffect(() => {
    if (!toast) {
      return;
    }
    const timer = window.setTimeout(() => setToast(null), 4200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    if (!isResizingChat) {
      return;
    }

    const onMove = (event: MouseEvent) => {
      const root = readerPageRef.current;
      if (!root) {
        return;
      }
      const rect = root.getBoundingClientRect();
      const next = rect.right - event.clientX;
      const minChat = 280;
      const minViewer = 320;
      const maxChat = Math.max(minChat, rect.width - minViewer);
      const clamped = Math.min(maxChat, Math.max(minChat, next));
      setChatWidth(clamped);
    };

    const onUp = () => {
      setIsResizingChat(false);
    };

    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => {
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
  }, [isResizingChat]);

  useEffect(() => {
    window.localStorage.setItem('researcher.reader.chatWidth', String(Math.round(chatWidth)));
  }, [chatWidth]);

  const showToast = (message: string) => {
    setToast(message);
  };

  const closeSelection = () => {
    setSelection(null);
    setHighlightColor(DEFAULT_HIGHLIGHT_COLOR);
    setTranslation(null);
    setIsTranslating(false);
    setExplanation(null);
    setIsExplaining(false);
    window.getSelection()?.removeAllRanges();
  };

  const focusPassage = (
    page: number,
    rect: { x: number; y: number; w: number; h: number } | null | undefined,
    options?: { rectUnit?: 'px' | 'ratio'; color?: string | null },
  ) => {
    if (!rect) {
      return;
    }
    setFlashFocus({
      id: `passage:${page}:${Date.now()}`,
      page,
      rect,
      rectUnit: options?.rectUnit ?? 'px',
      color: options?.color,
    });
  };

  const handlePassageSelect = (attachment: ChatContextAttachment) => {
    focusPassage(attachment.page, attachment.rect, { rectUnit: 'ratio' });
  };

  useEffect(() => {
    if (!paperId) {
      setIsLoading(false);
      setPdfStatus('idle');
      return;
    }

    let cancelled = false;

    const load = async () => {
      setIsLoading(true);
      setError(null);
      setPdfUrl(null);
      setPdfStatus('loading');
      if (pdfObjectUrlRef.current?.startsWith('blob:')) {
        URL.revokeObjectURL(pdfObjectUrlRef.current);
        pdfObjectUrlRef.current = null;
      }
      try {
        const nextPaper = await fetchPaper(paperId);
        if (cancelled) {
          return;
        }
        setPaper(nextPaper);
        setIsLoading(false);

        const nextAnnotations = await fetchAnnotations(paperId);
        if (!cancelled) {
          setAnnotations(nextAnnotations);
        }

        try {
          const pdf = await waitForPdfUrl(paperId);
          if (cancelled) {
            if (pdf.url.startsWith('blob:')) {
              URL.revokeObjectURL(pdf.url);
            }
            return;
          }
          pdfObjectUrlRef.current = pdf.url;
          setPdfUrl(pdf.url);
          setPdfStatus('ready');
        } catch (pdfErr) {
          if (!cancelled) {
            setPdfStatus('failed');
            setError(
              pdfErr instanceof ApiError
                ? pdfErr.detail
                : pdfErr instanceof Error
                  ? pdfErr.message
                  : 'PDF недоступен',
            );
          }
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Ошибка загрузки');
          setIsLoading(false);
          setPdfStatus('failed');
        }
      }
    };

    void load();
    return () => {
      cancelled = true;
      if (pdfObjectUrlRef.current?.startsWith('blob:')) {
        URL.revokeObjectURL(pdfObjectUrlRef.current);
        pdfObjectUrlRef.current = null;
      }
    };
  }, [paperId]);

  const handleTextSelect = (nextSelection: ReaderTextSelection) => {
    setTranslation(null);
    setIsTranslating(false);
    setExplanation(null);
    setIsExplaining(false);
    setHighlightColor(DEFAULT_HIGHLIGHT_COLOR);
    setSelection(nextSelection);
  };

  const handleNoteSelect = (note: PaperAnnotation) => {
    setActiveNoteId(note.id);
    if (note.source_chat_message_id) {
      setFocusChatMessageId(note.source_chat_message_id);
      setFocusChatMessageToken((token) => token + 1);
      setFocusAssistantToken((token) => token + 1);
      return;
    }
    if (!note.rect) {
      return;
    }
    setFlashFocus({
      id: `${note.id}:${Date.now()}`,
      page: note.page,
      rect: note.rect,
      rectUnit: 'px',
      color: note.color,
    });
  };

  const handleSaveNote = async (payload: { note: string; quote: string; color: string }) => {
    if (!paperId || !selection) {
      return;
    }

    setIsSavingNote(true);
    try {
      const pageElement = document.getElementById(`reader-pdf-page-${selection.page}`);
      const pageBox = pageElement?.getBoundingClientRect();
      const rect = pageBox
        ? toPagePixelRect(selection.rect, pageBox.width, pageBox.height)
        : selection.rect;

      const created = await createAnnotation(paperId, {
        page: selection.page,
        rect,
        selected_text: payload.quote,
        note: payload.note,
        color: payload.color,
      });
      setAnnotations((current) => [...current, created]);
      setActiveNoteId(created.id);
      setFocusNotesToken((token) => token + 1);
      closeSelection();
    } catch (err) {
      showToast(
        err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Не удалось сохранить заметку',
      );
    } finally {
      setIsSavingNote(false);
    }
  };

  const handleAskAssistant = (attachment: ChatContextAttachment) => {
    setChatAttachment(attachment);
    setFocusAssistantToken((token) => token + 1);
    closeSelection();
  };

  const handleTranslate = async (text: string) => {
    if (!paperId) {
      return;
    }
    setIsTranslating(true);
    setTranslation(null);
    try {
      const result = await translateText(paperId, text);
      setTranslation(result.translation);
    } catch (err) {
      setTranslation(
        err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Не удалось перевести',
      );
    } finally {
      setIsTranslating(false);
    }
  };

  const handleExplain = async (text: string) => {
    if (!paperId) {
      return;
    }
    setIsExplaining(true);
    setExplanation(null);
    try {
      const result = await explainPaper(paperId, text);
      setExplanation(result.reply);
    } catch (err) {
      setExplanation(
        err instanceof ApiError ? err.detail : err instanceof Error ? err.message : 'Не удалось объяснить',
      );
    } finally {
      setIsExplaining(false);
    }
  };

  if (!paperId) {
    return (
      <div className="library-page__state">
        Откройте статью из библиотеки или ленты — демо-ридер отключён.
      </div>
    );
  }

  if (isLoading) {
    return <div className="library-page__state">Загружаем статью…</div>;
  }

  if (error && !paper) {
    return <div className="library-page__error">{error}</div>;
  }

  const authors = paper?.authors.map((author) => author.name).join(', ') ?? '';
  const metaParts = [
    authors,
    paper?.arxiv_id ? `arXiv:${paper.arxiv_id}` : null,
    paper?.doi ? `DOI:${paper.doi}` : null,
    paper?.year ? String(paper.year) : null,
    paper?.latest_version ? `PDF: ${paper.latest_version.status}` : null,
  ].filter(Boolean);

  return (
    <div
      ref={readerPageRef}
      className={`reader-page${isResizingChat ? ' reader-page--resizing' : ''}`}
    >
      <ReaderPdfViewer
        title={paper?.title}
        meta={metaParts.join(' · ')}
        pdfUrl={pdfUrl}
        pdfLoading={pdfStatus === 'loading'}
        pdfError={pdfStatus === 'failed' ? error : null}
        onTextSelect={handleTextSelect}
        focusAnnotation={flashFocus}
        onFocusComplete={() => setFlashFocus(null)}
        activeHighlight={
          selection
            ? { page: selection.page, rect: selection.rect, color: highlightColor }
            : null
        }
      />
      <div
        className="reader-split-handle"
        role="separator"
        aria-orientation="vertical"
        aria-label="Изменить ширину чата"
        aria-valuenow={Math.round(chatWidth)}
        tabIndex={0}
        onMouseDown={(event) => {
          event.preventDefault();
          setIsResizingChat(true);
        }}
        onKeyDown={(event) => {
          const step = event.shiftKey ? 40 : 16;
          if (event.key === 'ArrowLeft') {
            event.preventDefault();
            setChatWidth((w) => Math.min(w + step, 720));
          } else if (event.key === 'ArrowRight') {
            event.preventDefault();
            setChatWidth((w) => Math.max(w - step, 280));
          }
        }}
      />
      <div className="reader-chat-shell" style={{ width: chatWidth }}>
        <ReaderChatPanel
          paperId={paperId}
          annotations={annotations}
          activeNoteId={activeNoteId}
          contextAttachment={chatAttachment}
          focusAssistantToken={focusAssistantToken}
          focusNotesToken={focusNotesToken}
          focusChatMessageId={focusChatMessageId}
          focusChatMessageToken={focusChatMessageToken}
          onClearContextAttachment={() => setChatAttachment(null)}
          onNoteSelect={handleNoteSelect}
          onPassageSelect={handlePassageSelect}
          onNoteUpdated={(note) => {
            setAnnotations((current) => current.map((item) => (item.id === note.id ? note : item)));
          }}
          onNoteCreated={(note) => {
            setAnnotations((current) =>
              current.some((item) => item.id === note.id) ? current : [...current, note],
            );
            setActiveNoteId(note.id);
            setFocusNotesToken((token) => token + 1);
          }}
          onAnnotationsChange={() => {
            if (paperId) {
              void loadAnnotations(paperId);
            }
          }}
        />
      </div>
      <ReaderSelectionPopup
        selection={selection}
        isSaving={isSavingNote}
        isTranslating={isTranslating}
        translation={translation}
        isExplaining={isExplaining}
        explanation={explanation}
        highlightColor={highlightColor}
        onHighlightColorChange={setHighlightColor}
        onClose={closeSelection}
        onSave={handleSaveNote}
        onAskAssistant={handleAskAssistant}
        onTranslate={(text) => {
          void handleTranslate(text);
        }}
        onExplain={(text) => {
          void handleExplain(text);
        }}
      />
      {toast ? <div className="reader-page__toast">{toast}</div> : null}
    </div>
  );
}
