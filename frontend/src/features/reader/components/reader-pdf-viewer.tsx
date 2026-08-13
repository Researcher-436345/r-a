import {
  Bookmark,
  BookmarkCheck,
  Check,
  Download,
  FileText,
  LoaderCircle,
  Minus,
  Plus,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import type { LibraryFolder } from '../../library/api';
import { flattenLibraryFolders, LibraryFolderIcon } from '../../library/folder-icons';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { useTheme } from '../../../shared/theme/theme-context';
import { readerStrings } from '../reader-data';
import { ReaderPdfCanvasViewer, type ReaderAnnotationFocus, type ReaderTextSelection } from './reader-pdf-canvas-viewer';

const MIN_READER_SCALE = 0.4;
const MAX_READER_SCALE = 2.5;
const READER_SCALE_STEP = 0.1;
/** Горизонтальный padding `.reader-pdf-frame-wrap` (28px с каждой стороны) */
const FRAME_HORIZONTAL_PADDING = 56;

interface ReaderPdfViewerProps {
  title?: string;
  meta?: string;
  pdfUrl?: string | null;
  pdfLoading?: boolean;
  pdfError?: string | null;
  libraryFolders?: LibraryFolder[];
  currentFolderId?: string | null;
  foldersLoading?: boolean;
  savingFolderId?: string | null;
  folderError?: string | null;
  onFolderSelect?: (folderId: string) => Promise<void>;
  onTextSelect?: (selection: ReaderTextSelection) => void;
  focusAnnotation?: ReaderAnnotationFocus | null;
  onFocusComplete?: () => void;
  activeHighlight?: {
    page: number;
    rect: { x: number; y: number; w: number; h: number };
    color?: string | null;
  } | null;
}

function clampScale(value: number) {
  return Math.min(MAX_READER_SCALE, Math.max(MIN_READER_SCALE, value));
}

function fitScaleForWidth(availableWidth: number, basePageWidth: number) {
  if (availableWidth <= 0 || basePageWidth <= 0) {
    return 1;
  }

  // Небольшой запас, чтобы не появлялся горизонтальный скролл из‑за округления
  return clampScale(Number(((availableWidth - 4) / basePageWidth).toFixed(3)));
}

export function ReaderPdfViewer({
  title,
  meta,
  pdfUrl,
  pdfLoading = false,
  pdfError = null,
  libraryFolders = [],
  currentFolderId = null,
  foldersLoading = false,
  savingFolderId = null,
  folderError = null,
  onFolderSelect,
  onTextSelect,
  focusAnnotation,
  onFocusComplete,
  activeHighlight = null,
}: ReaderPdfViewerProps) {
  const { locale } = useI18n();
  const { readerDark } = useTheme();
  const [isFolderMenuOpen, setIsFolderMenuOpen] = useState(false);
  const [pageCount, setPageCount] = useState(0);
  const [scale, setScale] = useState(1);
  /** Пока true — масштаб подстраивается под ширину области PDF */
  const [fitToWidth, setFitToWidth] = useState(true);
  const [basePageWidth, setBasePageWidth] = useState<number | null>(null);
  const frameWrapRef = useRef<HTMLDivElement | null>(null);
  const bookmarkMenuRef = useRef<HTMLDivElement | null>(null);
  const text = readerStrings[locale];
  const isBookmarked = Boolean(currentFolderId);
  const BookmarkIcon = isBookmarked ? BookmarkCheck : Bookmark;
  const folderChoices = useMemo(
    () => flattenLibraryFolders(libraryFolders),
    [libraryFolders],
  );
  const zoomLabel = fitToWidth ? 'Fit' : `${Math.round(scale * 100)}%`;
  const resolvedTitle = title || 'Статья';
  const resolvedMeta = meta || '';

  const applyFitToWidth = useCallback(() => {
    const wrap = frameWrapRef.current;
    if (!wrap || !basePageWidth) {
      return;
    }

    const availableWidth = wrap.clientWidth - FRAME_HORIZONTAL_PADDING;
    setScale(fitScaleForWidth(availableWidth, basePageWidth));
  }, [basePageWidth]);

  const handleBasePageWidth = useCallback((width: number) => {
    setBasePageWidth(width);
  }, []);

  useEffect(() => {
    setBasePageWidth(null);
    setFitToWidth(true);
    setScale(1);
  }, [pdfUrl]);

  useEffect(() => {
    if (!fitToWidth || !basePageWidth) {
      return undefined;
    }

    applyFitToWidth();

    const wrap = frameWrapRef.current;
    if (!wrap || typeof ResizeObserver === 'undefined') {
      return undefined;
    }

    const observer = new ResizeObserver(() => {
      applyFitToWidth();
    });
    observer.observe(wrap);

    return () => {
      observer.disconnect();
    };
  }, [applyFitToWidth, basePageWidth, fitToWidth, pdfUrl]);

  useEffect(() => {
    if (!isFolderMenuOpen) {
      return;
    }
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!bookmarkMenuRef.current?.contains(event.target as Node)) {
        setIsFolderMenuOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsFolderMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [isFolderMenuOpen]);

  const updateScale = (direction: -1 | 1) => {
    setFitToWidth(false);
    setScale((currentScale) => {
      const nextScale = currentScale + direction * READER_SCALE_STEP;
      return Number(clampScale(nextScale).toFixed(1));
    });
  };

  const resetFitToWidth = () => {
    setFitToWidth(true);
  };

  return (
    <section
      className={readerDark ? 'reader-viewer reader-viewer--dark' : 'reader-viewer'}
      aria-label="PDF viewer"
    >
      <div className="reader-toolbar">
        <div className="reader-bookmark-menu" ref={bookmarkMenuRef}>
          <button
            className={
              isBookmarked
                ? 'reader-bookmark-button reader-bookmark-button--active'
                : 'reader-bookmark-button'
            }
            type="button"
            title={text.bookmarkAdd}
            aria-label={text.bookmarkAdd}
            aria-haspopup="dialog"
            aria-expanded={isFolderMenuOpen}
            onClick={() => setIsFolderMenuOpen((value) => !value)}
          >
            <BookmarkIcon aria-hidden="true" size={19} strokeWidth={2} />
          </button>
          {isFolderMenuOpen ? (
            <div className="reader-folder-popover" role="dialog" aria-label={text.chooseFolder}>
              <div className="reader-folder-popover__heading">{text.chooseFolder}</div>
              {foldersLoading ? (
                <div className="reader-folder-popover__state">
                  <LoaderCircle className="spin" aria-hidden="true" size={14} />
                  {text.foldersLoading}
                </div>
              ) : (
                <div className="reader-folder-popover__folders">
                  {folderChoices.map(({ folder, depth }) => {
                    const isCurrent = folder.id === currentFolderId;
                    const isSaving = folder.id === savingFolderId;
                    return (
                      <button
                        type="button"
                        key={folder.id}
                        className={isCurrent ? 'is-current' : undefined}
                        disabled={Boolean(savingFolderId)}
                        onClick={() => {
                          if (!onFolderSelect) {
                            return;
                          }
                          void onFolderSelect(folder.id)
                            .then(() => setIsFolderMenuOpen(false))
                            .catch(() => {
                              // Ошибка показана внутри popover; оставляем его открытым.
                            });
                        }}
                      >
                        <span
                          className="reader-folder-popover__label"
                          style={{ paddingLeft: depth * 12 }}
                        >
                          <LibraryFolderIcon folder={folder} />
                          <span>{folder.name}</span>
                        </span>
                        {isSaving ? (
                          <LoaderCircle className="spin" aria-hidden="true" size={14} />
                        ) : isCurrent ? (
                          <Check aria-hidden="true" size={14} strokeWidth={2.2} />
                        ) : null}
                      </button>
                    );
                  })}
                </div>
              )}
              {folderError ? (
                <div className="reader-folder-popover__error">{folderError}</div>
              ) : null}
            </div>
          ) : null}
        </div>

        <div className="reader-toolbar__divider" />

        <div className="reader-toolbar__paper">
          <div className="reader-toolbar__title">{resolvedTitle}</div>
          <div className="reader-toolbar__meta">{resolvedMeta}</div>
        </div>

        <div className="reader-zoom" aria-label="Zoom controls">
          <button
            className="reader-zoom__button"
            type="button"
            title={text.zoomOut}
            disabled={scale <= MIN_READER_SCALE || !pdfUrl}
            onClick={() => updateScale(-1)}
          >
            <Minus aria-hidden="true" size={16} strokeWidth={2} />
          </button>
          <button
            className="reader-zoom__value reader-zoom__value--button"
            type="button"
            title="Вписать по ширине"
            disabled={!pdfUrl}
            onClick={resetFitToWidth}
          >
            {zoomLabel}
          </button>
          <button
            className="reader-zoom__button"
            type="button"
            title={text.zoomIn}
            disabled={scale >= MAX_READER_SCALE || !pdfUrl}
            onClick={() => updateScale(1)}
          >
            <Plus aria-hidden="true" size={16} strokeWidth={2} />
          </button>
        </div>

        <div className="reader-page-count">
          <FileText aria-hidden="true" size={15} strokeWidth={2} />
          <span>{pageCount > 0 ? `1 / ${pageCount}` : '—'}</span>
        </div>

        {pdfUrl ? (
          <a
            className="reader-download-button"
            href={pdfUrl}
            download
            title={text.download}
            aria-label={text.download}
          >
            <Download aria-hidden="true" size={17} strokeWidth={2} />
          </a>
        ) : null}
      </div>

      {pdfLoading ? (
        <div
          className="library-page__state reader-pdf-waiting"
          role="status"
          aria-label="Загрузка PDF"
        >
          <LoaderCircle className="spin" size={18} strokeWidth={2} />
        </div>
      ) : null}

      {pdfError && !pdfLoading ? (
        <div className="library-page__error" style={{ margin: '12px 16px' }}>
          {pdfError}
        </div>
      ) : null}

      {pdfUrl ? (
        <div
          ref={frameWrapRef}
          className={[
            'reader-pdf-frame-wrap',
            readerDark ? 'reader-pdf-frame-wrap--dark' : '',
            fitToWidth ? '' : 'reader-pdf-frame-wrap--manual-zoom',
          ]
            .filter(Boolean)
            .join(' ')}
        >
          <ReaderPdfCanvasViewer
            src={pdfUrl}
            scale={scale}
            onPageCount={setPageCount}
            onBasePageWidth={handleBasePageWidth}
            onTextSelect={onTextSelect}
            focusAnnotation={focusAnnotation}
            onFocusComplete={onFocusComplete}
            activeHighlight={activeHighlight}
          />
        </div>
      ) : null}
    </section>
  );
}
