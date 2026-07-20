import { useParams } from '@tanstack/react-router';
import { useCallback, useEffect, useRef, useState } from 'react';

import {
  fetchAnnotations,
  createAnnotation,
  translateText,
  type PaperAnnotation,
} from '../../features/reader/api';
import type { ChatContextAttachment } from '../../features/reader/chat-context';
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
  const [activeNoteId, setActiveNoteId] = useState<string | null>(null);
  const [flashFocus, setFlashFocus] = useState<ReaderAnnotationFocus | null>(null);
  const [chatAttachment, setChatAttachment] = useState<ChatContextAttachment | null>(null);
  const [focusAssistantToken, setFocusAssistantToken] = useState(0);
  const [focusNotesToken, setFocusNotesToken] = useState(0);
  const [isSavingNote, setIsSavingNote] = useState(false);
  const [isTranslating, setIsTranslating] = useState(false);
  const [translation, setTranslation] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(Boolean(paperId));
  const pdfObjectUrlRef = useRef<string | null>(null);

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

  const showToast = (message: string) => {
    setToast(message);
  };

  const closeSelection = () => {
    setSelection(null);
    setTranslation(null);
    setIsTranslating(false);
    window.getSelection()?.removeAllRanges();
  };

  const focusPassage = (page: number, rect: { x: number; y: number; w: number; h: number } | null | undefined) => {
    if (!rect) {
      return;
    }
    setFlashFocus({
      id: `passage:${page}:${Date.now()}`,
      page,
      rect,
    });
  };

  const handlePassageSelect = (attachment: ChatContextAttachment) => {
    focusPassage(attachment.page, attachment.rect);
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
    setSelection(nextSelection);
  };

  const handleNoteSelect = (note: PaperAnnotation) => {
    setActiveNoteId(note.id);
    if (!note.rect) {
      return;
    }
    setFlashFocus({
      id: `${note.id}:${Date.now()}`,
      page: note.page,
      rect: note.rect,
    });
  };

  const handleSaveNote = async (payload: { note: string; quote: string }) => {
    if (!paperId || !selection) {
      return;
    }

    setIsSavingNote(true);
    try {
      const created = await createAnnotation(paperId, {
        page: selection.page,
        rect: selection.rect,
        selected_text: payload.quote,
        note: payload.note,
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
    <div className="reader-page">
      <ReaderPdfViewer
        title={paper?.title}
        meta={metaParts.join(' · ')}
        pdfUrl={pdfUrl}
        pdfLoading={pdfStatus === 'loading'}
        pdfError={pdfStatus === 'failed' ? error : null}
        onTextSelect={handleTextSelect}
        focusAnnotation={flashFocus}
        onFocusComplete={() => setFlashFocus(null)}
        activeHighlight={selection ? { page: selection.page, rect: selection.rect } : null}
      />
      <ReaderChatPanel
        paperId={paperId}
        annotations={annotations}
        activeNoteId={activeNoteId}
        contextAttachment={chatAttachment}
        focusAssistantToken={focusAssistantToken}
        focusNotesToken={focusNotesToken}
        onClearContextAttachment={() => setChatAttachment(null)}
        onNoteSelect={handleNoteSelect}
        onPassageSelect={handlePassageSelect}
        onNoteUpdated={(note) => {
          setAnnotations((current) => current.map((item) => (item.id === note.id ? note : item)));
        }}
        onAnnotationsChange={() => {
          if (paperId) {
            void loadAnnotations(paperId);
          }
        }}
      />
      <ReaderSelectionPopup
        selection={selection}
        isSaving={isSavingNote}
        isTranslating={isTranslating}
        translation={translation}
        onClose={closeSelection}
        onSave={handleSaveNote}
        onAskAssistant={handleAskAssistant}
        onTranslate={(text) => {
          void handleTranslate(text);
        }}
      />
      {toast ? <div className="reader-page__toast">{toast}</div> : null}
    </div>
  );
}
