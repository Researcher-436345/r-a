# Researcher Go API

OpenAI-compatible drop-in for the former FastAPI backend. Same HTTP contract for `r-a/frontend`.

## Run (Docker)

From repo root:

```bash
docker compose up -d --build
```

Services:
- `migrate` — Alembic (Python image) once against Postgres
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

See [GO_MIGRATION.md](../GO_MIGRATION.md).
