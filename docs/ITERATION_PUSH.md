# Iteration push notes

Last refreshed: 2026-09-09
Branch: main
Repo: Researcher-436345/r-a

## One-liner

Production-ready auth: подтверждение email (Mailpit/SMTP), серверные сессии refresh с ротацией и reuse detection, httpOnly-cookie, сброс пароля, троттлинг `/auth/*`, UI активных сессий + pgbouncer перед Postgres.

Подробнее: [HANDOFF.md](./HANDOFF.md), [SERVICES.md](./SERVICES.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
cp .env.example .env   # LLM_API_KEY; SMTP_* опционально (в compose — Mailpit)
docker compose up -d --build
# migrations 001–007 via migrate service
# публичный API = gateway :8080

cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --port 5173
```

| Сервис | URL |
|--------|-----|
| UI | http://localhost:5173 |
| **Gateway** | http://localhost:8080/health |
| Mailpit (письма dev) | http://localhost:8025 |
| MinIO API | http://localhost:**9002** |
| MinIO UI | http://localhost:9003 |
| Postgres | localhost:5433 (сервисы внутри ходят через pgbouncer) |
| Redis | localhost:6379 |

Внутренние порты: identity 8101, catalog 8102, library 8103, annotations 8104, assistant 8105, feed 8106 (+ translator 8090, worker, pgbouncer).

## Done this iteration / currently working

### Auth: verification + sessions + password reset (`feature/auth-sessions`, миграция `007`)

- `register` не выдаёт токены → письмо (Mailpit :8025 в деве; SMTP через `SMTP_*`) → `verify-email` → авто-логин; resend-эндпоинт; generic-ответы против user enumeration
- Refresh = запись в `auth_sessions` (sha256-хеш, UA/IP/last_used): ротация при каждом refresh, reuse detection → отзыв всех сессий юзера
- Refresh в HttpOnly cookie (`researcher_refresh`, SameSite=Lax, `COOKIE_SECURE` для prod); access JWT 30 мин с `sid`-claim — контракт `Bearer` не менялся
- Forgot/reset пароля (`auth_tokens`, одноразовые, TTL 1h/24h); сброс пароля отзывает все сессии
- `GET/DELETE /auth/sessions[/id]` — список активных сессий, отзыв, logout-everywhere; страница `/settings/sessions`
- Redis-троттлинг: login 10/мин/IP + 5/15мин/email; register/resend/forgot 5/15мин; verify/reset 10/час → 429 + `Retry-After`; лог неудач
- Фронт: access в памяти + silent refresh, страницы verify/forgot/reset; тесты identity (miniredis, 797 строк)
- ⚠️ Миграционный момент: старые stateless refresh-JWT умерли — перелогин; существующие юзеры автоворифицированы миграцией

### Infra: pgbouncer (`feature/pgbouncer`)

- `edoburu/pgbouncer` transaction mode перед postgres; 7 Go-сервисов через `pgbouncer:5432`, migrate — напрямую
- `MAX_PREPARED_STATEMENTS=200` (совместимость с pgx), `MAX_DB_CONNECTIONS=60` — закрывает риск `too many clients` (7 пулов × 20 коннектов vs 100 дефолтных у postgres)

## Not done / known gaps

- EPIC-05 проекты, EPIC-10 теги — не начаты; Similar tab и sidebar — моки
- MFA/OAuth, смена email, блеклист паролей — вне скоупа этой итерации
- Прод-рассылка: SMTP-провайдер не выбран (dev — Mailpit, prod — любой SMTP через env)

## Architecture snapshot

Go microservices за gateway (CORS + JWT + `X-User-Id`), Postgres за pgbouncer, Redis (asynq + throttle), MinIO (PDF через API stream), parser/websearch — Python. Auth: identity-сервис, access JWT + серверные refresh-сессии в httpOnly cookie.

## Pitfalls

- Cursor/macOS занимает 9000/9002 — PDF только через API stream
- LLM из РФ — AITunnel/DeepSeek/Ollama
- Два git: канон — `r-a/`, корень `researcher/` — локальный workspace
- Cookie между :5173 и :8080 — same-site (порт не входит в site), CORS с `AllowCredentials` уже настроен

## Suggested next tasks

1. Прод-деплой: COOKIE_SECURE=true, FRONTEND_URL=prod-origin, реальный SMTP-провайдер
2. Проверить pgbouncer под нагрузкой (worker + параллельные юзеры), метрики `SHOW POOLS`
3. EPIC-05 проекты (API + живой sidebar) — следующий P2 после auth

## API surface (auth — изменился)

- `POST /auth/register` → 200 «check your email» (без токенов)
- `POST /auth/verify-email|resend-verification|login|refresh|logout|forgot-password|reset-password`
- `GET /auth/me` (+`email_verified`), `GET|DELETE /auth/sessions[/id]`
- Refresh: cookie (браузер) или `refresh_token` в body (не-браузерные клиенты); 403 `email_not_verified` при логине непроверенным

## Files to look at first

- `backend/internal/modules/identity/http.go` — все auth-эндпоинты
- `backend/internal/modules/identity/store.go` — sessions/tokens queries
- `migrations/007_auth_sessions.sql`
- `backend/internal/platform/mailer/`, `backend/internal/platform/throttle/`
- `docker-compose.yml` — mailpit + pgbouncer
- `frontend/src/features/auth/`, `frontend/src/pages/auth/`, `frontend/src/pages/settings/sessions-page.tsx`
