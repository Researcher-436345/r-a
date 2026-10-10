Рабочие инструкции для агентов, работающих с кодом Researcher (`r-a`).

Этот файл описывает только то, что подтверждено кодом репозитория. Дополняет, а не
заменяет [`AGENTS.md`](./AGENTS.md) (язык документов, правила локальной проверки
фронтенда, доступ к проду) — при расхождении правил проверки фронтенда и доступа к
серверу приоритет у `AGENTS.md`.

## Что это за проект

Научная библиотека и PDF-ридер: импорт статей (upload / arXiv / DOI), чтение PDF в
браузере, заметки на выделениях, чат и AI-обзор по полному тексту, лента trending,
веб-поиск и deep-research. Подробный контекст продукта — [`docs/HANDOFF.md`](./docs/HANDOFF.md),
состояние эпиков — [`docs/STATUS.md`](./docs/STATUS.md), карта сервисов и портов —
[`docs/SERVICES.md`](./docs/SERVICES.md).

## Стек

| Часть | Технологии |
|-------|-----------|
| Backend | Go (`go.mod`: `go 1.23.0`), chi v5, pgx v5, asynq (Redis), minio-go, golang-jwt v5 |
| Frontend | React 18.3 + TypeScript 5.6 + Vite 4.5, TanStack Router + Query, PDF.js (`pdfjs-dist` 3.x), react-markdown + KaTeX |
| Python-сервисы | FastAPI 0.115, uvicorn; `parser` — PyMuPDF (Docling опционально), `websearch` — httpx |
| БД / инфра | PostgreSQL (+ pgbouncer, transaction mode), Redis, MinIO (S3), Mailpit (dev-почта) |
| Сборка / запуск | Docker Compose (`docker-compose.yml` + overlay `docker-compose.prod.yml`), npm |

Менеджеры пакетов: Go modules (`backend/go.sum`), npm (`frontend/package-lock.json`),
pip с пинованными версиями (`services/*/requirements.txt`). Lock-файл npm есть —
использовать его, а не обновлять зависимости по своей инициативе.

## Карта каталогов

```
backend/              Go modular monolith + отдельные доменные сервисы
  cmd/                точки входа (по одному main.go на бинарь)
    gateway/          публичный вход: CORS, JWT, reverse-proxy к доменным сервисам
    identity/         /auth/*, сессии, письма
    catalog/          /papers/* (кроме chat/annotations), internal ACL
    library/          /library
    annotations/      CRUD заметок
    assistant/        chat SSE, explain, summary, models, прокси перевода
    feed/             /feed/trending
    searchapi/        /search/*, история чатов поиска, SSE-прокси
    translator/       изолированный сервис перевода выделенного текста
    worker/           asynq-джобы (парсинг PDF)
    api/              legacy-монолит: собирается в образе для rollback, в compose НЕ запускается
  internal/
    app/              composition root монолита (router.go)
    gateway/          логика reverse-proxy
    platform/         config, db, httpx, mailer, queue, storage, throttle
    modules/          домены: identity, catalog, library, annotations, assistant,
                      feed, searchapi, translation, content
frontend/             React + Vite SPA
  src/app/            router.tsx, query-client.ts, layout
  src/pages/          страницы: auth, chat, home, landing, library, open, reader,
                      settings, design-system
  src/features/       доменные фичи: auth, chat, library, papers, reader
  src/shared/         api, config, i18n, lib, theme, ui, styles, assets
  scripts/            вспомогательные скрипты + fixture API (не прод-код)
  docker/nginx.conf   статика за nginx для prod-образа
services/parser/      FastAPI: POST /v1/parse (PDF → markdown/text/pages/chunks)
services/websearch/   FastAPI: POST /v1/search/stream (web-search и deep-research)
migrations/           SQL-схема 001…010 + migrate.sh (без Alembic/Python)
docs/                 HANDOFF, STATUS, SERVICES, PARSER, ITERATION_PUSH, планы
.cursor/              скиллы проекта + pre-push hook
```

## Границы модулей backend

Правила из [`backend/README.md`](./backend/README.md) — соблюдать при любых правках:

- `internal/platform/*` не импортирует доменные модули.
- Модули не импортируют друг друга, кроме: `library` → `catalog` (DTO статей),
  `annotations` / `assistant` → `catalog` (доступ к статье и метаданные).
  HTTP-слои могут использовать `identity.UserID` и middleware.
- `catalog` общается с `library` только через `catalog.Membership` (связывается в `app`).
- SQL домена живёт в `store.go` этого домена — не растаскивать запросы по пакетам.
- Владельцы таблиц в общей БД расписаны в `docs/SERVICES.md` («Table owners») —
  не писать в таблицы чужого сервиса.
- Downstream-сервисы доверяют заголовку `X-User-Id` от gateway; gateway отбрасывает
  клиентский `X-User-Id` перед проксированием. Не ослаблять это.
- Классы «не-статейных» URL дублируются в трёх местах и должны меняться вместе:
  `services/websearch/app/main.py` (`is_non_paper_url`),
  `backend/internal/modules/catalog/indexpages.go` (`not_a_paper`),
  `frontend/src/features/papers/link-kind.ts`.

## Команды

Все команды ниже реально запущены в этом репозитории 2026-10-10 на macOS
(Go 1.24.5, Node 22.11.0, npm 10.9.0, Python 3.11.8) и прошли успешно, если не
помечено иначе.

### Backend (из `backend/`)

```bash
go build ./...     # проверено: успешно
go vet ./...       # проверено: без замечаний
go test ./...      # проверено: все пакеты с тестами проходят
```

Тесты backend не требуют поднятой БД: Redis подменяется `miniredis`, HTTP — тестовыми
серверами. Пакетов без тестов много (`cmd/*`, `internal/app`, `internal/platform/db`,
`library`, `annotations` и др.) — при правках в них опираться на тесты соседних модулей.

Локальный запуск отдельного бинаря (из `backend/`, требует поднятых postgres и redis):

```bash
export DATABASE_URL=postgres://researcher:researcher@localhost:5433/researcher
export REDIS_URL=redis://localhost:6379/0
go run ./cmd/api      # или ./cmd/gateway, ./cmd/worker, ./cmd/identity, ...
```

Не проверено в этой задаче: нужна запущенная инфраструктура.

### Frontend (из `frontend/`)

```bash
npm install                                      # проверено: успешно
npm run build                                    # проверено: tsc -b && vite build, успешно
node --test scripts/test-conversation-store.mjs  # проверено: 5 тестов, все прошли
npm run preview                                  # локальный просмотр прод-сборки
```

Типизация проверяется только через `npm run build` (`tsc -b`) — отдельного скрипта
`typecheck` нет. Скриптов `test` и `lint` в `package.json` нет, конфигов ESLint и
Prettier в репозитории нет — **линтера у фронтенда не существует, не выдумывать команду**.

Dev-сервер: команда запуска и обязательный порт заданы в [`AGENTS.md`](./AGENTS.md)
(`:5180`, `--strictPort`, настоящий API через прокси `/api`) — использовать её, а не
`npm run dev` по умолчанию. Vite проксирует `/api` на `DEV_API_TARGET`, только если эта
переменная задана (`frontend/vite.config.ts`).

### Python-сервисы

Зависимости тестов не входят в `requirements.txt` — `pytest` ставится отдельно.
Проверено в отдельном venv:

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install pytest

# из services/websearch
pip install -r requirements.txt
python -m pytest            # проверено: 119 тестов прошли

# из services/parser
pip install -r requirements.txt   # достаточно pymupdf==1.24.14 для тестов
python -m pytest            # проверено: 12 тестов прошли
```

Тесты запускать из каталога сервиса: они импортируют пакет как `app.*`.
`requirements-docling.txt` — тяжёлый опциональный движок, для тестов не нужен.

### Docker Compose

```bash
cp .env.example .env     # заполнить LLM_API_KEY; SMTP_* опционально (в dev письма идёт в Mailpit)
docker compose up -d --build
```

Не проверено в этой задаче: сборка всех образов и внешние LLM-провайдеры выходят за
рамки онбординга (нет ключей, долгая сборка). Миграции применяет сервис `migrate`
(`migrations/migrate.sh`, идемпотентно по таблице `schema_migrations`), он ходит в
postgres напрямую, остальные Go-сервисы — через pgbouncer.

Prod-overlay:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

### Порты (из `docker-compose.yml`)

| Что | Хост |
|-----|------|
| gateway (публичный API) | **8081** → контейнер 8000 |
| Postgres | 5433 → 5432 |
| Redis | 6379 |
| MinIO API / UI | 9002 / 9003 |
| Mailpit SMTP / UI | 1025 / 8025 |
| parser | 8091 |
| websearch | 8092 |

Внутри compose: identity 8101, catalog 8102, library 8103, annotations 8104,
assistant 8105, feed 8106, searchapi 8107, translator 8090.

⚠️ Хостовый порт gateway в `docker-compose.yml` — `8081`, а `.env.example`,
`frontend/.env.example`, `README.md` и доки в `docs/` указывают `8080`. См. раздел
«Известные расхождения».

## Переменные окружения

Полный список имён с комментариями — [`.env.example`](./.env.example) (корень, для
backend и compose) и [`frontend/.env.example`](./frontend/.env.example) (для сборки
фронтенда). Группы: `DATABASE_URL` / `REDIS_URL`, `S3_*`, `JWT_*` и `*_TOKEN_EXPIRE_*`,
`SMTP_*` / `MAIL_*`, `LLM_*`, `TRANSLATION_*`, `PARSER_*`, `WEBSEARCH_*`, `FEED_*`,
`INTERNAL_TOKEN`, `CITATIONS_ENABLED`, `OPENALEX_MAILTO`, `FRONTEND_URL`,
`COOKIE_SECURE`, `ENVIRONMENT`.

Флаги фронтенда включаются только значением `true` и требуют пересборки:
`VITE_ENABLE_ACTIVE_SESSIONS`, `VITE_ENABLE_LANDING`, `VITE_REQUIRE_LOGIN_ON_ENTRY`.
В prod-сборке они передаются как build args из корневого `.env`.

**Значения переменных — секреты.** `.env` и `.password.txt` в `.gitignore`. Не
переносить значения ключей, паролей и токенов в код, документацию, комментарии,
коммиты и логи. В файлах и обсуждениях упоминать только имена переменных.

## Конвенции

- **Язык документов — русский** (правило `AGENTS.md`): Markdown ведём по-русски,
  команды, пути, URL, имена файлов и переменных не переводим. В коде и README
  отдельных сервисов исторически встречается английский — следовать стилю файла,
  который правим.
- **Ветка по умолчанию — `main`** (`origin/HEAD → origin/main`). PR открывать в `main`,
  если задача не указывает другую базу. `README.md` упоминает ветку `develop`, но на
  remote её нет — ориентироваться на `main`.
- Фичи живут в ветках `feature/<тема>` (`feature/web-search`, `feature/auth-sessions`,
  `feature/pgbouncer`, …).
- **Коммиты.** В истории два стиля: короткий Conventional Commits со скоупом
  (`feat(assistant): …`, `fix(reader): …`, `docs: …`) и императивная строка-предложение
  (`Track visible PDF page in reader counter`). Для нового кода предпочтителен первый.
  Одна строка темы, без точки в конце.
- Миграции — только новые файлы `NNN_name.sql` по возрастанию номера; уже применённые
  файлы не редактировать (`migrate.sh` считает их по имени в `schema_migrations`).
- Статусные доки обновляются своими скиллами: `docs/STATUS.md` — скилл
  `project-status`, `docs/ITERATION_PUSH.md` — скилл `iteration-push-notes`
  (`.cursor/skills/`). Прогресс не выдумывать — проверять по коду.

### Pre-push hook

`.cursor/hooks.json` ставит `failClosed`-хук на команды `git`: реальный `git push`
блокируется, если нет `docs/ITERATION_PUSH.md` со строкой
`Last refreshed: <сегодняшняя дата>`. Хук срабатывает в Cursor; в других
средах он может не выполняться, но правило проекта остаётся: перед содержательным
push обновить `docs/ITERATION_PUSH.md` (дата + ветка + что в пуше) скиллом
`iteration-push-notes`. `--dry-run`, `-n` и `--help` хук пропускает.

## Чего не делать

- Не менять продуктовый код в задачах, где просили документацию или анализ.
- Не обновлять зависимости и lock-файлы без отдельной задачи: версии в
  `package.json`, `go.mod` и `requirements.txt` пинованы осознанно (в том числе
  `overrides.nanoid`).
- Не нарушать границы модулей из раздела выше и не писать SQL домена вне его `store.go`.
- Не редактировать уже применённые файлы в `migrations/`.
- Не коммитить `.env`, ключи, пароли, `*.key`, `.password.txt`, `node_modules/`,
  `dist/`, `.venv/`.
- Не ослаблять гостевой доступ и проверки аутентификации «чтобы проще тестировать»:
  набор публичных эндпоинтов зафиксирован в `backend/README.md` (Guest access).
- Не подменять API моками и фикстурами при проверке интерфейса и не поднимать
  фронтенд на другом порту без явной просьбы (`AGENTS.md`). `frontend/scripts/v3-fixture-server.py`
  — только для сценариев, где он прямо упомянут в документации.
- Не отключать проверку ключа хоста при SSH и не хранить пароли в файлах проекта.
- Не трогать `backend/cmd/api` (legacy-монолит для rollback) как основную точку входа —
  публичный вход это `gateway`.
- Не переписывать `docs/iteration-1.md`, если не просили изменить сам план.

## Чеклист перед коммитом

1. Изменения только по задаче; `git status` без лишних файлов (`.env`, `dist/`, `.venv/`, `node_modules/`).
2. Трогали Go → из `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`.
3. Трогали фронтенд → из `frontend/`: `npm run build`; если задели хранилище диалогов —
   `node --test scripts/test-conversation-store.mjs`.
4. Трогали `services/parser` или `services/websearch` → `python -m pytest` из каталога сервиса.
5. Трогали списки «не-статейных» URL → обновлены все три места (см. «Границы модулей backend»).
6. Добавили миграцию → новый файл с следующим номером, старые не изменены.
7. Новые переменные окружения → имена добавлены в `.env.example` с комментарием, значения — нет.
8. В диффе нет секретов, ключей и реальных URL с токенами.
9. Значимые изменения → обновлён `docs/STATUS.md` (скилл `project-status`).
10. Перед push → `docs/ITERATION_PUSH.md` с сегодняшней датой `Last refreshed` (скилл `iteration-push-notes`).
11. CI в репозитории нет — проверки из пп. 2–4 заменяют его, прогонять локально до пуша.

## Известные расхождения и риски

Наблюдения на 2026-10-10, фиксируются как контекст, не как задачи:

- **Порт gateway: 8081 против 8080.** `docker-compose.yml` публикует gateway как
  `8081:8000`, а `.env.example`, `frontend/.env.example`, `frontend/README.md`,
  `README.md`, `docs/HANDOFF.md`, `docs/SERVICES.md`, `docs/ITERATION_PUSH.md` говорят
  про `localhost:8080`. При запуске по README фронтенд со значениями по умолчанию
  стучится в несуществующий порт. `docker-compose.prod.yml` согласован с `8081`.
- **CI отсутствует.** Каталога `.github/` нет, workflow нет; `Makefile` тоже нет.
  Ничего не проверяет PR автоматически.
- **`INTERNAL_TOKEN` по умолчанию пустой**, а `services/websearch/app/main.py` проверяет
  заголовок только при непустом значении — в dev внутренний SSE-эндпоинт открыт. При этом
  `docker-compose.yml` публикует `parser` на `8091` и `websearch` на `8092` на хост
  (у `parser` проверки токена нет вовсе). `docker-compose.prod.yml` эти порты не публикует.
- **`WEBSEARCH_DOMAINS`** читается кодом (`services/websearch/app/main.py`) и описан в
  `services/websearch/README.md`, но отсутствует в `.env.example` и в `environment:`
  обоих compose-файлов (через `env_file` из `.env` всё же дойдёт).
- **Линтера и формата нет** ни во фронтенде, ни в Go (нет `.golangci.yml`), стиль
  держится только на `gofmt`-привычке и ревью.
- **Нет тестов** у `internal/platform/db`, `storage`, `mailer`, `throttle`,
  `internal/app`, модулей `library` и `annotations` и у всех `cmd/*`.
- **`npm audit` сообщает о 14 уязвимостях** (4 low, 1 moderate, 8 high, 1 critical) на
  текущем lock-файле; Vite 4.5 и pdfjs-dist 3.x заметно отстают от актуальных мажоров.
- **Фронтенд-бандл одним куском**: `dist/assets/index-*.js` ≈ 1.38 МБ (плюс
  pdf.worker ≈ 1.09 МБ), Vite предупреждает о чанках > 500 КБ.
- **Доки частично устаревшие**: `docs/STATUS.md` от 2026-09-08, `docs/ITERATION_PUSH.md`
  от 2026-09-09 (ветка `feature/paper-summary`), `docs/SERVICES.md` от 2026-08-11, тогда
  как в `main` после них легли гостевой доступ, лента и мобильный интерфейс.
  `docs/HANDOFF.md` описывает `r-a` как вложенный репозиторий внутри `researcher/` —
  к текущему чекауту это не относится.
- **`README.md` указывает ветки `develop` / `develop-aleksandr`**, но `develop` на remote
  отсутствует, а default-ветка — `main`.
- **`AGENTS.md` содержит координаты прод-сервера** (адрес, логин, отпечаток ключа). Пароля
  в репозитории нет, но сам файл — публичная поверхность; учитывать при правках.
- **Незавершённое по `docs/STATUS.md`**: EPIC-05 «Проекты» и EPIC-10 «Теги» не начаты,
  в сайдбаре и вкладке Similar остаются моки, RAG и span-highlight в AI отсутствуют.
