import { ApiError, apiRequest } from '../../shared/api/client';
import { getAccessToken } from '../auth/token-storage';
import { isPrefetchablePaperUrl } from '../papers/link-kind';

export type ReadingStatus = 'unread' | 'reading' | 'read';
export type PaperVersionStatus = 'uploading' | 'processing' | 'ready' | 'failed';

export interface LibraryPaper {
  id: string;
  title: string;
  abstract: string | null;
  year: number | null;
  venue: string | null;
  doi: string | null;
  arxiv_id: string | null;
  authors: Array<{ id: string; name: string }>;
  latest_version: {
    id: string;
    source: string;
    status: PaperVersionStatus;
    pdf_key: string | null;
    size_bytes: number | null;
    error_message: string | null;
  } | null;
  /** Parsed or open-access full text is stored (the overview and assistant read it). */
  has_full_text?: boolean;
  created_at: string;
}

export interface LibraryItem {
  id: string;
  status: ReadingStatus;
  favorite: boolean;
  folder_id: string | null;
  added_at: string;
  paper: LibraryPaper;
}

export type LibrarySystemFolder = 'want_to_read' | 'reading' | 'other';

export interface LibraryFolder {
  id: string;
  name: string;
  parent_id: string | null;
  system_key: LibrarySystemFolder | null;
  article_count: number;
  created_at: string;
}

export interface LibraryListResponse {
  items: LibraryItem[];
  page: number;
  limit: number;
  total: number;
}

function authToken() {
  return getAccessToken();
}

export function fetchLibrary(
  page = 1,
  limit = 100,
  folderId?: string,
): Promise<LibraryListResponse> {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) });
  if (folderId) {
    params.set('folder_id', folderId);
  }
  return apiRequest<LibraryListResponse>(`/library?${params.toString()}`, {
    token: authToken(),
  });
}

export function fetchLibraryFolders(): Promise<{ items: LibraryFolder[] }> {
  return apiRequest<{ items: LibraryFolder[] }>('/library/folders', {
    token: authToken(),
  });
}

export function fetchLibraryItem(paperId: string): Promise<LibraryItem> {
  return apiRequest<LibraryItem>(`/library/${paperId}`, {
    token: authToken(),
  });
}

export function saveToLibraryFolder(paperId: string, folderId: string): Promise<LibraryItem> {
  return apiRequest<LibraryItem>(`/library/${paperId}`, {
    method: 'POST',
    token: authToken(),
    body: { folder_id: folderId },
  });
}

export function createLibraryFolder(
  name: string,
  parentId: string | null,
): Promise<LibraryFolder> {
  return apiRequest<LibraryFolder>('/library/folders', {
    method: 'POST',
    token: authToken(),
    body: { name, parent_id: parentId },
  });
}

export function deleteLibraryFolder(folderId: string): Promise<void> {
  return apiRequest<void>(`/library/folders/${folderId}`, {
    method: 'DELETE',
    token: authToken(),
  });
}

export function addByArxiv(arxivId: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>('/papers/arxiv', {
    method: 'POST',
    token: authToken(),
    body: { arxiv_id: arxivId },
  });
}

const openByArxivRequests = new Map<string, Promise<LibraryPaper>>();
const arxivPrefetchQueue: string[] = [];
const queuedArxivIds = new Set<string>();
const MAX_CONCURRENT_ARXIV_PREFETCHES = 3;
let activeArxivPrefetches = 0;

/** Открывает публичную arXiv-статью, не добавляя её в список чтения. */
export function openByArxiv(arxivId: string): Promise<LibraryPaper> {
  const key = arxivId.trim();
  const cached = openByArxivRequests.get(key);
  if (cached) {
    return cached;
  }

  const request = apiRequest<LibraryPaper>('/papers/arxiv', {
    method: 'POST',
    token: authToken(),
    body: { arxiv_id: key, add_to_library: false },
  });

  openByArxivRequests.set(key, request);
  void request.catch(() => {
    if (openByArxivRequests.get(key) === request) {
      openByArxivRequests.delete(key);
    }
  });
  return request;
}

function drainArxivPrefetchQueue() {
  while (activeArxivPrefetches < MAX_CONCURRENT_ARXIV_PREFETCHES) {
    const arxivId = arxivPrefetchQueue.shift();
    if (!arxivId) {
      return;
    }
    queuedArxivIds.delete(arxivId);
    activeArxivPrefetches += 1;
    void openByArxiv(arxivId)
      .catch(() => {
        // Фоновая подготовка не должна показывать ошибку пользователю.
        // Обычный клик повторит неуспешный запрос.
      })
      .finally(() => {
        activeArxivPrefetches -= 1;
        drainArxivPrefetchQueue();
      });
  }
}

/** Ставит публичную статью в ограниченную фоновую очередь подготовки reader. */
export function prefetchArxiv(arxivId: string) {
  const key = arxivId.trim();
  if (!key || openByArxivRequests.has(key) || queuedArxivIds.has(key)) {
    return;
  }
  queuedArxivIds.add(key);
  arxivPrefetchQueue.push(key);
  window.setTimeout(drainArxivPrefetchQueue, 0);
}

export function addByDoi(doi: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>('/papers/doi', {
    method: 'POST',
    token: authToken(),
    body: { doi },
  });
}

// Keyed by URL and title: the backend opens the paper the link text names when
// the model paired it with a wrong arXiv id, so one URL can mean two papers.
const openByUrlRequests = new Map<string, Promise<LibraryPaper>>();
const urlPrefetchQueue: Array<{ key: string; url: string; titleHint: string }> = [];
const queuedPrefetchKeys = new Set<string>();
// Prefetch never retries a failed link by itself; opening the link does.
const failedPrefetchKeys = new Set<string>();
// Resolving a web link can take 15+ s; an answer with 30 links must not fire 30 at once.
const MAX_CONCURRENT_URL_PREFETCHES = 2;
let activeUrlPrefetches = 0;

function openKey(url: string, titleHint?: string) {
  return `${url.trim()}\n${(titleHint ?? '').trim().toLowerCase()}`;
}

// 422 not_found / not_a_paper answers the server would repeat: /open shows them
// at once instead of another 15 s wait, and reloads do not re-send them.
const OPEN_FAILURE_TTL_MS = 10 * 60_000;
const OPEN_FAILURE_STORAGE_KEY = 'odyssey.openFailures';
type OpenFailure = { code: string; detail: string; at: number };
const openFailures = new Map<string, OpenFailure>(readStoredOpenFailures());

function readStoredOpenFailures(): Array<[string, OpenFailure]> {
  try {
    const raw = window.sessionStorage.getItem(OPEN_FAILURE_STORAGE_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    const now = Date.now();
    return Array.isArray(parsed)
      ? (parsed as Array<[string, OpenFailure]>).filter(
          (entry) => Array.isArray(entry) && entry[1] && now - entry[1].at < OPEN_FAILURE_TTL_MS,
        )
      : [];
  } catch {
    return [];
  }
}

function storeOpenFailures() {
  try {
    window.sessionStorage.setItem(
      OPEN_FAILURE_STORAGE_KEY,
      JSON.stringify([...openFailures].slice(-200)),
    );
  } catch {
    // Private mode or blocked storage: the in-memory copy still works.
  }
}

function rememberOpenFailure(key: string, error: unknown) {
  if (
    error instanceof ApiError &&
    error.status === 422 &&
    (error.code === 'not_found' || error.code === 'not_a_paper')
  ) {
    openFailures.set(key, { code: error.code, detail: error.detail, at: Date.now() });
    storeOpenFailures();
  }
}

/** The remembered 422 for this link, if resolving it recently found nothing. */
export function cachedOpenFailure(url: string, titleHint?: string): ApiError | null {
  const key = openKey(url, titleHint);
  const failure = openFailures.get(key);
  if (!failure) {
    return null;
  }
  if (Date.now() - failure.at > OPEN_FAILURE_TTL_MS) {
    openFailures.delete(key);
    storeOpenFailures();
    return null;
  }
  return new ApiError(422, failure.detail, failure.code);
}

export async function addByUrl(url: string, titleHint?: string): Promise<LibraryPaper> {
  const paper = await apiRequest<LibraryPaper>('/papers/from-url', {
    method: 'POST',
    token: authToken(),
    body: { url, title_hint: titleHint ?? '', add_to_library: true },
  });
  // The same paper answers a later /open of this link without another resolve.
  const key = openKey(url, titleHint);
  if (!openByUrlRequests.has(key)) {
    openByUrlRequests.set(key, Promise.resolve(paper));
  }
  return paper;
}

/**
 * Готовит web-ссылку для reader, не добавляя её в библиотеку. Параллельные вызовы делят один запрос.
 * force: the user asked to try again, so neither a remembered miss nor a cached request is reused.
 */
export function openByUrl(
  url: string,
  titleHint?: string,
  { force = false }: { force?: boolean } = {},
): Promise<LibraryPaper> {
  const key = openKey(url, titleHint);
  if (!force) {
    const cached = openByUrlRequests.get(key);
    if (cached) {
      return cached;
    }
    const failure = cachedOpenFailure(url, titleHint);
    if (failure) {
      return Promise.reject(failure);
    }
  }

  const request = apiRequest<LibraryPaper>('/papers/from-url', {
    method: 'POST',
    token: authToken(),
    body: { url: url.trim(), title_hint: titleHint ?? '', add_to_library: false },
  });
  openByUrlRequests.set(key, request);
  void request
    .then(() => {
      failedPrefetchKeys.delete(key);
      if (openFailures.delete(key)) {
        storeOpenFailures();
      }
    })
    .catch((error: unknown) => {
      rememberOpenFailure(key, error);
      if (openByUrlRequests.get(key) === request) {
        openByUrlRequests.delete(key);
      }
    });
  return request;
}

function drainUrlPrefetchQueue() {
  while (activeUrlPrefetches < MAX_CONCURRENT_URL_PREFETCHES) {
    const next = urlPrefetchQueue.shift();
    if (!next) {
      return;
    }
    queuedPrefetchKeys.delete(next.key);
    if (openByUrlRequests.has(next.key)) {
      // Already opened by a click or added to a folder meanwhile.
      continue;
    }
    activeUrlPrefetches += 1;
    void openByUrl(next.url, next.titleHint)
      .catch(() => {
        failedPrefetchKeys.add(next.key);
      })
      .finally(() => {
        activeUrlPrefetches -= 1;
        drainUrlPrefetchQueue();
      });
  }
}

/** Ставит ссылку в ограниченную фоновую очередь подготовки, когда она попала во viewport. */
export function prefetchUrl(url: string, titleHint?: string) {
  const trimmed = url.trim();
  const key = openKey(trimmed, titleHint);
  if (
    !trimmed ||
    !isPrefetchablePaperUrl(trimmed) ||
    openByUrlRequests.has(key) ||
    queuedPrefetchKeys.has(key) ||
    failedPrefetchKeys.has(key) ||
    cachedOpenFailure(trimmed, titleHint)
  ) {
    return;
  }
  queuedPrefetchKeys.add(key);
  urlPrefetchQueue.push({ key, url: trimmed, titleHint: titleHint ?? '' });
  window.setTimeout(drainUrlPrefetchQueue, 0);
}

export async function uploadPdf(file: File): Promise<LibraryPaper> {
  const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';
  const form = new FormData();
  form.append('file', file);

  let response: Response;
  try {
    response = await fetch(`${API_URL}/papers/upload`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${authToken() ?? ''}`,
      },
      body: form,
    });
  } catch {
    throw new Error(
      'Не удалось связаться с API. Проверь, что backend запущен: docker compose up и VITE_API_URL=http://localhost:8080',
    );
  }

  if (!response.ok) {
    let detail = `Upload failed with status ${response.status}`;
    try {
      const data = (await response.json()) as { detail?: string };
      if (data.detail) {
        detail = data.detail;
      }
    } catch {
      // ignore
    }
    throw new Error(detail);
  }

  return (await response.json()) as LibraryPaper;
}

export function fetchPaper(paperId: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>(`/papers/${paperId}`, {
    token: authToken(),
  });
}

// PDF loading and the B3 pdf-url status codes live in features/reader/pdf-status.ts.

export function retryPdf(paperId: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>(`/papers/${paperId}/retry-pdf`, {
    method: 'POST',
    token: authToken(),
  });
}

export function removeFromLibrary(paperId: string): Promise<void> {
  return apiRequest<void>(`/library/${paperId}`, {
    method: 'DELETE',
    token: authToken(),
  });
}

export function patchLibraryItem(
  paperId: string,
  body: { status?: ReadingStatus; favorite?: boolean; folder_id?: string },
): Promise<LibraryItem> {
  return apiRequest<LibraryItem>(`/library/${paperId}`, {
    method: 'PATCH',
    token: authToken(),
    body,
  });
}
