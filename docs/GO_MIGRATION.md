# Миграция бэкенда Python → Go

**Статус:** ✅ завершена. Go API + worker в `docker-compose` (порт 8080). Python backend удалён. Миграции — SQL в `migrations/`.

## Зачем было

- один бинарник, проще деплой;
- меньше RAM;
- команда хочет Go.

## Что сделано

- HTTP-контракт сохранён для фронта (`backend/internal/httpapi`).
- Worker на asynq (Redis) вместо Celery.
- Схема вынесена в `migrations/001_init.sql` + `migrate.sh` (без Alembic/Python image).

## Стек (факт)

| Слой | Вариант |
|------|---------|
| HTTP | chi (см. `internal/httpapi`) |
| DB | pgx |
| Auth | JWT + bcrypt |
| S3 | MinIO SDK |
| Queue | asynq |
| Migrations | SQL + `psql` one-shot в compose |

## Исторический план

Шаги 1–7 из старого плана выполнены: каркас → auth → papers/worker → library/annotations/feed → LLM → cutover compose → удаление Python.
