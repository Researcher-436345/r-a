# Researcher API

Скелет бэкенда: FastAPI + Postgres + Redis + MinIO + Alembic + Auth (EPIC-01/02).

## Запуск

Из корня репозитория (`researcher/`):

```bash
cp .env.example .env
docker compose up --build
```

При старте API контейнер сам выполняет `alembic upgrade head`, затем поднимает uvicorn.

Проверка:

```bash
curl http://127.0.0.1:8080/health
# {"status":"ok","db":"ok","migrations":"ok"}
```

Auth:

```bash
# регистрация
curl -s -X POST http://127.0.0.1:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"password123"}'

# логин + профиль
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"password123"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')

curl -s http://127.0.0.1:8080/auth/me -H "Authorization: Bearer $TOKEN"
```

Документация API: http://localhost:8080/docs

> Порт снаружи — **8080**, потому что `8000` на macOS часто занят Cursor / JupyterHub.

## Сервисы

| Сервис | Порт | Назначение |
|--------|------|------------|
| api | 8080 → 8000 | FastAPI |
| postgres | 5432 | PostgreSQL 16 |
| redis | 6379 | Redis (очереди с EPIC-03) |
| minio | 9000 / 9001 | S3 + console |

## Миграции (Alembic)

Создать новую миграцию после изменения моделей:

```bash
docker compose exec api alembic revision --autogenerate -m "add users"
docker compose exec api alembic upgrade head
docker compose exec api alembic current
```

Текущая голова: миграция `add users` (таблица `users`).

Проверка версии миграции в Postgres:

```bash
docker compose exec postgres psql -U researcher -d researcher -c 'SELECT * FROM alembic_version;'
```

## Структура backend

```
backend/
├── alembic/                 # миграции
│   ├── env.py
│   └── versions/
├── app/
│   ├── core/config.py
│   ├── db.py                # engine, SessionLocal, get_db
│   ├── models/base.py       # DeclarativeBase
│   ├── routers/health.py
│   └── main.py
├── scripts/entrypoint.sh    # migrate → uvicorn
├── Dockerfile
├── alembic.ini
└── requirements.txt
```

## Локально без Docker (только API)

Нужны запущенные Postgres/Redis (`docker compose up postgres redis minio -d`).

```bash
cd backend
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
# DATABASE_URL должен указывать на localhost
alembic upgrade head
uvicorn app.main:app --reload --port 8080
```

## Papers / Library

```bash
# добавить arXiv (после login)
curl -s -X POST http://127.0.0.1:8080/papers/arxiv \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"arxiv_id":"1706.03762"}'

# библиотека
curl -s http://127.0.0.1:8080/library -H "Authorization: Bearer $TOKEN"
```

## Что дальше

- EPIC-05: projects tree
- EPIC-06: annotations
- EPIC-07/08: trending + AI
