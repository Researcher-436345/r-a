/**
 * PDF text extraction often glues figure labels ("TamilTeluguThai…")
 * or captures half a caption. Clean for display / storage.
 */

const FIGURE_RE =
  /\b((?:Figure|Fig\.|Рис(?:унок)?\.?|Table|Таблица)\s*[.:]?\s*\d+[a-zA-Z]?)\b/i;

const MAX_QUOTE_LENGTH = 220;

function softBreakLongTokens(text: string, maxToken = 36): string {
  return text.replace(new RegExp(`\\S{${maxToken},}`, 'g'), (token) =>
    token.replace(new RegExp(`(.{1,${Math.floor(maxToken / 2)}})`, 'g'), '$1\u200b'),
  );
}

function looksLikeGarbage(text: string): boolean {
  const compact = text.replace(/\s+/g, '');
  if (compact.length < 24) {
    return false;
  }
  const spaces = (text.match(/\s/g) ?? []).length;
  const spaceRatio = spaces / Math.max(text.length, 1);
  // "ianTamilTeluguThaiChinese…" — почти без пробелов
  if (spaceRatio < 0.04 && compact.length > 40) {
    return true;
  }
  // Много CamelCase слитных кусков
  const camelJoins = (text.match(/[a-z][A-Z]/g) ?? []).length;
  return camelJoins >= 4 && spaceRatio < 0.12;
}

/**
 * Prefers a clean Figure/Table caption when present; otherwise truncates
 * garbled extraction. Always safe for UI (no layout-busting runs).
 */
export function normalizeSelectedQuote(raw: string): string {
  const text = raw.replace(/\s+/g, ' ').trim();
  if (!text) {
    return '';
  }

  const figureMatch = text.match(FIGURE_RE);
  if (figureMatch) {
    const start = Math.max(0, figureMatch.index ?? 0);
    let snippet = text.slice(start);
    // Drop leading garbage before Figure if somehow still there
    if (start > 0 && looksLikeGarbage(text.slice(0, start))) {
      snippet = text.slice(start);
    }
    // Keep caption after Figure… until ~140 chars or obvious junk
    const junkStart = snippet.search(/\b(?:[A-Z][a-z]+){4,}/);
    if (junkStart > 20) {
      snippet = snippet.slice(0, junkStart).trim();
    }
    if (snippet.length > MAX_QUOTE_LENGTH) {
      snippet = `${snippet.slice(0, MAX_QUOTE_LENGTH - 1).trim()}…`;
    }
    return softBreakLongTokens(snippet);
  }

  if (looksLikeGarbage(text)) {
    // Нет «Figure N» — короткая подпись, чтобы не хранить мусор
    return 'Изображение / рисунок на странице';
  }

  let cleaned = text;
  if (cleaned.length > MAX_QUOTE_LENGTH) {
    cleaned = `${cleaned.slice(0, MAX_QUOTE_LENGTH - 1).trim()}…`;
  }
  return softBreakLongTokens(cleaned);
}
