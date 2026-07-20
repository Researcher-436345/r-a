# Миграция бэкенда Python → Go

**Статус:** Go API + worker в `docker-compose` (порт 8080). Python остался только для `migrate` (Alembic).  
**Цель:** заменить FastAPI/Celery (`backend/`) на Go-сервис с тем же HTTP-контрактом для фронта.

## Зачем

- один бинарник, проще деплой;
- меньше RAM;
- команда хочет Go.

## Что переносить (паритет API)

Сохранить пути и JSON из текущего Python API:

| Область | Эндпоинты |
|---------|-----------|
| Health | `GET /health` |
| Auth | `POST /auth/register\|login\|refresh`, `GET /auth/me` |
| Papers | `POST /papers/{arxiv,doi,upload}`, `GET /papers/{id}`, `GET .../pdf-url`, `POST .../retry-pdf` |
| AI | `POST .../chat`, `.../explain`, `.../translate` |
| Library | `GET/PATCH/DELETE /library...` |
| Annotations | CRUD |
| Feed | `GET /feed/trending` |

БД Postgres + MinIO + Redis остаются. Миграции Alembic → goose/golang-migrate (SQL можно переиспользовать).

## Стек (предложение)

| Слой | Вариант |
|------|---------|
| HTTP | chi или echo |
| DB | pgx + sqlc (или GORM) |
| Auth | golang-jwt + bcrypt |
| S3 | aws-sdk-go-v2 / minio-go |
| Queue | asynq (Redis) вместо Celery |
| Config | envconfig / koanf |

## План работ

1. Каркас `backend-go/`: `cmd/api`, `cmd/worker`, `/health`, конфиг, Docker.
2. Auth + users (паритет JWT).
3. Papers + storage + worker (arxiv/doi/upload).
4. Library + annotations + feed.
5. LLM/translate adapters.
6. Переключить `docker-compose.yml` на Go-образ; держать Python как fallback до зелёного E2E.
7. Удалить `backend/` Python после стабилизации.

## Оценка

~1–2 недели одному разработчику при сохранении контракта. Риск — регрессии worker/PDF, не язык.

## Не делать

- Менять контракт фронта «заодно»
- Параллельно пилить новые фичи только в Python без зеркала в Go после cutover
