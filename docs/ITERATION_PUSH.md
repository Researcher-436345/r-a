# Iteration push notes

Last refreshed: 2026-08-10
Branch: feature/reader-paper-chat
Repo: Researcher-436345/r-a

## One-liner

EPIC-08 paper chat: persist history, stuff **full paper text** into the LLM (TeX-first / PDF parser), rolling chat-history summary, reader UX (rich text, context meter, model picker).

Подробнее: [HANDOFF.md](./HANDOFF.md), [PARSER.md](./PARSER.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
cp .env.example .env   # LLM_API_KEY + опционально LLM_MODELS
docker compose up -d --build
# migrations 001–003 via migrate service

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

### Chat UX (reader)

- Context fill meter (`GET …/chat/context`, bar above composer)
- Model picker (`GET /assistant/models`, `LLM_MODELS`, `localStorage`)
- KaTeX + markdown in assistant bubbles (`shared/ui/rich-text.tsx`)
- Plain-text paste into composer; caret/placeholder fix
- «Уточнить» on selecting text in assistant replies
- Humanized LLM errors (e.g. 402 insufficient balance)

### Paper chat API (persistence)

- `migrations/002_chat_messages.sql`
- `GET/POST /papers/{id}/chat`, explain from selection popup
- Paths: `assistant/{http,llm,store}.go`, `frontend/.../reader-chat-panel.tsx`

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

## Architecture snapshot

```
frontend → JWT → Go API (:8080)
                → assistant: chat_messages + full paper_documents text
                → /assistant/models + per-request model
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
- `POST /papers/{paperID}/explain` `{ text, question?, model? }` → `{ reply }` (не пишет в `chat_messages`)

## Files to look at first

- `migrations/002_chat_messages.sql`, `003_paper_documents.sql`, `migrate.sh`
- `services/parser/`, `docs/PARSER.md`
- `backend/internal/modules/content/`
- `backend/internal/modules/assistant/{http,llm,prompt,store}.go`
- `frontend/src/features/reader/api.ts`
- `frontend/src/features/reader/components/{reader-chat-panel,chat-composer}.tsx`
- `frontend/src/shared/ui/rich-text.tsx`
- `docs/STATUS.md`
