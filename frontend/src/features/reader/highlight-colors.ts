/** Спокойные пастельные цвета для выделения в PDF */
export const HIGHLIGHT_COLORS = [
  { id: 'butter', hex: '#f0e0a0', label: 'Жёлтый' },
  { id: 'mint', hex: '#b5d9c4', label: 'Мятный' },
  { id: 'sky', hex: '#a9c7e0', label: 'Голубой' },
  { id: 'peach', hex: '#e8c4b0', label: 'Персиковый' },
  { id: 'lilac', hex: '#cbb8de', label: 'Сиреневый' },
] as const;

export const DEFAULT_HIGHLIGHT_COLOR = HIGHLIGHT_COLORS[0].hex;

export type HighlightColorHex = (typeof HIGHLIGHT_COLORS)[number]['hex'] | string;

export function hexToRgba(hex: string, alpha: number): string {
  const raw = hex.replace('#', '').trim();
  const normalized =
    raw.length === 3
      ? raw
          .split('')
          .map((char) => `${char}${char}`)
          .join('')
      : raw;

  if (!/^[0-9a-fA-F]{6}$/.test(normalized)) {
    return `rgba(240, 224, 160, ${alpha})`;
  }

  const value = Number.parseInt(normalized, 16);
  const r = (value >> 16) & 255;
  const g = (value >> 8) & 255;
  const b = value & 255;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

/** Прямоугольник в долях страницы (0–1), устойчив к смене масштаба */
export interface NormalizedRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export function toNormalizedRect(
  selection: { left: number; top: number; width: number; height: number },
  page: { left: number; top: number; width: number; height: number },
): NormalizedRect {
  if (page.width <= 0 || page.height <= 0) {
    return { x: 0, y: 0, w: 0, h: 0 };
  }

  return {
    x: (selection.left - page.left) / page.width,
    y: (selection.top - page.top) / page.height,
    w: selection.width / page.width,
    h: selection.height / page.height,
  };
}

export function toPagePixelRect(
  rect: NormalizedRect,
  pageWidth: number,
  pageHeight: number,
): NormalizedRect {
  return {
    x: rect.x * pageWidth,
    y: rect.y * pageHeight,
    w: rect.w * pageWidth,
    h: rect.h * pageHeight,
  };
}
