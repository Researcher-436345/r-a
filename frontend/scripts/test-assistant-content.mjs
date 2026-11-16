import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { after, test } from 'node:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import ts from 'typescript';
import { createServer } from 'vite';

async function loadHelper(path) {
  const source = await readFile(new URL(path, import.meta.url), 'utf8');
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
  });

  return import(`data:text/javascript;base64,${Buffer.from(compiled.outputText).toString('base64')}`);
}

const { normalizeMathMarkdown, stripSourcesFooter } = await loadHelper('../src/shared/lib/markdown.ts');
const { locateQuote, findQuotePage } = await loadHelper('../src/features/reader/quote-location.ts');
const vite = await createServer({
  configFile: false,
  esbuild: { jsx: 'automatic', jsxImportSource: 'react' },
  server: { middlewareMode: true },
  appType: 'custom',
});
after(() => vite.close());
const { MarkdownMessage } = await vite.ssrLoadModule('/src/features/chat/components/markdown-message.tsx');
const { RichText, linkifyPageCites } = await vite.ssrLoadModule('/src/shared/ui/rich-text.tsx');
// The page imports the browser API URL at module load; these SSR tests make no requests.
globalThis.window = { location: new URL('http://localhost:5180') };
let PendingResearchMessage;
try {
  ({ PendingResearchMessage } = await vite.ssrLoadModule('/src/pages/chat/chat-page.tsx'));
} finally {
  delete globalThis.window;
}

test('chat renders both TeX delimiter styles and dollar math as KaTeX', () => {
  const content = String.raw`Noise \(z\), sample $x_1$.

\[x_t=\alpha_t z+\beta_t x_1\]`;
  const html = renderToStaticMarkup(createElement(MarkdownMessage, { content }));
  assert.equal((html.match(/<math /g) ?? []).length, 3);
  assert.match(html, /class="katex-display"/);
  assert.doesNotMatch(html, /katex-error/);
});

test('normalizing math preserves inline, fenced and unfinished code', () => {
  const code = '`\\(x\\)`\n\n```python\nvalue = "\\[x\\]"\n```\n';
  assert.equal(normalizeMathMarkdown(code), code);
  assert.equal(normalizeMathMarkdown('~~~text\n\\(x\\)\n~~~~\n\\(y\\)'), '~~~text\n\\(x\\)\n~~~~\n$y$');
  assert.equal(normalizeMathMarkdown('```text\n\\(x\\)'), '```text\n\\(x\\)');
});

test('article chat and overview render display equations without consuming TeX escapes', () => {
  const content = String.raw`\[x_t=\alpha_t z+\beta_t x_1\]

where \(z\) is noise and \(\alpha_t,\beta_t\) are the schedule.`;
  const html = renderToStaticMarkup(createElement(RichText, null, content));
  assert.equal((html.match(/<math /g) ?? []).length, 3);
  assert.match(html, /class="katex-display"/);
  assert.doesNotMatch(html, /katex-error/);
});

test('page citation chips preserve the entire quote in the navigation target', () => {
  const quote = 'delay, avoid, or abandon health behaviors';
  const source = linkifyPageCites(`Evidence [p.1 «${quote}»], then [стр. 2].`);
  assert.match(source, /#cite-page-1\?q=delay%2C\+avoid%2C\+or\+abandon\+health\+behaviors/);
  const html = renderToStaticMarkup(createElement(RichText, { onPageCite: () => {} }, `Evidence [p.1 «${quote}»].`));
  assert.match(html, /class="rich-text__page-cite"/);
  assert.match(html, new RegExp(`title="${quote}"`));
});

test('page numbers without a passage never become clickable citation chips', () => {
  const html = renderToStaticMarkup(createElement(RichText, { onPageCite: () => {} },
    'See [p.1], [[p:2]], and [page](#cite-page-3).'));
  assert.doesNotMatch(html, /rich-text__page-cite|<button|href="#cite-page/);
  assert.match(html, /стр\. 1/);
  assert.match(html, /стр\. 2/);
});

test('deep research elapsed time survives remounting and resets only for a new request', (t) => {
  const startedAt = Date.parse('2026-11-16T17:00:00Z');
  let now = startedAt + 83_000;
  t.mock.method(Date, 'now', () => now);
  const renderPending = (start) => renderToStaticMarkup(createElement(PendingResearchMessage, {
    locale: 'ru', mode: 'deep', startedAt: start,
  }));
  assert.match(renderPending(startedAt), /1:23/);
  now += 120_000;
  assert.match(renderPending(startedAt), /3:23/);
  assert.match(renderPending(now), /0:00/);
});

test('a recovered research uses the server start time on the first render', (t) => {
  t.mock.method(Date, 'now', () => Date.parse('2026-11-16T17:04:10Z'));
  const html = renderToStaticMarkup(createElement(PendingResearchMessage, {
    locale: 'ru', mode: 'deep', startedAt: Date.parse('2026-11-16T17:00:00Z'),
  }));
  assert.match(html, /4:10/);
});

test('math support preserves tables, custom article links and code', () => {
  const content = '| Formula | Paper |\n| --- | --- |\n| $x^2$ | [Paper](https://arxiv.org/abs/1706.03762) |\n\n`\\(code\\)`';
  const html = renderToStaticMarkup(createElement(MarkdownMessage, {
    content,
    renderLink: ({ href, children }) => createElement('a', { href, 'data-paper': true }, children),
  }));
  assert.match(html, /chat-markdown__table-wrap/);
  assert.match(html, /data-paper="true"/);
  assert.match(html, /<code>\\\(code\\\)<\/code>/);
  assert.equal((html.match(/<math /g) ?? []).length, 1);
});

test('old generated source appendices disappear while inline links remain', () => {
  const answer = 'See [Paper](https://arxiv.org/abs/1706.03762).';
  assert.equal(stripSourcesFooter(`${answer}\n\n## Sources\n\n1. [Paper](https://arxiv.org/abs/1706.03762)`), answer);
  assert.equal(stripSourcesFooter(`${answer}\n\n## Sources\n\n1\\. [One](https://example.org/one)\n\n3\\. [Three](https://example.org/three)`), answer);
  assert.equal(stripSourcesFooter(`${answer}\n\n## Источники\n\n1. [Paper](https://example.org/paper)`), answer);
});

test('requested tables and sections with substantive prose remain intact', () => {
  for (const answer of [
    '## Sources\n\n| Paper | Result |\n| --- | --- |\n| [Paper](https://example.org/paper) | 42 |',
    '## Sources\n\n1. [Paper](https://example.org/paper) — explains the method.',
    '## Sources\n\n1. [Paper](https://example.org/paper)',
    '## Sources\n\n1. [Paper](https://example.org/paper)\n\n## Conclusion\nUseful result.',
  ]) {
    assert.equal(stripSourcesFooter(answer), answer);
  }
});

test('citation lookup spans PDF text nodes and ignores line breaks and hyphenation', () => {
  const texts = ['Intro. delay, avoid, or aban-', 'don health behav', 'iors. More text.'];
  const match = locateQuote(texts, 'delay, avoid, or abandon health behaviors');
  assert.deepEqual(match, { start: { index: 0, offset: 7 }, end: { index: 2, offset: 4 } });
});

test('citation offsets survive ligatures, curly punctuation and Unicode', () => {
  assert.deepEqual(locateQuote(['An eﬀective “model”'], 'effective model'), {
    start: { index: 0, offset: 3 }, end: { index: 0, offset: 18 },
  });
  assert.deepEqual(locateQuote(['😀 Важный ', 'ре-зультат.'], 'важный результат'), {
    start: { index: 0, offset: 3 }, end: { index: 1, offset: 10 },
  });
  assert.equal(locateQuote(['Different text'], 'missing phrase'), null);
  assert.equal(locateQuote(['Different text'], '…'), null);
  assert.deepEqual(locateQuote(['An e\u0301tude'], 'étude'), {
    start: { index: 0, offset: 3 }, end: { index: 0, offset: 9 },
  });
});

test('PDF citation resolves the actual passage when its page label is wrong', async () => {
  const visited = [];
  const document = {
    numPages: 3,
    async getPage(page) {
      visited.push(page);
      return { async getTextContent() {
        return { items: [{ str: page === 3 ? 'delay, avoid, or aban-' : 'Different content' }, { str: 'don health behaviors' }] };
      } };
    },
  };
  const page = await findQuotePage(document, 2, 'delay, avoid, or abandon health behaviors', new AbortController().signal);
  assert.equal(page, 3);
  assert.deepEqual(visited, [2, 1, 3]);
});

test('PDF citation never falls back to an unverified page', async () => {
  const document = {
    numPages: 1,
    async getPage() { return { async getTextContent() { return { items: [{ str: 'Different content' }] }; } }; },
  };
  assert.equal(await findQuotePage(document, 1, 'missing phrase', new AbortController().signal), null);
  const cancelled = new AbortController();
  cancelled.abort();
  assert.equal(await findQuotePage({ numPages: 1, getPage() { throw Error('Must not load a cancelled citation'); } }, 1, 'phrase', cancelled.signal), null);
});
