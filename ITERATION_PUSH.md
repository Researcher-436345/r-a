# Iteration push notes

Last refreshed: 2026-07-20
Branch: develop-aleksandr
Repo: Researcher-436345/r-a

## One-liner

Научная библиотека + PDF-ридер (итерация 1). Бэкенд на **Go**, фронт в `frontend/`. Сквозной сценарий «регистрация → статья → PDF → заметки → trending» работает. AI из РФ нестабилен. Этот файл — точка входа перед push.

Подробнее: [HANDOFF.md](./HANDOFF.md), краткий чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cp .env.example .env
docker compose up -d --build

cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --port 5173
```

| Сервис | URL |
|--------|-----|
| UI | http://localhost:5173 |
| API | http://localhost:8080/health |
| MinIO API | http://localhost:**9002** (не 9000 — конфликт с Cursor) |
| MinIO UI | http://localhost:9003 |
| Postgres | localhost:5432 |
| Redis | localhost:6379 |

Compose: Go `api` + `worker`, one-shot `migrate` (Alembic из `backend/`).

## Done / working now

- Auth JWT (register/login/refresh/me) — Go + UI
- Papers: upload / arXiv / DOI, dedup, asynq worker
- Library CRUD-ish (list/patch/delete, favorite)
- PDF reader via **`GET /papers/{id}/pdf`** → blob URL (не MinIO с хоста)
- Annotations + selection popup (заметка / в чат / перевод)
- Chat chips: `стр. N · слова`, клик → прыжок в PDF
- Trending feed + Redis cache
- Title из PDF при upload
- Docs: HANDOFF, STATUS, iteration-1, GO_MIGRATION
- Cursor: skill `iteration-push-notes` + hook на `git push`

## Not done / gaps

- EPIC-05 проекты (sidebar моки)
- EPIC-09 web-search
- EPIC-10 теги
- EPIC-08: нет `chat_messages` в БД; LLM часто недоступен из РФ
- Similar tab — моки
- Миграции ещё Alembic (Python image), не goose

## Architecture snapshot

```
frontend → JWT → Go API (:8080)
                → Postgres / Redis(asynq+cache) / MinIO(internal)
                → GET /papers/{id}/pdf streams file
worker ← asynq ← process_arxiv_pdf | finalize_uploaded_pdf
```

Код API: `backend-go/internal/httpapi/server.go`.  
Фронт ридера: `frontend/src/pages/reader/`, `frontend/src/features/reader/`, `frontend/src/features/library/api.ts`.

## Pitfalls

1. Cursor слушает 9000/9002 на localhost → PDF только через API.  
2. LLM: Gemini/OpenRouter из РФ часто блок; AITunnel/Ollama.  
3. Не коммитить `.env`.  
4. Перед `git push` хук требует сегодняшний `Last refreshed` в этом файле.

## Suggested next tasks

1. EPIC-05: projects API + живой sidebar  
2. chat_messages + история чата  
3. goose вместо Alembic; убрать Python из runtime  
4. Explain из selection popup  
5. Дочистить моки Similar / Ask-box  

## API surface (private = Bearer)

`/auth/*`, `/papers/{arxiv,doi,upload,id,pdf-url,pdf,retry-pdf,chat,explain,translate}`, `/library/*`, `/papers/{id}/annotations`, `/annotations/{id}`, `/feed/trending`

## Files to look at first

- `HANDOFF.md`, `STATUS.md`, `ITERATION_PUSH.md` (этот файл)
- `docker-compose.yml`, `.env.example`
- `backend-go/cmd/api`, `backend-go/cmd/worker`, `backend-go/internal/httpapi/server.go`
- `frontend/src/features/library/api.ts`, `frontend/src/pages/reader/reader-page.tsx`
- `.cursor/skills/iteration-push-notes/SKILL.md`, `.cursor/hooks.json`
