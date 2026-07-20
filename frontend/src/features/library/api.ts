import { ApiError, apiRequest } from '../../shared/api/client';
import { getAccessToken } from '../auth/token-storage';

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
  created_at: string;
}

export interface LibraryItem {
  id: string;
  status: ReadingStatus;
  favorite: boolean;
  added_at: string;
  paper: LibraryPaper;
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

export function fetchLibrary(page = 1, limit = 20): Promise<LibraryListResponse> {
  return apiRequest<LibraryListResponse>(`/library?page=${page}&limit=${limit}`, {
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

export function addByDoi(doi: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>('/papers/doi', {
    method: 'POST',
    token: authToken(),
    body: { doi },
  });
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

export function fetchPdfUrl(
  paperId: string,
): Promise<{ url: string; expires_in: number; status: string; source: string }> {
  return apiRequest<{ url: string; expires_in: number; status: string; source: string }>(
    `/papers/${paperId}/pdf-url`,
    {
      token: authToken(),
    },
  );
}

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

/** Скачивает PDF через API (не через MinIO напрямую) и отдаёт blob URL для PDF.js. */
export async function fetchPdfObjectUrl(paperId: string): Promise<string> {
  const token = authToken();
  const response = await fetch(`${API_URL}/papers/${paperId}/pdf`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) {
    let detail = `HTTP ${response.status}`;
    try {
      const data = (await response.json()) as { detail?: string };
      if (data.detail) {
        detail = data.detail;
      }
    } catch {
      // ignore
    }
    throw new ApiError(response.status, detail);
  }
  const blob = await response.blob();
  return URL.createObjectURL(blob);
}

export function fetchPaper(paperId: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>(`/papers/${paperId}`, {
    token: authToken(),
  });
}

/** Ждём, пока worker положит PDF в MinIO, затем отдаём blob URL. */
export async function waitForPdfUrl(
  paperId: string,
  {
    attempts = 40,
    delayMs = 750,
  }: { attempts?: number; delayMs?: number } = {},
): Promise<{ url: string; expires_in: number; status: string; source: string }> {
  let lastError: unknown;
  for (let i = 0; i < attempts; i += 1) {
    try {
      await fetchPdfUrl(paperId);
      const objectUrl = await fetchPdfObjectUrl(paperId);
      return { url: objectUrl, expires_in: 0, status: 'ready', source: 'api' };
    } catch (err) {
      lastError = err;
      const detail = err instanceof ApiError ? err.detail : '';
      const stillProcessing =
        err instanceof ApiError &&
        (err.status === 409 || detail.toLowerCase().includes('processing'));
      if (!stillProcessing && err instanceof ApiError && err.status !== 404) {
        throw err;
      }
      await new Promise((resolve) => {
        window.setTimeout(resolve, delayMs);
      });
    }
  }
  throw lastError instanceof Error ? lastError : new Error('PDF не готов');
}

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
  body: { status?: ReadingStatus; favorite?: boolean },
): Promise<LibraryItem> {
  return apiRequest<LibraryItem>(`/library/${paperId}`, {
    method: 'PATCH',
    token: authToken(),
    body,
  });
}
