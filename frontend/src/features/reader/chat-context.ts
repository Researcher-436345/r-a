export interface ChatPassageRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface ChatContextAttachment {
  id: string;
  page: number;
  /** Координаты выделения — для прыжка по клику на чип */
  rect?: ChatPassageRect | null;
  /** Короткая метка: «стр. N · первые слова» */
  locationLabel: string;
  /** Полный/длинный превью (tooltip) */
  preview: string;
  /** Полный исходный текст выделения — то, что уйдёт модели */
  text: string;
}

/**
 * Метка чипа: страница + первые слова цитаты.
 * Когда появятся проекты — можно префиксовать названием статьи.
 */
export function buildPassageChipLabel(page: number, text: string, maxWords = 4): string {
  const cleaned = text.replace(/\s+/g, ' ').trim();
  if (!cleaned) {
    return `стр. ${page}`;
  }
  const words = cleaned.split(' ').filter(Boolean).slice(0, maxWords);
  let head = words.join(' ');
  if (cleaned.split(' ').length > maxWords) {
    head = `${head}…`;
  }
  return `стр. ${page} · ${head}`;
}
