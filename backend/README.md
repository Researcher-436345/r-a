# Researcher Go API

Modular monolith with the same HTTP contract for `frontend/`.

## Layout

```text
cmd/
  api/                 # composition root (HTTP)
  worker/              # PDF ingest (asynq)
  translator/          # isolated selected-text translation service
internal/
  app/                 # wires modules into one router
  platform/            # config, db, httpx, queue, storage
  modules/
    identity/          # users, JWT, /auth/*
    catalog/           # papers, PDF, arxiv/doi/upload
    library/           # user_library_items
    annotations/       # highlights / notes
    feed/              # trending + citation enrich (OpenAlex / optional S2)
    assistant/         # chat / explain + translation service client
    translation/       # validation + OpenAI-compatible provider adapter
```

### Boundary rules

- `platform/*` does not import domain modules.
- Modules do not import each other except:
  - `library` → `catalog` (paper DTOs)
  - `annotations` / `assistant` → `catalog` (paper access + metadata)
  - HTTP layers may use `identity.UserID` / middleware
- Catalog talks to library only via `catalog.Membership` (wired in `app`).
- SQL for a domain stays inside that module’s `store.go`.

To grow a module into its own process later: add `cmd/<name>` and move wiring out of `app/router.go`.

## Citations

Лента обогащает статьи цитированиями:

| Сейчас | Потом (вставить в `.env`) |
|--------|---------------------------|
| **OpenAlex** без ключа (`OPENALEX_MAILTO`) | `SEMANTIC_SCHOLAR_API_KEY` — [форма ключа](https://www.semanticscholar.org/product/api#api-key-form) |
| Кеш Redis `cite:v1:*` на 24h | Опционально OpenAlex API key |

На brand-new arXiv часто `citation_count: null` (ещё нет в индексе) — на UI бейдж не показываем.  
`sort=popular` после enrich переранжирует по известным цитированиям.

См. комментарии в корневом `.env.example`.

## Run (Docker)

From repo root:

```bash
docker compose up -d --build
```

- `migrate` — SQL in `../migrations`
- `api` — Go HTTP on `:8080`
- `worker` — asynq PDF jobs
- `translator` — selected-text translation through ProxyAPI/OpenRouter
- postgres / redis / minio

## Translation

`POST /papers/{id}/translate` accepts `{ "text": "...", "target_lang": "ru" }`.
The authenticated API verifies paper access and forwards the request to the private
`translator` service. Text is limited to `TRANSLATION_MAX_CHARS` Unicode characters
(5000 by default); supported target languages are `ru`, `en`, `de`, `fr`, `es`,
`it`, `pt`, `zh`, `ja`, and `ko`.

Configure an OpenAI-compatible provider with `LLM_BASE_URL`, `LLM_API_KEY`, and
`LLM_MODEL`. See the root `.env.example` for ProxyAPI and OpenRouter examples.

## Local binary

```bash
export DATABASE_URL=postgres://researcher:researcher@localhost:5433/researcher
export REDIS_URL=redis://localhost:6379/0
go run ./cmd/api
go run ./cmd/worker
```

See [docs/GO_MIGRATION.md](../docs/GO_MIGRATION.md) and [docs/HANDOFF.md](../docs/HANDOFF.md).
