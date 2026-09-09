# Services map

Last refreshed: 2026-08-11

Публичная точка входа для фронта: **gateway** `http://localhost:8080` (`VITE_API_URL` без изменений).

## Compose services

| Service | Port (container) | Role |
|---------|------------------|------|
| gateway | 8000 → host **8080** | CORS (+credentials, точный origin), JWT, reverse-proxy |
| identity | 8101 | `/auth/*` — register/verify-email/login/refresh/logout/forgot/reset/resend, sessions, me; письма (verify/reset) |
| catalog | 8102 | `/papers/*` (кроме chat/annotations), internal ACL |
| library | 8103 | `/library` |
| annotations | 8104 | annotations CRUD |
| assistant | 8105 | chat SSE, explain, models, translate proxy |
| feed | 8106 | `/feed/trending` |
| searchapi | 8107 | `/search/*`, chat ownership/history, SSE proxy |
| translator | 8090 | перевод |
| parser | 8091 | PDF→text |
| websearch | 8092 | internal Perplexity workflow and source normalization |
| worker | — | asynq jobs |
| mailpit | 1025 SMTP / 8025 UI → host **8025** | dev-почта: все письма видны на http://localhost:8025 |
| pgbouncer | 5432 (только внутри compose) | connection pooler (transaction mode) перед postgres; все Go-сервисы ходят через него, migrate — напрямую |
| postgres / redis / minio | 5432 → host **5433** / 6379 / 9002 | infra (redis также для auth-throttle) |

Legacy monolith binary `api` ещё собирается в образе (rollback), в compose **не** запускается.

## Table owners (shared Postgres)

| Tables | Owner service |
|--------|---------------|
| users, auth_sessions, auth_tokens | identity |
| papers, paper_documents, paper_chunks… | catalog (+ worker writes parse results) |
| library_* | library (catalog also uses membership for ACL) |
| annotations | annotations |
| chat_messages, chat_thread_summaries | assistant |
| search_chats, search_chat_messages | searchapi |

## Internal

- `GET /internal/papers/{id}/access` on **catalog** — `X-User-Id`, `204` for a public catalog paper or a paper in the user's library
- Downstream services trust `X-User-Id` from gateway (`identity.MiddlewareFromGateway`)

## Env (gateway)

`IDENTITY_URL`, `CATALOG_URL`, `LIBRARY_URL`, `ANNOTATIONS_URL`, `ASSISTANT_URL`, `FEED_URL`, `SEARCH_API_URL`

## Env (identity / auth)

`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`, `MAIL_ENABLED` (false → письма в stdout; в compose по умолчанию true через Mailpit), `FRONTEND_URL` (ссылки в письмах), `COOKIE_SECURE` (prod за HTTPS → true), `REDIS_URL` (throttle). Политики: login 10/мин/IP + 5/15мин/email; register/resend/forgot 5/15мин/IP+email; verify/reset 10/час/IP → 429 + `Retry-After`.
