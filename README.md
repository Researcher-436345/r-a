# Researcher

Научная библиотека + PDF-ридер.

## Структура

```
backend/      # Go API + PDF worker + translation microservice
frontend/     # React (Vite)
docs/         # планы, статус, handoff
migrations/   # SQL schema (без Python)
settings.py   # настройки со значениями по умолчанию и переопределением через env
```

Документация — в [`docs/`](./docs/). Старт для новичка: [`docs/HANDOFF.md`](./docs/HANDOFF.md).
Настройки, прямые OpenAI/OpenRouter API и выбор моделей: [`docs/SETTINGS.md`](./docs/SETTINGS.md).

## Запуск

```bash
cp .env.example .env
docker compose up -d --build

cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --host 127.0.0.1 --port 5180 --strictPort
```

| Сервис | URL |
|--------|-----|
| UI | http://localhost:5180 |
| API | http://localhost:8080/health |
| MinIO | http://localhost:9002 |

## Ветки

- `develop` — общая
- `develop-aleksandr` — рабочая ветка
