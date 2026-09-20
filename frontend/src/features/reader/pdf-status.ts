import { ApiError, apiFetch, apiRequest } from '../../shared/api/client';
import type { LibraryPaper } from '../library/api';

/** Client side of the catalog PDF contract (GET /papers/{id}/pdf-url, B3). */
export type ReaderPdfResult =
  | { kind: 'ready'; url: string }
  | { kind: 'unavailable'; detail: string }
  | { kind: 'timeout' }
  | { kind: 'error'; detail: string }
  | { kind: 'aborted' };

export type ReaderPdfPhase = 'loading' | 'processing';

interface LoadReaderPdfOptions {
  signal?: AbortSignal;
  onState?: (phase: ReaderPdfPhase) => void;
  /** Total time to wait for the worker, measured from the first request. */
  maxWaitMs?: number;
}

const DEFAULT_MAX_WAIT_MS = 120_000;
const FIRST_DELAY_MS = 1_000;
const MAX_DELAY_MS = 5_000;
const MAX_TRANSIENT_FAILURES = 3;

type Classified =
  | { kind: 'processing' }
  | { kind: 'unavailable'; detail: string }
  | { kind: 'transient'; detail: string }
  | { kind: 'error'; detail: string };

function isAbortError(err: unknown, signal?: AbortSignal) {
  return Boolean(signal?.aborted) || (err instanceof DOMException && err.name === 'AbortError');
}

function classify(err: unknown): Classified {
  if (!(err instanceof ApiError)) {
    // fetch() rejects with TypeError on network failures; worth a few retries.
    return { kind: 'transient', detail: err instanceof Error ? err.message : 'Network error' };
  }
  if (err.status === 409) {
    if (err.code === 'pdf_processing') {
      return { kind: 'processing' };
    }
    // Older backends answer 409 without a code both while processing and for a failed version.
    if (!err.code && /processing/i.test(err.detail) && !/failed/i.test(err.detail)) {
      return { kind: 'processing' };
    }
    return { kind: 'unavailable', detail: err.detail };
  }
  // 404 here means "no version at all": the page itself handles a missing paper.
  if (err.status === 422 || err.status === 404) {
    return { kind: 'unavailable', detail: err.detail };
  }
  if (err.status === 502 || err.status === 503 || err.status === 504) {
    return { kind: 'transient', detail: err.detail };
  }
  return { kind: 'error', detail: err.detail };
}

function sleep(ms: number, signal?: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Aborted', 'AbortError'));
      return;
    }
    const timer = window.setTimeout(() => {
      signal?.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      window.clearTimeout(timer);
      reject(new DOMException('Aborted', 'AbortError'));
    };
    signal?.addEventListener('abort', onAbort, { once: true });
  });
}

async function fetchPdfBlobUrl(paperId: string, signal?: AbortSignal) {
  await apiRequest(`/papers/${paperId}/pdf-url`, { signal, public: true });
  const response = await apiFetch(`/papers/${paperId}/pdf`, { signal, public: true });
  const blob = await response.blob();
  return URL.createObjectURL(blob);
}

/**
 * Waits for the paper's PDF and returns a blob URL for PDF.js.
 * Never rejects: every outcome, including abort, is a result kind.
 * The caller owns (and must revoke) the URL of a 'ready' result.
 */
export async function loadReaderPdf(
  paperId: string,
  { signal, onState, maxWaitMs = DEFAULT_MAX_WAIT_MS }: LoadReaderPdfOptions = {},
): Promise<ReaderPdfResult> {
  const deadline = Date.now() + maxWaitMs;
  let delay = FIRST_DELAY_MS;
  let phase: ReaderPdfPhase = 'loading';
  let transientFailures = 0;
  onState?.(phase);

  for (;;) {
    if (signal?.aborted) {
      return { kind: 'aborted' };
    }
    try {
      const url = await fetchPdfBlobUrl(paperId, signal);
      if (signal?.aborted) {
        URL.revokeObjectURL(url);
        return { kind: 'aborted' };
      }
      return { kind: 'ready', url };
    } catch (err) {
      if (isAbortError(err, signal)) {
        return { kind: 'aborted' };
      }
      const outcome = classify(err);
      if (outcome.kind === 'unavailable') {
        return { kind: 'unavailable', detail: outcome.detail };
      }
      if (outcome.kind === 'error') {
        return { kind: 'error', detail: outcome.detail };
      }
      if (outcome.kind === 'transient') {
        transientFailures += 1;
        if (transientFailures > MAX_TRANSIENT_FAILURES) {
          return { kind: 'error', detail: outcome.detail };
        }
      } else {
        transientFailures = 0;
        if (phase !== 'processing') {
          phase = 'processing';
          onState?.(phase);
        }
      }
    }

    const remaining = deadline - Date.now();
    if (remaining <= 0) {
      return { kind: 'timeout' };
    }
    try {
      await sleep(Math.min(delay, remaining), signal);
    } catch {
      return { kind: 'aborted' };
    }
    delay = Math.min(MAX_DELAY_MS, Math.round(delay * 1.5));
  }
}

/** POST /papers/{id}/find-fulltext (B4). Throws ApiError, 422 code "not_found" when nothing turned up. */
export function findFullText(paperId: string): Promise<LibraryPaper> {
  return apiRequest<LibraryPaper>(`/papers/${paperId}/find-fulltext`, { method: 'POST' });
}

/** True when the paper response means a PDF is on its way (or already stored). */
export function paperHasPdfComing(paper: LibraryPaper) {
  const version = paper.latest_version;
  if (!version || version.status === 'failed') {
    return false;
  }
  return version.status === 'processing' || version.status === 'uploading' || Boolean(version.pdf_key);
}
