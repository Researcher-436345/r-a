# Iteration push notes

Last refreshed: 2026-08-11
Branch: feature/reader-paper-chat
Repo: Researcher-436345/r-a

## One-liner

EPIC-08 paper chat: full-text LLM context, model picker, notes↔chat deep links, single-passage quote UI, compact context chip, tighter PDF text selection.

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

### Full-text parse + chat

- `services/parser` (PyMuPDF default; Docling optional), compose `:8091`
- TeX-first for arXiv (`content.TryArxivTeX`), else PDF parse
- `migrations/003_paper_documents.sql` — documents, chunks, `chat_thread_summaries`
- Worker `process_paper_parse` after PDF ready
- Chat: full `plain_text` in prompt + token budget; rolling history summary on overflow
- Paths: `backend/internal/modules/content/`, `assistant/{http,llm,prompt}.go`, `cmd/worker/main.go`

### Chat UX (reader) — 2026-08-11

- Compact context chip next to model picker (tooltip for details)
- Single selection → quote card above composer; multiple → inline chips
- Save whole chat message or selected reply excerpt → notes
- Notes with `source_chat_message_id` jump back to the chat bubble
- PDF text selection: pointer-events + `.endOfContent` DOM placement (less “select all”)
- Model picker, KaTeX/markdown, plain-text paste, humanized LLM errors

### Paper chat API (persistence)

- `migrations/002_chat_messages.sql`, `004_annotation_chat_source.sql`
- `GET/POST /papers/{id}/chat`, explain from selection popup
- Annotations may store `source_chat_message_id` (page `0` = chat note)

### Already on main (context)

- Reader UX: fit-to-width, resizable chat, pastel highlights, translate selection
- Feed New/Hot/Popular + OpenAlex cites
- Modular monolith

## Not done / known gaps

- EPIC-05 проекты (sidebar моки)
- EPIC-09 web-search — ветка `feature/web-search`, не смержена
- EPIC-10 теги
- LLM баланс/ключ у провайдера (ProxyAPI); дорогие модели могут отдавать 402
- Similar tab — моки
- Streaming / RAG / multi-paper agent — вне scope
- Annotation rect в БД всё ещё px при save (выделение — ratio)
- Старые заметки из чата без `source_chat_message_id` не прыгают назад

## Architecture snapshot

```
frontend → JWT → Go API (:8080)
                → assistant: chat_messages + full paper_documents text
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

## Suggested next tasks

1. PR `feature/reader-paper-chat` → `main`
2. Смержить/перенести `feature/web-search` в `modules/research`
3. EPIC-05: projects API + живой sidebar
4. Нормализовать annotation rect в БД (ratio)
5. Дочистить моки Similar / Ask-box
6. Опционально: per-model context limits вместо одного `LLM_CONTEXT_TOKENS`

## API surface

- `GET /assistant/models` → `{ default, items: [{id,label}] }`
- `GET /papers/{paperID}/chat/messages` → `{ items: ChatMessage[] }`
- `GET /papers/{paperID}/chat/context?model=` → context usage estimate
- `POST /papers/{paperID}/chat` `{ message, context_text?, model? }` → `{ reply, …, context_usage }`
- `POST /papers/{paperID}/explain` `{ text, question?, model? }` → `{ reply }`
- `POST /papers/{paperID}/annotations` may include `source_chat_message_id`, `page: 0` for chat notes

## Files to look at first

- `migrations/002_chat_messages.sql`, `003_paper_documents.sql`, `004_annotation_chat_source.sql`
- `services/parser/`, `docs/PARSER.md`
- `backend/internal/modules/content/`
- `backend/internal/modules/assistant/{http,llm,prompt,store}.go`
- `backend/internal/modules/annotations/{http,store,dto}.go`
- `frontend/src/features/reader/components/{reader-chat-panel,chat-composer,assistant-reply-selection-bar,reader-pdf-canvas-viewer}.tsx`
- `docs/STATUS.md`
