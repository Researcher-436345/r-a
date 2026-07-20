import {
  Bookmark,
  BookmarkCheck,
  Download,
  FileText,
  LoaderCircle,
  Minus,
  Plus,
} from 'lucide-react';
import { useState } from 'react';

import { useI18n } from '../../../shared/i18n/i18n-context';
import { useTheme } from '../../../shared/theme/theme-context';
import { readerStrings } from '../reader-data';
import { ReaderPdfCanvasViewer, type ReaderAnnotationFocus, type ReaderTextSelection } from './reader-pdf-canvas-viewer';

const DEFAULT_READER_SCALE = 1.5;
const MIN_READER_SCALE = 0.8;
const MAX_READER_SCALE = 1.8;
const READER_SCALE_STEP = 0.1;

interface ReaderPdfViewerProps {
  title?: string;
  meta?: string;
  pdfUrl?: string | null;
  pdfLoading?: boolean;
  pdfError?: string | null;
  onTextSelect?: (selection: ReaderTextSelection) => void;
  focusAnnotation?: ReaderAnnotationFocus | null;
  onFocusComplete?: () => void;
  activeHighlight?: { page: number; rect: { x: number; y: number; w: number; h: number } } | null;
}

export function ReaderPdfViewer({
  title,
  meta,
  pdfUrl,
  pdfLoading = false,
  pdfError = null,
  onTextSelect,
  focusAnnotation,
  onFocusComplete,
  activeHighlight = null,
}: ReaderPdfViewerProps) {
  const { locale } = useI18n();
  const { readerDark } = useTheme();
  const [isBookmarked, setIsBookmarked] = useState(false);
  const [pageCount, setPageCount] = useState(0);
  const [scale, setScale] = useState(DEFAULT_READER_SCALE);
  const text = readerStrings[locale];
  const BookmarkIcon = isBookmarked ? BookmarkCheck : Bookmark;
  const zoomLabel = `${Math.round(scale * 100)}%`;
  const resolvedTitle = title || 'Статья';
  const resolvedMeta = meta || '';

  const updateScale = (direction: -1 | 1) => {
    setScale((currentScale) => {
      const nextScale = currentScale + direction * READER_SCALE_STEP;
      const clampedScale = Math.min(MAX_READER_SCALE, Math.max(MIN_READER_SCALE, nextScale));

      return Number(clampedScale.toFixed(1));
    });
  };

  return (
    <section
      className={readerDark ? 'reader-viewer reader-viewer--dark' : 'reader-viewer'}
      aria-label="PDF viewer"
    >
      <div className="reader-toolbar">
        <button
          className={
            isBookmarked
              ? 'reader-bookmark-button reader-bookmark-button--active'
              : 'reader-bookmark-button'
          }
          type="button"
          title={isBookmarked ? text.bookmarkRemove : text.bookmarkAdd}
          aria-label={isBookmarked ? text.bookmarkRemove : text.bookmarkAdd}
          onClick={() => setIsBookmarked((value) => !value)}
        >
          <BookmarkIcon aria-hidden="true" size={19} strokeWidth={2} />
        </button>

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
          <span className="reader-zoom__value">{zoomLabel}</span>
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
        <div className="library-page__state reader-pdf-waiting">
          <LoaderCircle className="spin" size={18} strokeWidth={2} />
          Загружаем PDF… обычно несколько секунд
        </div>
      ) : null}

      {pdfError && !pdfLoading ? (
        <div className="library-page__error" style={{ margin: '12px 16px' }}>
          {pdfError}
        </div>
      ) : null}

      {pdfUrl ? (
        <div
          className={
            readerDark ? 'reader-pdf-frame-wrap reader-pdf-frame-wrap--dark' : 'reader-pdf-frame-wrap'
          }
        >
          <ReaderPdfCanvasViewer
            src={pdfUrl}
            scale={scale}
            onPageCount={setPageCount}
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
