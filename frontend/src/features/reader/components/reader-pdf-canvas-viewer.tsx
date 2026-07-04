import {
  getDocument,
  GlobalWorkerOptions,
  type PDFDocumentProxy,
  type RenderTask,
} from 'pdfjs-dist';
import { useEffect, useRef, useState } from 'react';

import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.js?url';

GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

interface ReaderPdfCanvasViewerProps {
  src: string;
  scale: number;
  onPageCount: (pageCount: number) => void;
}

interface ReaderPdfPageProps {
  pdf: PDFDocumentProxy;
  pageNumber: number;
  scale: number;
  defaultSize: ReaderPdfPageSize | null;
  renderImmediately: boolean;
  onNavigateToDestination: (destination: PdfLinkDestination) => void;
}

interface ReaderPdfPageSize {
  width: number;
  height: number;
}

type PdfLinkDestination = string | unknown[];

interface PdfReference {
  num: number;
  gen: number;
}

interface PdfLinkAnnotation {
  subtype?: string;
  url?: string;
  unsafeUrl?: string;
  dest?: PdfLinkDestination;
  rect?: number[];
  contents?: string;
}

interface ReaderPdfLink {
  href?: string;
  destination?: PdfLinkDestination;
  title: string;
  left: number;
  top: number;
  width: number;
  height: number;
}

export function ReaderPdfCanvasViewer({
  src,
  scale,
  onPageCount,
}: ReaderPdfCanvasViewerProps) {
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null);
  const [pageNumbers, setPageNumbers] = useState<number[]>([]);
  const [defaultPageSize, setDefaultPageSize] = useState<ReaderPdfPageSize | null>(null);
  const [hasError, setHasError] = useState(false);

  useEffect(() => {
    let isMounted = true;
    const loadingTask = getDocument(src);

    setPdf(null);
    setPageNumbers([]);
    setDefaultPageSize(null);
    setHasError(false);

    loadingTask.promise
      .then((loadedPdf) => {
        if (!isMounted) {
          void loadedPdf.destroy();
          return;
        }

        const pages = Array.from({ length: loadedPdf.numPages }, (_, index) => index + 1);
        setPdf(loadedPdf);
        setPageNumbers(pages);
        onPageCount(loadedPdf.numPages);
      })
      .catch(() => {
        if (isMounted) {
          setHasError(true);
        }
      });

    return () => {
      isMounted = false;
      void loadingTask.destroy();
    };
  }, [onPageCount, src]);

  useEffect(() => {
    if (!pdf) {
      return undefined;
    }

    let isMounted = true;

    void pdf.getPage(1).then((page) => {
      if (!isMounted) {
        return;
      }

      const viewport = page.getViewport({ scale });
      setDefaultPageSize({
        width: viewport.width,
        height: viewport.height,
      });
    });

    return () => {
      isMounted = false;
    };
  }, [pdf, scale]);

  const navigateToDestination = async (destination: PdfLinkDestination) => {
    if (!pdf) {
      return;
    }

    const resolvedDestination = Array.isArray(destination)
      ? destination
      : await pdf.getDestination(destination);
    const target = resolvedDestination?.[0];

    if (!target) {
      return;
    }

    const pageIndex =
      typeof target === 'number'
        ? target
        : await pdf.getPageIndex(target as PdfReference);
    const pageElement = document.getElementById(`reader-pdf-page-${pageIndex + 1}`);

    pageElement?.scrollIntoView({
      behavior: 'smooth',
      block: 'start',
    });
  };

  if (hasError) {
    return <div className="reader-pdf-status">PDF could not be loaded</div>;
  }

  if (!pdf) {
    return <div className="reader-pdf-status">Loading PDF...</div>;
  }

  return (
    <div className="reader-pdf-document">
      {pageNumbers.map((pageNumber) => (
        <ReaderPdfPage
          key={pageNumber}
          pageNumber={pageNumber}
          pdf={pdf}
          scale={scale}
          defaultSize={defaultPageSize}
          renderImmediately={pageNumber === 1}
          onNavigateToDestination={navigateToDestination}
        />
      ))}
    </div>
  );
}

function ReaderPdfPage({
  pdf,
  pageNumber,
  scale,
  defaultSize,
  renderImmediately,
  onNavigateToDestination,
}: ReaderPdfPageProps) {
  const pageRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [shouldRender, setShouldRender] = useState(renderImmediately);
  const [pageSize, setPageSize] = useState<ReaderPdfPageSize | null>(defaultSize);
  const [links, setLinks] = useState<ReaderPdfLink[]>([]);
  const [hasRenderedPage, setHasRenderedPage] = useState(false);
  const [hasRenderError, setHasRenderError] = useState(false);

  useEffect(() => {
    if (defaultSize && !hasRenderedPage) {
      setPageSize(defaultSize);
    }
  }, [defaultSize, hasRenderedPage]);

  useEffect(() => {
    if (shouldRender) {
      return undefined;
    }

    const pageElement = pageRef.current;

    if (!pageElement || typeof IntersectionObserver === 'undefined') {
      setShouldRender(true);
      return undefined;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setShouldRender(true);
          observer.disconnect();
        }
      },
      { rootMargin: '900px 0px' },
    );

    observer.observe(pageElement);

    return () => observer.disconnect();
  }, [shouldRender]);

  useEffect(() => {
    if (!shouldRender) {
      return undefined;
    }

    let isCancelled = false;
    let renderTask: RenderTask | null = null;
    setHasRenderError(false);

    void pdf.getPage(pageNumber).then((page) => {
      if (isCancelled) {
        return;
      }

      const viewport = page.getViewport({ scale });
      const outputScale = window.devicePixelRatio || 1;
      const nextCanvas = document.createElement('canvas');
      const nextCanvasContext = nextCanvas.getContext('2d');

      if (!nextCanvasContext) {
        return;
      }

      nextCanvas.width = Math.floor(viewport.width * outputScale);
      nextCanvas.height = Math.floor(viewport.height * outputScale);

      nextCanvasContext.setTransform(outputScale, 0, 0, outputScale, 0, 0);
      nextCanvasContext.fillStyle = '#ffffff';
      nextCanvasContext.fillRect(0, 0, viewport.width, viewport.height);

      renderTask = page.render({
        canvasContext: nextCanvasContext,
        viewport,
        background: '#ffffff',
      });

      const nextLinksPromise = page.getAnnotations({ intent: 'display' }).then((annotations) =>
        (annotations as PdfLinkAnnotation[])
          .filter((annotation) => annotation.subtype === 'Link' && annotation.rect)
          .map((annotation) => {
            const [x1, y1, x2, y2] = viewport.convertToViewportRectangle(
              annotation.rect as number[],
            );
            const left = Math.min(x1, x2);
            const top = Math.min(y1, y2);
            const width = Math.abs(x2 - x1);
            const height = Math.abs(y2 - y1);
            const href = annotation.url ?? annotation.unsafeUrl;

            return {
              href,
              destination: annotation.dest,
              title: annotation.contents ?? href ?? 'PDF link',
              left,
              top,
              width,
              height,
            };
          })
          .filter((link) => (link.href || link.destination) && link.width > 0 && link.height > 0),
      ).catch(() => []);

      renderTask.promise
        .then(async () => {
          const canvas = canvasRef.current;
          const canvasContext = canvas?.getContext('2d');
          const nextLinks = await nextLinksPromise;

          if (isCancelled || !canvas || !canvasContext) {
            return;
          }

          canvas.width = nextCanvas.width;
          canvas.height = nextCanvas.height;
          canvas.style.width = `${viewport.width}px`;
          canvas.style.height = `${viewport.height}px`;

          canvasContext.setTransform(1, 0, 0, 1, 0, 0);
          canvasContext.clearRect(0, 0, nextCanvas.width, nextCanvas.height);
          canvasContext.drawImage(nextCanvas, 0, 0);

          setPageSize({
            width: viewport.width,
            height: viewport.height,
          });
          setLinks(nextLinks);
          setHasRenderedPage(true);
        })
        .catch(() => {
          if (!isCancelled) {
            setHasRenderError(true);
          }
        });
    });

    return () => {
      isCancelled = true;
      renderTask?.cancel();
    };
  }, [pageNumber, pdf, scale, shouldRender]);

  return (
    <div
      id={`reader-pdf-page-${pageNumber}`}
      ref={pageRef}
      className="reader-pdf-page"
      style={
        pageSize
          ? {
              width: pageSize.width,
              height: pageSize.height,
            }
          : undefined
      }
    >
      {hasRenderError ? <div className="reader-pdf-page__error">Page failed to render</div> : null}
      {shouldRender ? <canvas ref={canvasRef} className="reader-pdf-page__canvas" /> : null}
      {links.length > 0 ? (
        <div className="reader-pdf-page__link-layer" aria-hidden={false}>
          {links.map((link, index) => (
            <a
              className="reader-pdf-page__link"
              href={link.href ?? '#'}
              key={`${pageNumber}-${index}-${link.left}-${link.top}`}
              target={link.href ? '_blank' : undefined}
              rel={link.href ? 'noreferrer' : undefined}
              title={link.title}
              aria-label={link.title}
              style={{
                left: link.left,
                top: link.top,
                width: link.width,
                height: link.height,
              }}
              onClick={(event) => {
                if (link.href || !link.destination) {
                  return;
                }

                event.preventDefault();
                onNavigateToDestination(link.destination);
              }}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
