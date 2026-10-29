import { getAccessToken } from '../../features/auth/token-storage';
import { API_URL } from '../api/base-url';

export type ResearchFeature = 'summary' | 'translation' | 'chat' | 'explanation' | 'search' | 'deep_research';
type EventType = 'project_opened' | 'paper_opened' | 'paper_saved' | 'note_created' | 'result_viewed' | 'result_copied' | 'feedback';
interface AnalyticsEvent {
  client_at?: string;
  action_id: string;
  session_id: string;
  event_type: EventType;
  feature?: ResearchFeature;
  project_id?: string;
  paper_id?: string;
  result_id?: string;
  source_action_id?: string;
  cache_hit?: boolean;
  locale: string;
  device: 'mobile' | 'desktop';
  source: 'feed' | 'search' | 'library' | 'reader' | 'other';
  feedback?: 1 | -1;
}
interface Action { id: string; feature: ResearchFeature; paperId?: string; sessionId: string; owner: string }
const responses = new WeakMap<Response, Action>();
const results = new Map<string, Action>();
const views = new Map<string, string>();
let queue: Array<{ owner: string; event: AnalyticsEvent }> = [];
let timer: ReturnType<typeof setTimeout> | undefined;
let sending = false;
let retryDelay = 1000;
let session = { id: '', owner: '', activeAt: 0 };
let attribution: string | undefined;
let initialized = false;

export function analyticsUUID(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  const h = Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
function owner(): string {
  try { return String(JSON.parse(atob((getAccessToken() ?? '').split('.')[1].replace(/-/g, '+').replace(/_/g, '/'))).sub ?? ''); }
  catch { return ''; }
}
function enabled() {
  return Boolean(getAccessToken()) && import.meta.env.VITE_ENABLE_ANALYTICS !== 'false'
    && (!import.meta.env.DEV || import.meta.env.VITE_ANALYTICS_IN_DEV === 'true');
}
function sessionId(): string {
  const now = Date.now(); const user = owner();
  try {
    const saved = JSON.parse(localStorage.getItem('research.analytics.session') ?? 'null');
    if (saved && saved.owner === user && now - saved.activeAt < 30 * 60_000) session = saved;
  } catch { /* Storage restrictions do not interrupt research. */ }
  if (!session.id || session.owner !== user || now - session.activeAt >= 30 * 60_000) {
    session = { id: analyticsUUID(), owner: user, activeAt: now };
    views.clear(); results.clear(); attribution = undefined;
  }
  session.activeAt = now;
  try { localStorage.setItem('research.analytics.session', JSON.stringify(session)); } catch { /* no storage */ }
  return session.id;
}
function locale() {
  const lang = document.documentElement.lang;
  if (lang === 'ru' || lang === 'en') return lang;
  try { return localStorage.getItem('researcher_locale') === 'en' ? 'en' : 'ru'; } catch { return 'ru'; }
}
function source(): AnalyticsEvent['source'] {
  if (attribution) return 'search';
  const path = location.pathname;
  return path.startsWith('/reader') ? 'reader' : path.startsWith('/library') ? 'library' : path.startsWith('/chat') ? 'search' : path === '/' ? 'feed' : 'other';
}
function schedule(delay = 1000) {
  if (!timer) timer = setTimeout(() => { timer = undefined; void flush(); }, delay);
}
async function flush() {
  if (sending || !queue.length) return;
  const token = getAccessToken();
  if (!token || !enabled()) { queue = []; return; }
  const currentOwner = owner();
  queue = queue.filter(item => item.owner === currentOwner);
  if (!queue.length) return;
  const batch = queue.splice(0, 20); sending = true;
  try {
    const response = await fetch(`${API_URL}/analytics/events`, {
      method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ events: batch.map(item => item.event) }), keepalive: true, credentials: 'omit',
    });
    if (response.status === 401 || response.status === 429 || response.status >= 500) throw new Error('analytics unavailable');
    // Invalid/expired events are dropped rather than triggering login or blocking the UI.
    retryDelay = 1000;
  } catch {
    queue = [...batch, ...queue].slice(0, 100); retryDelay = Math.min(retryDelay * 2, 60_000);
  } finally {
    sending = false; if (queue.length) schedule(retryDelay);
  }
}
export function initializeAnalytics() {
  if (initialized) return; initialized = true;
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'hidden') void flush(); });
  window.addEventListener('pagehide', () => { void flush(); });
  document.addEventListener('click', event => {
    const target = event.target instanceof Element ? event.target : null;
    if (!target?.closest('a')) return;
    const container = target.closest<HTMLElement>('[data-research-result-id]');
    if (container) {
      const id = container.dataset.researchResultId;
      const context = id ? results.get(`result:${id}`) : undefined;
      attribution = context && ['search', 'deep_research'].includes(context.feature) ? context.id : undefined;
    } else {
      const href = target.closest('a')?.getAttribute('href') ?? '';
      if (href === '/' || href.startsWith('/library') || location.pathname === '/' || location.pathname.startsWith('/library')) attribution = undefined;
    }
  }, { capture: true });
}
function emit(type: EventType, data: Partial<AnalyticsEvent> = {}) {
  if (!enabled()) return;
  initializeAnalytics();
  const sid = sessionId();
  const event: AnalyticsEvent = { action_id: analyticsUUID(), session_id: sid, event_type: type, locale: locale(), client_at: new Date().toISOString(),
    device: matchMedia('(max-width: 820px)').matches ? 'mobile' : 'desktop', source: source(),
    source_action_id: attribution, ...data };
  for (const field of ['project_id', 'paper_id', 'result_id', 'source_action_id'] as const) {
    if (event[field] && !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(event[field]!)) delete event[field];
  }
  queue.push({ owner: owner(), event });
  if (queue.length > 100) queue.shift(); schedule();
}
export function trackingHeaders(path: string, method: string, body: unknown, headers: Headers): Action | undefined {
  initializeAnalytics();
  if (method.toUpperCase() !== 'POST') return undefined;
  const match = path.match(/^\/papers\/([0-9a-f-]{36})\/(summary|translate|chat|explain)(?:\?|$)/i);
  let feature: ResearchFeature | undefined; let paperId: string | undefined;
  if (match) { feature = ({ summary: 'summary', translate: 'translation', chat: 'chat', explain: 'explanation' } as const)[match[2] as 'summary']; paperId = match[1]; }
  if (/^\/search\/chats\/[0-9a-f-]{36}\/messages(?:\?|$)/i.test(path)) feature = (body as { mode?: string } | undefined)?.mode === 'deep' ? 'deep_research' : 'search';
  if (!feature) return undefined;
  const action: Action = { id: headers.get('X-Research-Action-ID') || analyticsUUID(), feature, paperId, sessionId: sessionId(), owner: owner() };
  headers.set('X-Research-Action-ID', action.id); headers.set('X-Research-Session-ID', action.sessionId);
  headers.set('X-Research-Locale', locale()); headers.set('X-Research-Device', matchMedia('(max-width: 820px)').matches ? 'mobile' : 'desktop');
  return action;
}
export function bindTrackedResponse(response: Response, action?: Action) { if (action) responses.set(response, action); }
export function rememberCompletedResponse(response: Response, resultKey: string) {
  const action = responses.get(response); if (!action || action.owner !== owner()) return;
  results.set(`${action.feature}:${resultKey}`, action); results.set(`result:${resultKey}`, action);
  if (action.paperId) results.set(`${action.feature}:paper:${action.paperId}`, action);
  while (results.size > 300) results.delete(results.keys().next().value!);
}
function viewId(feature: ResearchFeature, key: string): string {
  const sid = sessionId(); const storageKey = `research.analytics.view:${sid}:${feature}:${key}`;
  const action = results.get(`${feature}:${key}`);
  if (action) return action.id;
  const memory = views.get(storageKey); if (memory) return memory;
  let id: string | null = null;
  try { id = sessionStorage.getItem(storageKey); } catch { /* no storage */ }
  id ||= analyticsUUID(); views.set(storageKey, id);
  try { sessionStorage.setItem(storageKey, id); } catch { /* no storage */ }
  return id;
}
export function trackResultViewed(feature: ResearchFeature, key: string, paperId?: string, resultId?: string) {
  if (!enabled()) return;
  const id = viewId(feature, key); const dedupe = `sent:${id}`;
  if (views.has(dedupe)) return;
  try { if (sessionStorage.getItem(`research.analytics.${dedupe}`)) return; } catch { /* no storage */ }
  views.set(dedupe, id);
  try { sessionStorage.setItem(`research.analytics.${dedupe}`, '1'); } catch { /* no storage */ }
  emit('result_viewed', { action_id: id, feature, paper_id: paperId, result_id: resultId, cache_hit: !results.has(`${feature}:${key}`) });
}
export function trackResultAction(type: 'feedback' | 'result_copied', feature: ResearchFeature, key: string, paperId?: string, resultId?: string, feedback?: 1 | -1) {
  emit(type, { action_id: viewId(feature, key), feature, paper_id: paperId, result_id: resultId, feedback });
}
export function attributeToResult(feature: ResearchFeature, key: string) {
  const action = results.get(`${feature}:${key}`);
  attribution = action && ['search', 'deep_research'].includes(feature) ? action.id : undefined;
}
export function trackNavigation(type: 'paper_opened' | 'project_opened', id: string) {
  if (!enabled() || document.visibilityState !== 'visible') return;
  const sid = sessionId(); const key = `${type}:${sid}:${new Date().toISOString().slice(0, 10)}:${id}`;
  if (views.has(key)) return; views.set(key, id);
  emit(type, type === 'paper_opened' ? { paper_id: id } : { project_id: id });
}
export function trackMutation(path: string, method: string, body: unknown, result: unknown) {
  const paper = path.match(/^\/library\/([0-9a-f-]{36})(?:\?|$)/i);
  const data = body as { favorite?: boolean; folder_id?: string } | undefined;
  if (paper && (method === 'POST' || (method === 'PATCH' && (data?.favorite === true || data?.folder_id)))) emit('paper_saved', { paper_id: paper[1] });
  const annotation = path.match(/^\/papers\/([0-9a-f-]{36})\/annotations$/i);
  if (annotation && method === 'POST') emit('note_created', { paper_id: annotation[1], result_id: (result as { id?: string })?.id });
}
