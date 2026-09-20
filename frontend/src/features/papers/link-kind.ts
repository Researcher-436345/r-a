import { isValidElement, type ReactNode } from 'react';

/*
 * URL classes that are never a single paper (index, listing, home, help and
 * account pages). The not_a_paper check of POST /papers/from-url in
 * backend/internal/modules/catalog/indexpages.go is authoritative; this copy
 * only lets the chat render such links as explicit external links without a
 * round trip. services/websearch/app/main.py (is_non_paper_url) keeps the same
 * list: keep all three in sync. This copy and websearch are deliberately
 * broader in two places (any site root, /vol/N on any host).
 */
const NON_PAPER_HOSTS = new Set([
  'info.arxiv.org',
  'blog.arxiv.org',
  'status.arxiv.org',
  'search.arxiv.org',
  'labs.arxiv.org',
  'confluence.arxiv.org',
]);

// Scholarly sites whose search, login and help pages are portals.
const SCHOLARLY_HOSTS = new Set([
  'arxiv.org', 'export.arxiv.org', 'ar5iv.labs.arxiv.org', 'ar5iv.org',
  'openreview.net', 'semanticscholar.org', 'sciencedirect.com',
  'aclanthology.org', 'proceedings.neurips.cc', 'papers.nips.cc', 'papers.neurips.cc',
  'openaccess.thecvf.com', 'proceedings.mlr.press', 'jmlr.org',
  'dl.acm.org', 'ieeexplore.ieee.org', 'link.springer.com', 'springer.com',
  'nature.com', 'science.org', 'cell.com', 'pnas.org',
  'pubmed.ncbi.nlm.nih.gov', 'ncbi.nlm.nih.gov', 'pmc.ncbi.nlm.nih.gov', 'europepmc.org',
  'researchgate.net', 'paperswithcode.com', 'doi.org', 'dx.doi.org',
  'openalex.org', 'crossref.org', 'biorxiv.org', 'medrxiv.org', 'ssrn.com',
  'papers.ssrn.com', 'jstor.org', 'onlinelibrary.wiley.com', 'tandfonline.com',
  'mdpi.com', 'frontiersin.org', 'journals.plos.org', 'hindawi.com',
  'ojs.aaai.org', 'ijcai.org', 'dblp.org', 'cyberleninka.ru', 'elibrary.ru',
]);

const PORTAL_SECTIONS = new Set([
  'search', 'login', 'logout', 'signin', 'sign-in', 'signup',
  'register', 'help', 'about', 'contact', 'faq', 'subscribe',
]);

const ARXIV_SECTIONS = new Set([
  'list', 'archive', 'catchup', 'year', 'search', 'a', 'login', 'logout', 'help',
  'user', 'auth', 'register', 'stats', 'submit', 'localization', 'corr', 'rss',
  'new', 'multi', 'institutional_banner', 'ignoreme',
]);

// First path segment that is a listing on hosts whose papers live elsewhere.
const NON_PAPER_SECTIONS: Record<string, Set<string>> = {
  'arxiv.org': ARXIV_SECTIONS,
  'export.arxiv.org': ARXIV_SECTIONS,
  'openreview.net': new Set([
    'venues', 'venue', 'group', 'profile', 'search', 'login', 'signup', 'about',
    'tasks', 'activity', 'messages', 'sponsors', 'legal', 'invitation',
  ]),
  'semanticscholar.org': new Set([
    'topic', 'search', 'product', 'author', 'venue', 'about', 'faq', 'me', 'sign-in',
    'alerts', 'feed', 'library', 'api', 'research', 'cord19', 'faqs',
  ]),
  'sciencedirect.com': new Set(['journal', 'browse', 'search', 'topics', 'user']),
  'aclanthology.org': new Set([
    'events', 'venues', 'volumes', 'people', 'search', 'sigs', 'faq', 'info', 'posts',
  ]),
};

// Whole-year pages ("Advances in Neural Information Processing Systems 36"),
// book, author and admin pages.
const NEURIPS_INDEX_RE =
  /^(?:\/paper_files)?(?:\/paper)?(?:\/\d{4})?$|^\/(?:book|author|admin)(?:\/|$)/i;

const NON_PAPER_PATHS: Record<string, RegExp> = {
  'proceedings.neurips.cc': NEURIPS_INDEX_RE,
  'papers.nips.cc': NEURIPS_INDEX_RE,
  'papers.neurips.cc': NEURIPS_INDEX_RE,
  'proceedings.mlr.press': /^\/v\d+$/i,
  'jmlr.org': /^\/papers(?:\/v\d+)?$/i,
};

// ar5iv serves papers only under these sections.
const AR5IV_HOSTS = new Set(['ar5iv.labs.arxiv.org', 'ar5iv.org']);
const AR5IV_PAPER_SECTIONS = new Set(['html', 'abs', 'pdf']);

// Journal volume/issue tables of contents on any publisher.
const VOLUME_PATH_RE = /\/vol\/\d+(\/|$)/i;
const ROOT_PATHS = new Set(['', '/index.html', '/index.htm', '/index.php']);

/** True for links that list, search or introduce papers instead of being one. */
export function isNonPaperUrl(url: string): boolean {
  let parsed: URL;
  try {
    parsed = new URL(url.trim());
  } catch {
    return false;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return false;
  }
  const host = parsed.hostname.toLowerCase().replace(/^www\./, '');
  if (!host) {
    return false;
  }
  const path = parsed.pathname.replace(/\/+$/, '');
  if (
    NON_PAPER_HOSTS.has(host) ||
    host.startsWith('scholar.google.') ||
    ROOT_PATHS.has(path.toLowerCase())
  ) {
    return true;
  }
  const section = path.replace(/^\/+/, '').split('/', 1)[0].toLowerCase();
  if (SCHOLARLY_HOSTS.has(host) && PORTAL_SECTIONS.has(section)) {
    return true;
  }
  if (NON_PAPER_SECTIONS[host]?.has(section)) {
    return true;
  }
  if (NON_PAPER_PATHS[host]?.test(path)) {
    return true;
  }
  if (AR5IV_HOSTS.has(host) && !AR5IV_PAPER_SECTIONS.has(section)) {
    return true;
  }
  // CVF papers all live under /content*; the rest are menus and day indexes.
  if (host === 'openaccess.thecvf.com' && !section.startsWith('content')) {
    return true;
  }
  return VOLUME_PATH_RE.test(path);
}

function isScholarlyHost(hostname: string): boolean {
  const host = hostname.toLowerCase().replace(/^www\./, '');
  if (SCHOLARLY_HOSTS.has(host)) {
    return true;
  }
  for (const known of SCHOLARLY_HOSTS) {
    if (host.endsWith(`.${known}`)) {
      return true;
    }
  }
  return false;
}

/**
 * Links worth resolving in the background: known scholarly sites and direct
 * PDFs. Blogs, wikis and course pages still open through /open on click, but
 * warming them costs the server a page fetch that rarely finds a paper.
 */
export function isPrefetchablePaperUrl(url: string): boolean {
  let parsed: URL;
  try {
    parsed = new URL(url.trim());
  } catch {
    return false;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return false;
  }
  return /\.pdf$/i.test(parsed.pathname) || isScholarlyHost(parsed.hostname);
}

const URL_LIKE_RE =/^(?:[a-z][a-z0-9+.-]*:\/\/|www\.)\S*$|^[\w-]+(?:\.[\w-]+)*\.[a-z]{2,}(?::\d+)?\/\S*$/i;
const ARXIV_ID = String.raw`(?:arxiv:\s*)?\d{4}\.\d{4,5}(?:v\d+)?`;
const LEADING_TAG_RE = /^\[(?:pdf|html|book|citation)\]\s*/i;
const LEADING_FORMAT_RE = /^(?:pdf|html)\s+/i;
const LEADING_ARXIV_RE = new RegExp(String.raw`^[\[(]?${ARXIV_ID}[\])]?\s*[:–—-]?\s*`, 'i');
const TRAILING_ARXIV_RE = new RegExp(String.raw`\s*[\[(]?${ARXIV_ID}[\])]?$`, 'i');
const LEADING_DOI_RE = /^(?:doi:?\s*)?10\.\d{4,9}\/\S+\s*/i;
const TRAILING_DOI_RE = /(?:^|\s+)(?:doi:?\s*)?10\.\d{4,9}\/\S+$/i;
const TRAILING_FORMAT_RE = /\s*(?:\[(?:pdf|html)\]|\((?:pdf|html)\)|\s[|–—-]\s*(?:pdf|html))$/i;
// Site names search engines append to page titles.
const SITE_SUFFIX_RE =
  /\s+[|/–—-]\s*(?:semantic\s+scholar|arxiv(?:\.org)?|ar5iv|sciencedirect(?:\.com)?|openreview(?:\.net)?|researchgate|springer(?:link)?|ieee\s+xplore|acm\s+digital\s+library|pubmed|europe\s+pmc|pmc|google\s+scholar|хабр|habr(?:\.com)?)\s*$/i;
const TRAILING_ELLIPSIS_RE = /\s*(?:\.{3,}|…)$/;
const EDGE_PUNCTUATION_RE = /^[\s|:;,/–—-]+|[\s|:;,/–—-]+$/g;
const WRAPPING_QUOTES_RE = /^["'«“„](.*)["'»”“]$/;
// Link labels that name a format or site rather than a work.
const GENERIC_LABEL_RE =
  /^(?:pdf|html|arxiv|ar5iv|doi|link|source|paper|abstract|full\s+text|here|openreview|semantic\s+scholar|sciencedirect|pubmed|ссылка|источник|статья|здесь|полный\s+текст)$/i;
// "CVPRW 2022 PDF", "NeurIPS 2023": a venue badge, not a title.
const VENUE_LABEL_RE = /^[a-z][a-z&-]{1,15}\s+\d{4}(?:\s+(?:pdf|html|paper))?$/i;

/**
 * Turns a link label or search-result title into a title hint for the
 * resolver. Returns '' when nothing title-like is left (URLs, ids, numbers).
 */
export function cleanPaperTitle(raw: string): string {
  let value = (raw ?? '').replace(/\s+/g, ' ').trim();
  if (!value || URL_LIKE_RE.test(value)) {
    return '';
  }

  for (let pass = 0; pass < 6; pass += 1) {
    const before = value;
    value = value
      .replace(LEADING_TAG_RE, '')
      .replace(LEADING_FORMAT_RE, '')
      .replace(LEADING_ARXIV_RE, '')
      .replace(LEADING_DOI_RE, '')
      .replace(SITE_SUFFIX_RE, '')
      .replace(TRAILING_ELLIPSIS_RE, '')
      .replace(TRAILING_FORMAT_RE, '')
      .replace(TRAILING_ARXIV_RE, '')
      .replace(TRAILING_DOI_RE, '')
      .replace(EDGE_PUNCTUATION_RE, '')
      .replace(WRAPPING_QUOTES_RE, '$1')
      .trim();
    if (value === before) {
      break;
    }
  }

  if (
    !value ||
    !/\p{L}/u.test(value) ||
    URL_LIKE_RE.test(value) ||
    GENERIC_LABEL_RE.test(value) ||
    VENUE_LABEL_RE.test(value)
  ) {
    return '';
  }
  return value;
}

/** Plain text of rendered markdown children (link labels may contain <code>, <em>…). */
export function textFromChildren(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') {
    return '';
  }
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map((child: ReactNode) => textFromChildren(child)).join('');
  }
  if (isValidElement<{ children?: ReactNode }>(node)) {
    return textFromChildren(node.props.children);
  }
  return '';
}
