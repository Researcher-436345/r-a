# Go API Researcher

Модули приложения работают как отдельные сервисы за gateway или вместе через
`cmd/api`, сохраняя один HTTP-контракт для `frontend/`.

## Структура

```text
cmd/
  api/                 # сборка общего HTTP-приложения
  gateway/             # внешний HTTP-вход и проверка авторизации
  worker/              # обработка PDF через asynq
  translator/          # отдельный сервис перевода выделенного текста
  assistant/           # чат, объяснения и обзоры статей
  searchapi/           # история поиска и сохранение исследований
internal/
  app/                 # подключение модулей к общему роутеру
  platform/            # конфигурация, БД, HTTP, очередь, хранилище
  modules/
    identity/          # пользователи, JWT, /auth/*
    catalog/           # статьи, PDF, arXiv, DOI, загрузка
    library/           # пользовательская библиотека
    annotations/       # выделения и заметки
    feed/              # лента, цитирования и описания Gemma
    assistant/         # OpenAI Responses API и клиент сервиса перевода
    translation/       # проверка текста и прямой OpenRouter API
    searchapi/         # поисковые чаты и состояние deep research
```

## Границы модулей

- `platform/*` не импортирует предметные модули.
- `library` использует DTO статей из `catalog`.
- `annotations` и `assistant` используют `catalog` для доступа к статьям и метаданным.
- HTTP-слои используют `identity.UserID` и middleware авторизации.
- `catalog` обращается к библиотеке через `catalog.Membership`, связанный в `app`.
- SQL предметной области хранится рядом с модулем, обычно в `store.go`.

Для выделения модуля в отдельный процесс используют `cmd/<name>` и переносят
подключение зависимостей из `app/router.go`.

## Настройки и LLM

Общие значения по умолчанию находятся в корневом `settings.py`; env имеет
приоритет. Go-загрузчик — `internal/platform/config`. Подробная таблица моделей,
адресов и ключей: [`docs/SETTINGS.md`](../docs/SETTINGS.md).

Чат, объяснения и обзоры используют прямой OpenAI Responses API. Основная модель —
`gpt-6.1-terra`, уровень рассуждений — `max`. Перевод фрагментов и описания ленты используют прежнюю Gemma
через прямой OpenRouter; ключи и адреса перевода независимы от OpenAI.

`searchapi` передаёт поиск Python-сервису `websearch`, сохраняет историю и
состояние фонового исследования. Миграция `013_search_runs.sql` нужна для
восстановления отчёта после разрыва соединения или перезапуска процесса.

## Запуск в Docker

Из корня репозитория:

```bash
cp .env.example .env
docker compose up -d --build
```

Compose запускает gateway, предметные сервисы, worker, translator, parser,
websearch, PostgreSQL, Redis и MinIO. Сервис `migrate` применяет SQL из `migrations/`.
Образы собираются из корня, чтобы Go и Python получили один `settings.py`.

Для локального запуска отдельных процессов из каталога `backend`:

```bash
export DATABASE_URL=postgres://researcher:researcher@localhost:5433/researcher
export REDIS_URL=redis://localhost:6379/0
go run ./cmd/api
go run ./cmd/worker
```

## Перевод

`POST /papers/{id}/translate` принимает `{ "text": "...", "target_lang": "ru" }`.
API проверяет доступ к статье и передаёт текст внутреннему сервису `translator`.
Лимит — `TRANSLATION_MAX_CHARS` символов Unicode. Поддерживаются `ru`, `en`, `de`,
`fr`, `es`, `it`, `pt`, `zh`, `ja`, `ko`; доступна потоковая выдача SSE.

## Цитирования

Лента использует OpenAlex без ключа (`OPENALEX_MAILTO`) и кеширует результаты в
Redis на 24 часа. Для Semantic Scholar можно задать `SEMANTIC_SCHOLAR_API_KEY`.
У новых статей arXiv число цитирований может отсутствовать; интерфейс скрывает
пустой показатель. После обогащения `sort=popular` учитывает известные цитирования.

## Гостевой доступ

Без входа доступны `GET /feed/trending`, `GET /papers/{id}`,
`GET /papers/{id}/pdf`, `GET /papers/{id}/pdf-url`,
`POST /papers/{id}/translate`, включая SSE. Доступ ограничен последней публичной
версией статьи; загруженные пользователем PDF остаются закрытыми.

`POST /papers/arxiv/open` с `{ "arxiv_id": "..." }` открывает публичную статью,
сохраняя библиотеку без изменений; параметр `add_to_library` игнорируется.
Импорт и сохранение в библиотеку, заметки, чат и объяснения, а также исследовательский
поиск требуют входа. Gateway удаляет присланный клиентом `X-User-Id`.

См. также [`docs/GO_MIGRATION.md`](../docs/GO_MIGRATION.md) и
[`docs/HANDOFF.md`](../docs/HANDOFF.md).
