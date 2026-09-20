# Iteration push notes

Last refreshed: 2026-09-09
Branch: feature/paper-summary (main влит 2026-09-09)
Repo: Researcher-436345/r-a

## One-liner

Вкладка **«Обзор»** в ридере в формате alphaXiv: переведённый заголовок, карточка с вкладками Резюме / Проблема / Метод / Результаты / Выводы / Ограничения и длинный «Разбор» (blog-style, с формулами и кликабельными цитатами `[p.N «…»]` → страница PDF). Кэш в `paper_summaries`, SSE-стриминг с прогрессивной разметкой карточки. Поверх main: production-ready auth (email, серверные сессии, сброс пароля) + pgbouncer.

Подробнее: [HANDOFF.md](./HANDOFF.md), [SERVICES.md](./SERVICES.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
cp .env.example .env   # LLM_API_KEY; SMTP_* опционально (в compose — Mailpit)
docker compose up -d --build
# migrations 001–008 via migrate service (008 — paper_summaries)
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

### Paper overview в стиле alphaXiv (`feature/paper-summary`, миграция `008_paper_summaries.sql`)

Что было: один markdown-блоб с фикс. секциями TL;DR / Задача / Метод / Результаты / Ограничения.
Что стало (сверено с alphaXiv по скринам мобильного «Blog» и web «AI Overview»):

- **Скелет ответа** — `summaryHeadings` в `backend/internal/modules/assistant/summary.go`:
  `# <заголовок на языке обзора>` → `## TL;DR` (2–4 предложения) → `## Problem` / `## Method` /
  `## Results` / `## Takeaways` / `## Limitations` (по 2–4 буллета, 15–35 слов, Results с числами) →
  `## Deep dive` (900–1400 слов: вводный абзац без заголовка, 4–6 `###`-разделов с названиями под
  статью, формулы `$…$`, таблица результатов, итоговый абзац). Заголовки-маркеры всегда английские —
  клиент маппит их на локализованные вкладки, поэтому один парсер на оба языка
- **Парсер на клиенте** `frontend/src/features/reader/summary-doc.ts`: терпим к русским/иным вариантам
  заголовков (алиасы), работает на частичном тексте — карточка заполняется по мере стриминга;
  без единого маркера падает в рендер сырого markdown
- **UI** `reader-summary-panel.tsx` под дизайн v3 (radius 0, Fraunces для заголовков): sticky-шапка
  (бейдж «AI-обзор», модель · дата, пересборка), заголовок статьи, карточка с горизонтальными
  вкладками + иконки как у alphaXiv, ниже «Разбор», внизу «Копировать» через общий `MessageActions`.
  Кэш готовых обзоров в памяти по `(paper, lang)` — переключение вкладок панели не дёргает API
- **Бюджет ответа и защита от обрыва**: `summaryReplyReserve = 9000` (вместо `LLM_REPLY_RESERVE`),
  в запрос уходит `max_tokens = 16000`; `finish_reason=length` / стрим без финального сигнала →
  `ErrLLMTruncated`. Обрыв (по лимиту или по эвристике `looksComplete` — текст не кончается точкой /
  строкой таблицы / формулой) лечится до 2 continuation-проходов: частичный текст уходит как ход
  ассистента + «продолжи с места обрыва», при этом текст статьи в промпте **ужимается вдвое на каждый
  проход** (обрыв почти всегда = исчерпанное окно контекста, см. Pitfalls). Пустой обрезанный ответ
  (reasoning-модель сожгла лимит на размышления) → ошибка, не кэшируется
- **Статусы**: текст ещё парсится → `425 Too Early` (клиент ждёт и повторяет POST каждые 5 с);
  чужая генерация → `409` (клиент опрашивает GET). Раньше оба были 409 и клиент показывал
  «генерируется в другой вкладке» во время парсинга
- Живой прогон промпта на arXiv:2607.16097 (та же статья, что на скринах alphaXiv), `deepseek-v4-flash`
  через proxyapi: скелет соблюдён на ru и en, 1.3–1.8k слов, 11 цитат страниц, формулы в разборе

### Влито из main (2026-09-09)


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

- Обзор живёт в боковой панели 360px — у alphaXiv это отдельный полноэкранный режим «Blog»;
  полноширинный режим чтения обзора не делали
- Нет картинок/фигур из статьи в разборе (alphaXiv вставляет figure captions) — парсер их не отдаёт
- Нет блока «Related work» (alphaXiv показывает 3–4 ключевые цитируемые работы)
- Локальная проверка UI в браузере не проводилась (Docker не был запущен); проверены `go test`,
  `tsc -b`, `vite build`, парсер на живом выводе модели
- EPIC-05 проекты, EPIC-10 теги — не начаты; Similar tab и sidebar — моки
- MFA/OAuth, смена email, блеклист паролей — вне скоупа этой итерации
- Прод-рассылка: SMTP-провайдер не выбран (dev — Mailpit, prod — любой SMTP через env)

## Architecture snapshot

Go microservices за gateway (CORS + JWT + `X-User-Id`), Postgres за pgbouncer, Redis (asynq + throttle), MinIO (PDF через API stream), parser/websearch — Python. Auth: identity-сервис, access JWT + серверные refresh-сессии в httpOnly cookie.

## Pitfalls

0. Обзор кэшируется по языку: смена локали = отдельная генерация. Маркеры секций в кэше английские —
   не «чинить» их на русские, парсер ждёт `## Problem`, а не «## Проблема» (алиас есть, но не для всего).
0. `LLM_CONTEXT_TOKENS=120000` — обещание конфига, не провайдера: OpenRouter (через proxyapi) роутит
   `deepseek-v4-flash` на апстримы с окном **32k**, и при промпте ~29k токенов (статья целиком) ответ
   обрывается на ~4k токенах с `finish_reason=stop`. Это и есть причина continuation с ужатием статьи;
   на 128k-провайдере он не срабатывает вовсе. Диагностика: `usage.total_tokens ≈ 32.9k` в ответе.
- Cursor/macOS занимает 9000/9002 — PDF только через API stream
- LLM из РФ — AITunnel/DeepSeek/Ollama
- Два git: канон — `r-a/`, корень `researcher/` — локальный workspace
- Cookie между :5173 и :8080 — same-site (порт не входит в site), CORS с `AllowCredentials` уже настроен

## Suggested next tasks

0. PR `feature/paper-summary` → `main`; прогнать UI глазами: стриминг карточки, 425/409, тёмная тема
0. Полноширинный режим «Обзор» (как alphaXiv Blog) + figures из парсера в разборе
1. Прод-деплой: COOKIE_SECURE=true, FRONTEND_URL=prod-origin, реальный SMTP-провайдер
2. Проверить pgbouncer под нагрузкой (worker + параллельные юзеры), метрики `SHOW POOLS`
3. EPIC-05 проекты (API + живой sidebar) — следующий P2 после auth

## API surface (summary)

- `GET /papers/{id}/summary?lang=ru|en` — кэш (`content` = markdown со скелетом) или 404; флаг `stale`
- `POST /papers/{id}/summary?lang=ru&stream=1[&force=1][&model=…]` — SSE `delta`/`done`/`error`;
  готовый кэш отдаётся тем же SSE; `425` — текст ещё парсится, `409` — уже генерируется

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
