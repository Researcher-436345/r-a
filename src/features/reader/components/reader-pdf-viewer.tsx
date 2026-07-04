import {
  Bookmark,
  BookmarkCheck,
  Download,
  FileText,
  Minus,
  Plus,
} from 'lucide-react';
import { useState } from 'react';

import readerPdfUrl from '../../../shared/assets/qgf-flow-policies.pdf';
import { useI18n } from '../../../shared/i18n/i18n-context';
import { readerPaper, readerStrings } from '../reader-data';
import { ReaderPdfCanvasViewer } from './reader-pdf-canvas-viewer';

const DEFAULT_READER_SCALE = 1.3;
const MIN_READER_SCALE = 0.8;
const MAX_READER_SCALE = 1.8;
const READER_SCALE_STEP = 0.1;

export function ReaderPdfViewer() {
  const { locale } = useI18n();
  const [isBookmarked, setIsBookmarked] = useState(false);
  const [pageCount, setPageCount] = useState(28);
  const [scale, setScale] = useState(DEFAULT_READER_SCALE);
  const text = readerStrings[locale];
  const BookmarkIcon = isBookmarked ? BookmarkCheck : Bookmark;
  const zoomLabel = `${Math.round(scale * 100)}%`;

  const updateScale = (direction: -1 | 1) => {
    setScale((currentScale) => {
      const nextScale = currentScale + direction * READER_SCALE_STEP;
      const clampedScale = Math.min(MAX_READER_SCALE, Math.max(MIN_READER_SCALE, nextScale));

      return Number(clampedScale.toFixed(1));
    });
  };

  return (
    <section className="reader-viewer" aria-label="PDF viewer">
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
          <div className="reader-toolbar__title">{readerPaper.title}</div>
          <div className="reader-toolbar__meta">{readerPaper.meta}</div>
        </div>

        <div className="reader-zoom" aria-label="Zoom controls">
          <button
            className="reader-zoom__button"
            type="button"
            title={text.zoomOut}
            disabled={scale <= MIN_READER_SCALE}
            onClick={() => updateScale(-1)}
          >
            <Minus aria-hidden="true" size={16} strokeWidth={2} />
          </button>
          <span className="reader-zoom__value">{zoomLabel}</span>
          <button
            className="reader-zoom__button"
            type="button"
            title={text.zoomIn}
            disabled={scale >= MAX_READER_SCALE}
            onClick={() => updateScale(1)}
          >
            <Plus aria-hidden="true" size={16} strokeWidth={2} />
          </button>
        </div>

        <div className="reader-page-count">
          <FileText aria-hidden="true" size={15} strokeWidth={2} />
          <span>1 / {pageCount}</span>
        </div>

        <a
          className="reader-download-button"
          href={readerPdfUrl}
          download
          title={text.download}
          aria-label={text.download}
        >
          <Download aria-hidden="true" size={17} strokeWidth={2} />
        </a>
      </div>

      <div className="reader-pdf-frame-wrap">
        <ReaderPdfCanvasViewer src={readerPdfUrl} scale={scale} onPageCount={setPageCount} />
      </div>
    </section>
  );
}
