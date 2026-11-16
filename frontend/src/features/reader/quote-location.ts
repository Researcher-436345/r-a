interface TextPosition {
  index: number;
  offset: number;
}

interface QuoteMatch {
  start: TextPosition;
  end: TextPosition;
}

function searchableCharacters(text: string): string {
  return text.normalize('NFKC').toLowerCase().replace(/[^\p{L}\p{N}]/gu, '');
}

/** PDF line breaks, hyphenation and ligatures must not change the cited phrase. */
export function locateQuote(texts: readonly string[], quote: string): QuoteMatch | null {
  const needle = searchableCharacters(quote);

  if (!needle) {
    return null;
  }

  let haystack = '';
  const starts: TextPosition[] = [];
  const ends: TextPosition[] = [];

  texts.forEach((text, index) => {
    let offset = 0;

    for (const character of text) {
      const normalized = searchableCharacters(character);
      haystack += normalized;

      for (let unit = 0; unit < normalized.length; unit += 1) {
        starts.push({ index, offset });
        ends.push({ index, offset: offset + character.length });
      }

      offset += character.length;
    }
  });

  const start = haystack.indexOf(needle);

  if (start < 0) {
    return null;
  }

  return { start: starts[start], end: ends[start + needle.length - 1] };
}

/** Separate text-node ranges avoid highlighting whole marked-content containers. */
export function findQuoteRanges(textLayer: HTMLElement, quote: string): Range[] {
  const nodes: Text[] = [];
  const walker = document.createTreeWalker(textLayer, NodeFilter.SHOW_TEXT);
  let node = walker.nextNode();

  while (node) {
    nodes.push(node as Text);
    node = walker.nextNode();
  }

  const match = locateQuote(nodes.map((text) => text.data), quote);

  if (!match) {
    return [];
  }

  const ranges: Range[] = [];

  for (let index = match.start.index; index <= match.end.index; index += 1) {
    const text = nodes[index];
    const range = document.createRange();
    range.setStart(text, index === match.start.index ? match.start.offset : 0);
    range.setEnd(text, index === match.end.index ? match.end.offset : text.length);
    ranges.push(range);
  }

  return ranges;
}
