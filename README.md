# Researcher

Научная библиотека + PDF-ридер.

## Структура

```
backend/      # Go API + worker
frontend/     # React (Vite)
docs/         # планы, статус, handoff
migrations/   # SQL schema (без Python)
```

Документация — в [`docs/`](./docs/). Старт для новичка: [`docs/HANDOFF.md`](./docs/HANDOFF.md).

## Запуск

```bash
cp .env.example .env
docker compose up -d --build

cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --port 5173
```

| Сервис | URL |
|--------|-----|
| UI | http://localhost:5173 |
| API | http://localhost:8080/health |
| MinIO | http://localhost:9002 |

## Ветки

- `develop` — общая
- `develop-aleksandr` — рабочая ветка
