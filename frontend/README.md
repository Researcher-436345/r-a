# r-a frontend

Frontend stack: React, TypeScript, Vite.  
Routing: TanStack Router. Data/cache: TanStack Query. PDF rendering: PDF.js.

## Run

```bash
npm install
npm run dev -- --port 5173
```

Open `http://localhost:5173`.

Auth pages: `/login`, `/register`.  
API URL: `VITE_API_URL` in `.env` (default `http://localhost:8080`).  
Backend must be running (`docker compose up` from repo root).

## Commands

```bash
npm run build    # Creates a production build in dist/
npm run preview  # Run this build locally
```

## Гостевой доступ и флаги

По умолчанию `/` открывает главную с поисковой строкой и лентой без входа.
Публичные статьи из ленты можно читать и переводить без аккаунта. Библиотека,
загрузка/сохранение статей, заметки, веб-поиск с ассистентом и вопросы по статье
и AI-обзор требуют входа. Форма входа возвращает на исходный маршрут через `next`; запрос
с главной сохраняется в адресе чата. Приватные загруженные PDF гостям недоступны.

Флаги задаются в `frontend/.env` перед сборкой и включаются только значением `true`:

- `VITE_ENABLE_ACTIVE_SESSIONS=false` — скрывает пункт и экран активных сессий.
  Серверные сессии, refresh-cookie и отзыв сессий продолжают обеспечивать вход.
- `VITE_ENABLE_LANDING=false` — лендинг `/welcome` сохранён, но выключен.
  При `true` гости с главной попадают на лендинг.
- `VITE_REQUIRE_LOGIN_ON_ENTRY=false` — при `true` вход требуется перед приложением.

Для применения флагов пересоберите фронтенд. Гостевой доступ также требует
обновления gateway, catalog, assistant и feed (или монолитного API).

При сборке через `docker-compose.prod.yml` те же флаги задаются в корневом `.env`
и передаются как build args. Для AI-обзоров нужна миграция `008_paper_summaries.sql`;
модель можно задать серверной переменной `SUMMARY_LLM_MODEL` (по умолчанию `LLM_MODEL`).
