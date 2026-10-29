import { useEffect, useRef, useState, type PointerEvent } from 'react';
import { createPortal } from 'react-dom';

interface PaperPreviewProps {
  title: string;
  snippet: string;
  year: string;
  arxivId: string;
  label: string;
  image?: string;
  disabled: boolean;
  onOpen: () => void;
  onImageError: () => void;
}

interface HoverPoint {
  x: number;
  y: number;
  u: number;
  v: number;
  previewWidth: number;
}

const clamp = (value: number, low: number, high: number) => Math.min(Math.max(value, low), high);

export function PaperPreview({ title, snippet, year, arxivId, label, image, disabled, onOpen, onImageError }: PaperPreviewProps) {
  const [dimensions, setDimensions] = useState<{ image: string; aspect: number } | null>(null);
  const aspect = dimensions && dimensions.image === image ? dimensions.aspect : null;
  const [point, setPoint] = useState<HoverPoint | null>(null);
  const pendingPoint = useRef<HoverPoint | null>(null);
  const frame = useRef<number | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const close = () => {
    pendingPoint.current = null;
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    frame.current = null;
    setPoint(null);
  };

  useEffect(() => {
    close();
  }, [image]);

  useEffect(() => {
    const button = buttonRef.current;
    const dismissOutside = (event: MouseEvent) => {
      if (!pendingPoint.current || !button) return;
      const rect = button.getBoundingClientRect();
      if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) close();
    };
    button?.addEventListener('mouseleave', close);
    window.addEventListener('mousemove', dismissOutside);
    window.addEventListener('scroll', close, true);
    window.addEventListener('resize', close);
    window.addEventListener('blur', close);
    return () => {
      button?.removeEventListener('mouseleave', close);
      window.removeEventListener('mousemove', dismissOutside);
      window.removeEventListener('scroll', close, true);
      window.removeEventListener('resize', close);
      window.removeEventListener('blur', close);
      if (frame.current !== null) cancelAnimationFrame(frame.current);
    };
  }, []);

  const move = (event: PointerEvent<HTMLButtonElement>) => {
    if (event.pointerType !== 'mouse' || !image || !aspect || disabled) return;
    const rect = event.currentTarget.getBoundingClientRect();
    pendingPoint.current = {
      x: event.clientX,
      y: event.clientY,
      u: clamp((event.clientX - rect.left) / rect.width, 0, 1),
      v: clamp((event.clientY - rect.top) / rect.height, 0, 1),
      previewWidth: rect.width,
    };
    if (frame.current === null) {
      frame.current = requestAnimationFrame(() => {
        frame.current = null;
        setPoint(pendingPoint.current);
      });
    }
  };

  let lens = null;
  if (point && aspect && image && !disabled) {
    const width = Math.min(320, window.innerWidth - 16);
    const height = Math.min(240, window.innerHeight - 16);
    const left = clamp(point.x - width / 2, 8, window.innerWidth - width - 8);
    const top = clamp(point.y - height / 2, 8, window.innerHeight - height - 8);
    const imageWidth = Math.max(width, point.previewWidth * 3.25);
    const imageHeight = imageWidth * aspect;
    // Края миниатюры позволяют рассмотреть края всей первой страницы, включая низ.
    const x = -point.u * Math.max(0, imageWidth - width);
    const y = -point.v * Math.max(0, imageHeight - height);
    lens = createPortal(
      <div className="paper-preview-lens" aria-hidden="true" style={{ left, top, width, height }}>
        <img src={image} alt="" draggable={false} style={{ width: imageWidth, height: imageHeight, transform: `translate(${x}px, ${y}px)` }} />
      </div>,
      document.body,
    );
  }

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        className="paper-card__preview"
        aria-label={label}
        disabled={disabled}
        onClick={() => { close(); onOpen(); }}
        onPointerEnter={move}
        onPointerMove={move}
        onPointerLeave={close}
        onPointerCancel={close}
        onPointerDown={close}
      >
        <div className="paper-card__preview-placeholder" aria-hidden="true">
          <div className="paper-card__preview-top">{year ? <span className="paper-card__preview-year">{year}</span> : null}</div>
          <div className="paper-card__preview-title">{title}</div>
          <div className="paper-card__preview-body">{snippet}</div>
          <div className="paper-card__preview-foot">arXiv:{arxivId}</div>
        </div>
        {image ? (
          <div className="paper-card__preview-document" aria-hidden="true">
            <img
              className="paper-card__preview-image"
              src={image}
              alt=""
              draggable={false}
              onLoad={(event) => setDimensions({ image: image!, aspect: event.currentTarget.naturalHeight / event.currentTarget.naturalWidth })}
              onError={() => { close(); onImageError(); }}
            />
          </div>
        ) : null}
      </button>
      {lens}
    </>
  );
}
