# Services map

Last refreshed: 2026-08-11

Публичная точка входа для фронта: **gateway** `http://localhost:8080` (`VITE_API_URL` без изменений).

## Compose services

| Service | Port (container) | Role |
|---------|------------------|------|
| gateway | 8000 → host **8080** | CORS, JWT, reverse-proxy |
| identity | 8101 | `/auth/*` |
| catalog | 8102 | `/papers/*` (кроме chat/annotations), internal ACL |
| library | 8103 | `/library` |
| annotations | 8104 | annotations CRUD |
| assistant | 8105 | chat SSE, explain, models, translate proxy |
| feed | 8106 | `/feed/trending` |
| translator | 8090 | перевод |
| parser | 8091 | PDF→text |
| worker | — | asynq jobs |
| postgres / redis / minio | 5432 / 6379 / 9002 | infra |

Legacy monolith binary `api` ещё собирается в образе (rollback), в compose **не** запускается.

## Table owners (shared Postgres)

| Tables | Owner service |
|--------|---------------|
| users… | identity |
| papers, paper_documents, paper_chunks… | catalog (+ worker writes parse results) |
| library_* | library (catalog also uses membership for ACL) |
| annotations | annotations |
| chat_messages, chat_thread_summaries | assistant |

## Internal

- `GET /internal/papers/{id}/access` on **catalog** — `X-User-Id`, `204` if in library
- Downstream services trust `X-User-Id` from gateway (`identity.MiddlewareFromGateway`)

## Env (gateway)

`IDENTITY_URL`, `CATALOG_URL`, `LIBRARY_URL`, `ANNOTATIONS_URL`, `ASSISTANT_URL`, `FEED_URL`
