# Iteration push notes

Last refreshed: 2026-08-08
Branch: develop-aleksandr
Repo: Researcher-436345/r-a

## One-liner

Научная библиотека + PDF-ридер. В этом пуше: **modular monolith** Go-бэкенда (grow-ready модули, не отдельные микросервисы в проде) + фиксы upload PDF metadata / пустой library.

Подробнее: [HANDOFF.md](./HANDOFF.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
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
| MinIO API | http://localhost:**9002** |
| MinIO UI | http://localhost:9003 |
| Postgres | localhost:5432 |
| Redis | localhost:6379 |

Compose: Go `api` + `worker`, SQL `migrate` из `migrations/`.

## Done this iteration / currently working

- **Modular backend** (один деплой, границы модулей):
  - `backend/internal/app/router.go` — composition root
  - `backend/internal/platform/{config,db,httpx,queue,storage}`
  - `backend/internal/modules/{identity,catalog,library,annotations,feed,assistant}`
- HTTP-контракт без изменений (`/auth`, `/papers`, `/library`, `/annotations`, `/feed`)
- Worker читает `catalog.Store` (PDF ingest asynq)
- Empty library: API отдаёт `"items": []` (не `null`); фронт `?? []`
- Local PDF upload metadata:
  - не брать `/Title` из outline (bookmarks)
  - UTF-16 / PDF escapes
  - если найден arXiv id → title/authors/abstract с arXiv API

## Not done / known gaps

- EPIC-05 проекты (sidebar моки)
- EPIC-09 web-search — есть чужая ветка `feature/web-search` (Ilia), не смержена
- EPIC-10 теги
- EPIC-08: нет `chat_messages` в БД; LLM из РФ нестабилен
- Similar tab — моки
- Ветка `papper_chat` (Gleb) — отдельный assistant-прототип от старого `main`, не влита
- Отдельные Docker-сервисы / gateway — **намеренно не сейчас** (сначала модули)

## Architecture snapshot

```
frontend → JWT → Go API (:8080)  [cmd/api + internal/app]
                → modules: identity / catalog / library / annotations / feed / assistant
                → platform: Postgres / Redis(asynq+cache) / MinIO
                → GET /papers/{id}/pdf streams file
worker ← asynq ← process_arxiv_pdf | finalize_uploaded_pdf  [uses catalog]
```

Правила границ: `backend/README.md`.  
Catalog ↔ library только через `catalog.Membership` (wired in `app`).

## Pitfalls

1. Cursor слушает 9000/9002 → PDF только через API stream.
2. LLM: Gemini/OpenRouter из РФ часто блок; AITunnel/Ollama.
3. Два git: пушь из `r-a/`, не из parent `researcher/`.
4. Старый compose в parent может занять порты — `docker compose down` там перед `r-a`.
5. Перед `git push` хук требует сегодняшний `Last refreshed` в этом файле.

## Suggested next tasks

1. Смержить/перенести `feature/web-search` в `modules/research`
2. EPIC-05: projects API + живой sidebar
3. chat_messages + история чата в `assistant`
4. Explain из selection popup
5. Дочистить моки Similar / Ask-box
6. Когда заболит LLM/SSE — вынести `assistant` в `cmd/assistant`

## API surface

Без breaking changes:  
`/auth/*`, `/papers/{arxiv,doi,upload,id,pdf-url,pdf,retry-pdf,chat,explain,translate}`, `/library/*`, `/papers/{id}/annotations`, `/annotations/{id}`, `/feed/trending`

## Files to look at first

- `backend/internal/app/router.go`
- `backend/internal/modules/catalog/` (http, store, pdfmeta, arxiv)
- `backend/internal/modules/identity/`
- `backend/internal/modules/library/`
- `backend/cmd/api/main.go`, `backend/cmd/worker/main.go`
- `backend/README.md`
- `frontend/src/pages/library/library-page.tsx`
- `docs/HANDOFF.md`, `docs/STATUS.md`
