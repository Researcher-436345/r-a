# Handoff — Researcher

**Для кого:** любой, кто продолжает разработку.  
**Дата:** 2026-07-20  
**Коротко:** локальный прототип научной библиотеки + PDF-ридер. Сквозной сценарий «зарегистрироваться → добавить статью → читать → заметки» работает. AI из РФ нестабилен. Бэкенд уже на **Go**; Python остался только для миграций.

Живой чеклист эпиков: [`STATUS.md`](./STATUS.md).  
План итерации 1: [`iteration-1.md`](./iteration-1.md).  
Долгий roadmap: [`roadmap.md`](./roadmap.md).

---

## 1. Что это за продукт

**Researcher** — веб-приложение для работы с научными статьями:

1. Регистрация / вход  
2. Добавление PDF (upload / arXiv / DOI) в личную библиотеку  
3. Чтение PDF в браузере (PDF.js)  
4. Заметки на выделениях  
5. Чат/explain/перевод по статье (LLM)  
6. Лента trending с arXiv на главной  

UI изначально был прототипом на моках (`frontend/`), потом подключён к реальному API.

---

## 2. Структура репозитория

```
r-a/   (GitHub: Researcher-436345/r-a)
├── backend-go/          # Актуальный API + worker (Go) ← основной бэкенд
├── backend/             # Старый FastAPI; сейчас только Alembic (сервис migrate)
├── frontend/        # React + Vite + TanStack Router/Query + PDF.js
├── docker-compose.yml   # postgres, redis, minio, migrate, api, worker
├── .env / .env.example
├── iteration-1.md       # детальный план эпиков TASK-*
├── roadmap.md           # этапы 0–5 + backlog идей
├── STATUS.md            # краткий статус ✅/🟡/❌
├── GO_MIGRATION.md      # заметки по переезду Python → Go
├── HANDOFF.md           # этот файл
└── .cursor/skills/project-status/  # скилл обновления STATUS.md
```

### Git (важно)

| Путь | Remote | Ветка |
|------|--------|--------|
| `researcher/` (корень) | часто **нет** remote; локальный `.git` | `main` |
| этот репозиторий (`r-a`) | `github.com/Researcher-436345/r-a.git` | `develop` / `develop-aleksandr` |

`r-a` — **вложенный** git-репозиторий. Фронт коммитить/пушить из `r-a/`. Корень держит бэкенд, compose, доки.

SSH для GitHub: ключ обычно `~/.ssh/id_ed25519`. Remote лучше на `git@github.com:...`.

---

## 3. Как поднять локально

```bash
# 1) Инфра + Go API
cd /path/to/researcher
cp .env.example .env   # если ещё нет
docker compose up -d --build

# 2) Фронт
cd frontend
cp .env.example .env   # VITE_API_URL=http://localhost:8080
npm install
npm run dev -- --port 5173
```

| Сервис | URL |
|--------|-----|
| Frontend | http://localhost:5173 |
| API | http://localhost:8080 (`/health`, `/docs` нет — это Go) |
| MinIO API | http://localhost:**9002** (не 9000!) |
| MinIO Console | http://localhost:**9003** |
| Postgres | localhost:5432 (`researcher` / `researcher`) |
| Redis | localhost:6379 |

Проверка API: `curl http://localhost:8080/health` → `status: ok`.

---

## 4. Архитектура (как устроено сейчас)

```
Browser (5173)
    │  JWT Bearer
    ▼
Go API (:8080)  ──► Postgres (схема из Alembic)
    │           ──► Redis (asynq queue + feed cache)
    │           ──► MinIO (PDF), внутри Docker: minio:9000
    │
    └── GET /papers/{id}/pdf  ← стрим PDF через API
                               (браузер НЕ ходит на MinIO напрямую)

Go worker  ◄── asynq ──  process_arxiv_pdf / finalize_uploaded_pdf
```

### Почему PDF через API

Cursor на macOS часто занимает порты **9000/9002**. Signed URL на MinIO с хоста отдавал пустой 200 → «PDF could not be loaded».  
Решение: фронт качает `GET /papers/{id}/pdf` с JWT, делает `blob:` URL для PDF.js.

### Auth

- JWT HS256, claims: `sub` = user UUID, `type` = `access` | `refresh`
- Access ~30 мин, refresh ~14 дней
- Пароли: bcrypt  
- Контракт как у старого FastAPI — фронт не менялся по полям токенов

### LLM

- Env: `LLM_PROVIDER`, `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`
- Дефолт в примере: AITunnel (`api.aitunnel.ru`) — из РФ реалистичнее Gemini/OpenRouter
- Эндпоинты: `/papers/{id}/chat`, `/explain`, `/translate` (translate: LLM → fallback MyMemory)
- Без ключа или при гео-блоке ответ — текст ошибки в `reply`, не обязательно 5xx

---

## 5. Что готово (итерация 1)

| Область | Статус | Где смотреть |
|---------|--------|--------------|
| Docker Compose, health | ✅ | `docker-compose.yml`, `backend-go` |
| Auth API + UI login/register | ✅ | `backend-go/internal/httpapi`, `frontend/src/pages/auth` |
| Papers: arXiv / DOI / upload | ✅ | `store/papers.go`, `services/arxiv|crossref|pdfmeta` |
| Worker PDF | ✅ | `cmd/worker`, asynq |
| Дедуп DOI/arXiv/SHA-256 | ✅ | papers service |
| Library list / patch / delete | ✅ | library routes + UI |
| PDF reader (PDF.js) | ✅ | `reader-pdf-canvas-viewer.tsx` + blob через API |
| Annotations CRUD + UI | ✅ | notes tab, selection popup |
| Trending feed + Redis | ✅ | `services/feed.go`, home page |
| Chat UI + tokens из выделения | ✅ | `chat-composer`, чипы `стр. N · слова`, клик → прыжок |
| Translate selection | ✅ | popup «Перевод» |
| Title из PDF при upload | ✅ | pdfmeta (Go) / pypdf (Python legacy) |
| Go вместо FastAPI в compose | ✅ | api/worker images from `backend-go` |

### User story

| Шаг | OK? |
|-----|-----|
| Регистрация / вход | ✅ |
| Добавить arXiv/DOI/PDF | ✅ |
| Библиотека | ✅ |
| Открыть PDF | ✅ (через API stream) |
| Заметка на выделении | ✅ |
| Спросить AI | 🟡 код есть, провайдер из РФ часто нет |
| Trending + в библиотеку | ✅ |

---

## 6. Что не готово / дыры

| Тема | Приоритет | Комментарий |
|------|-----------|-------------|
| **EPIC-05 Проекты** | P2 | Сайдбар — моки (`sidebar.tsx`) |
| **EPIC-09 Web-search** | P2 | AskAiBox UI есть, бэка нет |
| **EPIC-10 Теги** | P3 | status/favorite в API частично, UI тегов нет |
| **EPIC-08 довести** | P1 | нет `chat_messages` в БД; explain из попапа слабо; LLM |
| **EPIC-11** | P0 остаток | Similar tab — моки; проекты — моки |
| История чата | — | только local state во фронте |
| Миграции на Go (goose) | — | всё ещё Alembic из `backend/` |
| Удаление Python-кода | — | можно после goose + уверенности в Go |
| Свободные заметки без выделения | backlog | идеи в `roadmap.md` §13 |

---

## 7. Ключевые файлы

**Go**

- `backend-go/cmd/api/main.go` — HTTP  
- `backend-go/cmd/worker/main.go` — asynq  
- `backend-go/internal/httpapi/server.go` — все роуты  
- `backend-go/internal/store/` — SQL  
- `backend-go/internal/storage/s3.go` — MinIO  
- `backend-go/internal/services/` — arxiv, crossref, feed, llm, translate, pdfmeta  

**Frontend**

- `frontend/src/features/library/api.ts` — library + `waitForPdfUrl` / blob  
- `frontend/src/pages/reader/reader-page.tsx` — ридер  
- `frontend/src/features/reader/components/` — popup, chat, PDF canvas  
- `frontend/src/shared/api/client.ts` — JWT + refresh  

**Схема БД**

- `backend/alembic/versions/` — users → papers/library → annotations  

---

## 8. Известные грабли

1. **Порты MinIO 9000/9002** — Cursor может слушать те же порты на localhost. Не полагаться на presigned URL с хоста; PDF только через API.  
2. **LLM из России** — прямой Gemini и OpenRouter часто режут. AITunnel / DeepSeek / Ollama.  
3. **Два git** — не пушь фронт из корня как submodule «вслепую»; работай в `r-a/`.  
4. **Старый Python API** — не в compose как основной; код в `backend/app` может расходиться с Go. Новые фичи — в **Go**.  
5. **После `docker compose down`** volumes обычно остаются; данные Postgres/MinIO не обязаны пропадать.

---

## 9. С чего начинать новому человеку

### Если чинишь / доводишь итерацию 1

1. Прочитай этот файл + `STATUS.md`.  
2. Подними `docker compose up -d --build` и фронт.  
3. Пройди user story руками.  
4. Логичные следующие фичи без LLM:  
   - **EPIC-05** проекты (API + живой sidebar)  
   - дочистить моки Similar / Ask-box stub  
   - статусы чтения в UI (EPIC-10 light)

### Если нужен AI

1. Ключ AITunnel (или Ollama локально: `LLM_BASE_URL=http://host.docker.internal:11434/v1`).  
2. Потом: таблица `chat_messages`, история, explain из popup.

### Если дочищаешь Go

1. Перенести миграции на goose, убрать зависимость от Python image.  
2. Удалить неиспользуемый FastAPI код.  
3. Обновить `GO_MIGRATION.md` / `STATUS.md`.

### Если работаешь агентом в Cursor

- После заметных изменений обновляй `STATUS.md` скиллом **project-status**.  
- Не коммить `.env` с секретами.

---

## 10. API (кратко, паритет с фронтом)

Все приватные: `Authorization: Bearer <access>`.

- `POST /auth/register|login|refresh`, `GET /auth/me`  
- `POST /papers/arxiv|doi|upload`  
- `GET /papers/{id}`, `GET /papers/{id}/pdf-url`, `GET /papers/{id}/pdf`, `POST .../retry-pdf`  
- `POST /papers/{id}/chat|explain|translate`  
- `GET|PATCH|DELETE /library...`  
- `GET|POST /papers/{id}/annotations`, `PATCH|DELETE /annotations/{id}`  
- `GET /feed/trending`  

JSON — snake_case, как в старом FastAPI.

---

## 11. Контакты / ветки

- Ветка фронта для работы Александра: **`develop-aleksandr`** в `r-a`.  
- Документ обновлять при крупных сдвигах (смена стека, cutover, смена портов).

*Конец handoff. Удачи — сначала `docker compose up` и один проход user story.*
