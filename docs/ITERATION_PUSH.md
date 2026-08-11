# Iteration push notes

Last refreshed: 2026-08-11
Branch: feature/reader-paper-chat
Repo: Researcher-436345/r-a

## One-liner

EPIC-08 paper chat + **API microservices** behind gateway (`identity/catalog/library/annotations/assistant/feed`). Streaming replies, page cites, notes markdown/collapse. Shared Postgres.

Подробнее: [HANDOFF.md](./HANDOFF.md), [SERVICES.md](./SERVICES.md), [PARSER.md](./PARSER.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
cp .env.example .env   # LLM_API_KEY + опционально LLM_MODELS
docker compose up -d --build
# migrations 001–004 via migrate service
# публичный API = gateway :8080 (не монолитный api)

cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --port 5173
```

| Сервис | URL |
|--------|-----|
| UI | http://localhost:5173 |
| **Gateway** | http://localhost:8080/health |
| Parser | http://localhost:8091/health |
| MinIO API | http://localhost:**9002** |
| MinIO UI | http://localhost:9003 |
| Postgres | localhost:5432 |
| Redis | localhost:6379 |

Внутренние порты: identity 8101, catalog 8102, library 8103, annotations 8104, assistant 8105, feed 8106 (+ translator 8090, worker).

Тест-аккаунт (если сид есть): `test@researcher.local` / `testpass123`.

## Done this iteration / currently working

### API microservices (latest)

- Gateway: CORS + JWT + reverse-proxy; фронт без смены URL
- `cmd/{gateway,identity,catalog,library,annotations,assistant,feed}`
- Catalog `GET /internal/papers/{id}/access` для ACL; `X-User-Id` от gateway
- Legacy `api` бинарь ещё в образе (rollback), в compose **не** стартует
- Карта: [SERVICES.md](./SERVICES.md)

### Streaming chat + page citations

- `POST /papers/{id}/chat?stream=1` → SSE через gateway → assistant
- Page markers `<<<p=N>>>`, UI `[p.N «quote»]` → PDF page

### Chat UX / notes

- Notes: RichText markdown + collapse «Показать ещё»
- Quote UI, context chip, notes↔chat (`004`)

### Full-text parse

- parser + TeX-first; `paper_documents` / chunks; worker parse job

## Not done / known gaps

- EPIC-05 проекты; EPIC-09 web-search; EPIC-10 теги; Similar mocks
- DB-per-service ещё нет (shared Postgres)
- Passage-level cite highlight / RAG
- Annotation rect в БД — px
- Старый контейнер `r-a-api-1` нужно снести вручную, если занимает 8080

## Architecture snapshot

```
frontend → gateway:8080
         → identity | catalog | library | annotations | assistant | feed
assistant → catalog /internal/.../access
worker → parser | postgres | minio
```

## Pitfalls

1. После сплита на 8080 должен слушать **gateway**, не старый `api` (`docker rm -f r-a-api-1`).
2. Cursor 9000/9002 → PDF через API stream.
3. Пушь из `r-a/`; compose читает **`r-a/.env`**.
4. Хук push: сегодняшний `Last refreshed` в этом файле.
5. SSE: gateway `FlushInterval = -1`; длинный chat timeout.
6. Downstream доверяют только `X-User-Id` от gateway (не публиковать 810x наружу без нужды).

## Suggested next tasks

1. PR `feature/reader-paper-chat` → `main`
2. Internal paper context HTTP (убрать прямые SQL reads assistant→documents где возможно)
3. Cite → text-layer highlight
4. EPIC-05 / web-search
5. DB-per-service по триггеру нагрузки

## API surface (через gateway, без изменений для фронта)

- `/auth/*` → identity
- `/papers/*` → catalog (кроме chat/annotations subpaths)
- `/library*` → library
- `/papers/{id}/annotations`, `/annotations*` → annotations
- `/papers/{id}/chat*`, explain, translate, `/assistant/*` → assistant
- `/feed/*` → feed

## Files to look at first

- `docs/SERVICES.md`, `docker-compose.yml`, `backend/Dockerfile`
- `backend/cmd/*/main.go`, `backend/internal/gateway/handler.go`
- `backend/internal/modules/catalog/client.go`, `identity/http.go` (`MiddlewareFromGateway`)
- `backend/internal/modules/assistant/*`, reader frontend notes/chat
- `docs/STATUS.md`, `docs/HANDOFF.md`
