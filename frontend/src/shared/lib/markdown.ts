function normalizeTextMath(text: string): string {
  return text
    .replace(/\\\(([\s\S]*?)\\\)/g, (_match, formula: string) => `$${formula.trim()}$`)
    .replace(/\\\[([\s\S]*?)\\\]/g, (_match, formula: string) => `\n$$\n${formula.trim()}\n$$\n`);
}

/** Convert TeX delimiters before Markdown consumes their backslashes; leave code verbatim. */
export function normalizeMathMarkdown(input: string): string {
  const text = input.replace(/\r\n/g, '\n');
  const code = /^ {0,3}(`{3,}|~{3,})[^\n]*(?:\n|$)|(`+)[\s\S]*?\2(?!`)/gm;
  let result = '';
  let offset = 0;
  let match: RegExpExecArray | null;

  while ((match = code.exec(text))) {
    result += normalizeTextMath(text.slice(offset, match.index));
    let end = code.lastIndex;

    if (match[1]) {
      const fence = match[1];
      const closingFence = new RegExp(`^ {0,3}${fence[0]}{${fence.length},}[ \\t]*$`, 'gm');
      closingFence.lastIndex = end;
      const closing = closingFence.exec(text);
      end = closing ? closingFence.lastIndex : text.length;
    }

    result += text.slice(match.index, end);
    offset = end;
    code.lastIndex = end;
  }

  return result + normalizeTextMath(text.slice(offset));
}

/** Hide the link-only appendix added by older search responses, including copied Markdown. */
export function stripSourcesFooter(content: string): string {
  const heading = /(?:^|\n)## (?:Sources|Источники)[ \t]*\r?\n/gi;
  let match: RegExpExecArray | null;

  while ((match = heading.exec(content))) {
    const lines = content.slice(heading.lastIndex).trim().split(/\r?\n/).filter((line) => line.trim());
    const onlySourceLinks = lines.length > 0 && lines.every((line) =>
      /^\d+\\?\. \[[^\]\n]+\]\(https?:\/\/[^\s]+\)[ \t]*$/.test(line),
    );

    if (onlySourceLinks && content.slice(0, match.index).trim()) {
      return content.slice(0, match.index).trimEnd();
    }
  }

  return content;
}
