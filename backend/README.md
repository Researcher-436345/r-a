# Researcher Go API

Modular monolith with the same HTTP contract for `frontend/`.

## Layout

```text
cmd/
  api/                 # composition root (HTTP)
  worker/              # PDF ingest (asynq)
internal/
  app/                 # wires modules into one router
  platform/            # config, db, httpx, queue, storage
  modules/
    identity/          # users, JWT, /auth/*
    catalog/           # papers, PDF, arxiv/doi/upload
    library/           # user_library_items
    annotations/       # highlights / notes
    feed/              # trending
    assistant/         # chat / explain / translate
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

## Run (Docker)

From repo root:

```bash
docker compose up -d --build
```

- `migrate` — SQL in `../migrations`
- `api` — Go HTTP on `:8080`
- `worker` — asynq PDF jobs
- postgres / redis / minio

## Local binary

```bash
export DATABASE_URL=postgres://researcher:researcher@localhost:5432/researcher
export REDIS_URL=redis://localhost:6379/0
go run ./cmd/api
go run ./cmd/worker
```

See [docs/GO_MIGRATION.md](../docs/GO_MIGRATION.md) and [docs/HANDOFF.md](../docs/HANDOFF.md).
