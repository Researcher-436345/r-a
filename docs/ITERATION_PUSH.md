# Iteration push notes

Last refreshed: 2026-08-11
Branch: feature/reader-paper-chat
Repo: Researcher-436345/r-a

## One-liner

EPIC-08 paper chat: full-text LLM context, **SSE streaming replies**, page cites `[p.N «quote»]` → jump to PDF page (no RAG), model picker, notes↔chat, quote UI, compact context chip.

Подробнее: [HANDOFF.md](./HANDOFF.md), [PARSER.md](./PARSER.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
cp .env.example .env   # LLM_API_KEY + опционально LLM_MODELS
docker compose up -d --build
# migrations 001–004 via migrate service

cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --port 5173
```

| Сервис | URL |
|--------|-----|
| UI | http://localhost:5173 |
| API | http://localhost:8080/health |
| Parser | http://localhost:8091/health |
| MinIO API | http://localhost:**9002** |
| MinIO UI | http://localhost:9003 |
| Postgres | localhost:5432 |
| Redis | localhost:6379 |

Compose: Go `api` + `worker` + `parser` + `translator`, SQL `migrate` из `migrations/`.

Тест-аккаунт (если сид есть): `test@researcher.local` / `testpass123`.

## Done this iteration / currently working

### Streaming chat + page citations (latest)

- `POST /papers/{id}/chat?stream=1` (or `Accept: text/event-stream`) → SSE: `delta` / `done` / `error`
- OpenAI-compatible stream in `assistant/llm.go`; Gemini пока one-shot fallback
- Frontend `chatPaperStream` updates bubble as tokens arrive
- Paper text in prompt marked `<<<p=N>>>` from `paper_chunks` (`FormatPaperWithPageMarkers`)
- Model cites `[p.N]` or `[p.N «short quote»]`; UI chips jump to PDF page (page-only, no span highlight yet)
- Paths: `assistant/{http,llm,prompt,pages}.go`, `content/store.go` (`ListChunks`), `frontend/.../rich-text.tsx`, `reader-chat-panel.tsx`

### Full-text parse + chat

- `services/parser` (PyMuPDF default; Docling optional), compose `:8091`
- TeX-first for arXiv (`content.TryArxivTeX`), else PDF parse
- `migrations/003_paper_documents.sql` — documents, chunks, `chat_thread_summaries`
- Worker `process_paper_parse` after PDF ready
- Chat: full text in prompt + token budget; rolling history summary on overflow

### Chat UX (reader)

- Compact context chip next to model picker
- Single selection → quote card above composer; multiple → inline chips
- Save chat message / reply excerpt → notes; jump note→chat (`004`)
- Free notes composer on Notes tab; chat bubble restyle + markdown CSS
- **Notes render Markdown** via `RichText` (same as chat); long notes collapse (~140px) with «Показать ещё» / «Свернуть»
- PDF selection overshoot fixes; model picker; KaTeX; humanized LLM errors

### Paper chat API (persistence)

- `migrations/002_chat_messages.sql`, `004_annotation_chat_source.sql`
- `GET/POST /papers/{id}/chat` (+ stream), explain from selection popup
- Annotations may store `source_chat_message_id` (page `0` = chat note)

## Not done / known gaps

- EPIC-05 проекты (sidebar моки)
- EPIC-09 web-search — ветка `feature/web-search`, не смержена
- EPIC-10 теги
- LLM баланс/ключ у провайдера (ProxyAPI); дорогие модели могут отдавать 402
- Similar tab — моки
- **Passage-level cite highlight** (text-layer search / RAG) — сейчас только страница
- RAG retriever / multi-paper agent — вне scope
- Annotation rect в БД всё ещё px при save (выделение — ratio)
- Старые заметки из чата без `source_chat_message_id` не прыгают назад

## Architecture snapshot

```
frontend → JWT → Go API (:8080)
                → assistant SSE chat + page-marked paper_chunks
                → /assistant/models + per-request model
                → annotations.source_chat_message_id ↔ chat bubble
worker ← asynq ← PDF ready → process_paper_parse → parser|TeX
```

## Pitfalls

1. Cursor слушает 9000/9002 → PDF только через API stream.
2. LLM: Gemini/OpenRouter из РФ часто блок; ProxyAPI/AITunnel; `LLM_MODELS` должен совпадать с тем, что реально оплачено.
3. Пушь из `r-a/`, не из parent `researcher/`.
4. `migrate.sh`: на legacy DB сначала помечает только `001`, затем применяет новые файлы — иначе `002` пропускается.
5. Перед `git push` хук требует сегодняшний `Last refreshed` в этом файле.
6. Compose читает **`r-a/.env`**, не родительский `researcher/.env`.
7. Селект модели в узком футере: `flex-shrink: 0` — иначе схлопывается.
8. Stream client timeout ≥ 5m; прокси не должен буферить SSE (`X-Accel-Buffering: no`).

## Suggested next tasks

1. PR `feature/reader-paper-chat` → `main`
2. Cite click → find quote on page text layer and flash highlight
3. Смержить/перенести `feature/web-search` в `modules/research`
4. EPIC-05: projects API + живой sidebar
5. Нормализовать annotation rect в БД (ratio)
6. Дочистить моки Similar / Ask-box
7. Опционально: per-model context limits; Gemini native stream

## API surface

- `GET /assistant/models` → `{ default, items: [{id,label}] }`
- `GET /papers/{paperID}/chat/messages` → `{ items: ChatMessage[] }`
- `GET /papers/{paperID}/chat/context?model=` → context usage estimate
- `POST /papers/{paperID}/chat` `{ message, context_text?, model? }` → JSON reply (non-stream)
- `POST /papers/{paperID}/chat?stream=1` → SSE `data: {"type":"delta"|"done"|"error",...}`
- `POST /papers/{paperID}/explain` `{ text, question?, model? }` → `{ reply }`
- `POST /papers/{paperID}/annotations` may include `source_chat_message_id`, `page: 0` for chat notes

## Files to look at first

- `backend/internal/modules/assistant/{http,llm,prompt,pages}.go`
- `backend/internal/modules/content/store.go` (`ListChunks`)
- `frontend/src/features/reader/components/{reader-chat-panel,reader-note-card,chat-composer}.tsx`
- `frontend/src/shared/ui/rich-text.tsx` (`linkifyPageCites`)
- `migrations/002–004_*.sql`, `services/parser/`, `docs/PARSER.md`
- `docs/STATUS.md`
