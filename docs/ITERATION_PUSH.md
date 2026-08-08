# Iteration push notes

Last refreshed: 2026-08-08
Branch: develop-aleksandr
Repo: Researcher-436345/r-a

## One-liner

Научная библиотека + PDF-ридер. В этом пуше: reader UX (fit-to-width, sticky selection, цвета), лента **New/Hot/Popular**, цитирования через **OpenAlex** (без ключа; S2 key — TODO в `.env.example`).

Подробнее: [HANDOFF.md](./HANDOFF.md), чеклист: [STATUS.md](./STATUS.md).

## How to run

```bash
cd r-a   # канон для GitHub
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
| MinIO API | http://localhost:**9002** |
| MinIO UI | http://localhost:9003 |
| Postgres | localhost:5432 |
| Redis | localhost:6379 |

Compose: Go `api` + `worker`, SQL `migrate` из `migrations/`.

Тест-аккаунт (если сид есть): `test@researcher.local` / `testpass123`.

## Done this iteration / currently working

### Reader UX (этот пуш)

- **Fit-to-width**: PDF масштабируется под ширину области; `Fit` в тулбаре; `+/−` — ручной zoom
  - `frontend/src/features/reader/components/reader-pdf-viewer.tsx`
- **Resizable chat**: drag-handle между PDF и чатом; ширина в `localStorage` (`researcher.reader.chatWidth`)
  - `frontend/src/pages/reader/reader-page.tsx`
- **Selection popup**:
  - не закрывается при ресайзе сплита
  - rect в долях страницы (0–1) → переживает zoom/resize
  - 5 пастельных цветов хайлайта → сохраняются в `annotations.color`
  - `frontend/src/features/reader/components/reader-selection-popup.tsx`
  - `frontend/src/features/reader/highlight-colors.ts`
- Чат-баблы: chip’ы без overflow (`reader-chat-bubble__body`)
- PDF scroll: frame-wrap `block` + `margin-inline: auto` у страниц (без обрезания слева)

### Feed / citations

- Sort `new|hot|popular` на `GET /feed/trending?sort=`
  - new: arXiv `submittedDate`
  - hot: `lastUpdatedDate` + скор от `updated`
  - popular: rising-скор + переранжирование по `citation_count` когда есть
- Citation enrich: OpenAlex batch (`doi:10.48550/arXiv.{id}`), Redis `cite:v1:*`
- UI: бейдж цитирований только если count **> 0**; фейковый popularity 896 убран
- Sort control закреплён справа, кнопки равной ширины (не прыгают от subtitle)
- **TODO:** `SEMANTIC_SCHOLAR_API_KEY` в `.env` — см. `.env.example`, `backend/README.md`

### Backend (уже на ветке)

- Modular monolith: `internal/app` + `platform` + `modules/{identity,catalog,library,annotations,feed,assistant}`
- Empty library: `"items": []`; PDF meta: не брать outline `/Title`, UTF-16, arXiv lookup

## Not done / known gaps

- EPIC-05 проекты (sidebar моки)
- EPIC-09 web-search — ветка `feature/web-search` (Ilia), не смержена
- EPIC-10 теги
- EPIC-08: нет `chat_messages` в БД; LLM из РФ нестабилен
- Similar tab — моки
- Ветка `papper_chat` (Gleb) — отдельный assistant-прототип, не влита
- Сохранённые annotation `rect` всё ещё в **px** на момент сохранения (старые заметки после сильного zoom могут «плыть»); текущее выделение — ratio
- Цитирования: без S2-ключа покрытие свежих arXiv слабое (OpenAlex часто 404) — нужен `SEMANTIC_SCHOLAR_API_KEY`

## Architecture snapshot

```
frontend → JWT → Go API (:8080)  [cmd/api + internal/app]
                → modules: identity / catalog / library / annotations / feed / assistant
                → platform: Postgres / Redis(asynq+cache) / MinIO
                → GET /papers/{id}/pdf streams file
worker ← asynq ← process_arxiv_pdf | finalize_uploaded_pdf  [uses catalog]
```

Правила границ: `backend/README.md`.

## Pitfalls

1. Cursor слушает 9000/9002 → PDF только через API stream.
2. LLM: Gemini/OpenRouter из РФ часто блок; AITunnel/Ollama.
3. Два git: пушь из `r-a/`, не из parent `researcher/`.
4. Старый compose в parent может занять порты — `docker compose down` там перед `r-a`.
5. Перед `git push` хук требует сегодняшний `Last refreshed` в этом файле.
6. Parent `researcher/docs/ITERATION_PUSH.md` тоже держать в синхроне, если хук смотрит cwd parent.

## Suggested next tasks

1. Смержить/перенести `feature/web-search` в `modules/research`
2. EPIC-05: projects API + живой sidebar
3. chat_messages + история чата в `assistant`
4. Нормализовать annotation rect в БД (ratio) + миграция/совместимость
5. Explain из selection popup
6. Дочистить моки Similar / Ask-box

## API surface

`GET /feed/trending?category=&limit=&sort=new|hot|popular` → items с опциональным `citation_count` / `citation_source`.

Annotations: `color`. Остальное без breaking changes.

`/auth/*`, `/papers/{…}`, `/library/*`, `/papers/{id}/annotations`, `/annotations/{id}`, `/feed/trending`

## Files to look at first

- `backend/internal/modules/feed/service.go`
- `backend/internal/modules/feed/citations.go`
- `frontend/src/features/papers/components/trending-papers.tsx`
- `frontend/src/features/papers/components/paper-card.tsx`
- `.env.example` (S2 key TODO)
- `frontend/src/pages/reader/reader-page.tsx`
- `frontend/src/features/reader/components/reader-selection-popup.tsx`
- `docs/HANDOFF.md`, `docs/STATUS.md`
