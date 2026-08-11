# Project Status — Researcher

> Краткий чеклист. **Полный контекст для нового человека:** [`HANDOFF.md`](./HANDOFF.md).  
> Обновлять скиллом `project-status` после значимых изменений.

**Последнее обновление:** 2026-08-11

## Итерация 1 — эпики

| Эпик | Приоритет | Статус | Комментарий |
|------|-----------|--------|-------------|
| EPIC-01 Инфра | P0 | ✅ | Compose: gateway + domain services + worker/parser/translator + SQL migrations |
| EPIC-02 Auth | P0 | ✅ | register/login/refresh, JWT (identity service) |
| EPIC-03 Статьи | P0 | ✅ | upload / arXiv / DOI, asynq worker; title из PDF |
| EPIC-04 Библиотека + PDF | P0 | ✅ | library, ридер; PDF через API stream `GET /papers/{id}/pdf` (blob) |
| EPIC-12 Дедуп | P1 | ✅ | unique DOI/arXiv, SHA-256 |
| EPIC-06 Заметки | P1 | ✅ | CRUD + notes from chat / reply selection + jump back to message |
| EPIC-07 Trending | P1 | ✅ | feed + Redis |
| EPIC-11 Фронт без моков | P0 | 🟡 | сайдбар проектов и Similar — моки |
| EPIC-08 AI | P1 | 🟡 | full-text chat + SSE + cites; LLM зависит от ключа/баланса; нет RAG/span-highlight |
| EPIC-05 Проекты | P2 | ❌ | не начато (sidebar моки) |
| EPIC-09 Web-search | P2 | ❌ | ветка `feature/web-search`, не в main |
| EPIC-10 Теги | P3 | ❌ | нет |

## Недавние изменения (changelog)

- **2026-08-11 — notes scroll:** длинный список заметок скроллится, карточки не сжимаются
- **2026-08-11 — API split:** gateway + identity/catalog/library/annotations/assistant/feed как отдельные контейнеры; фронт по-прежнему `:8080`
- **2026-08-11 — notes UX:** Markdown в карточках заметок; длинные сворачиваются с «Показать ещё»
- **2026-08-11 — stream + page cites:** SSE `POST …/chat?stream=1`; промпт с `<<<p=N>>>` из chunks; кликабельные `[p.N «quote»]` → страница PDF (без RAG/span)
- **2026-08-11 — reader chat polish:** заметки из сообщения/выделения ответа; jump note→chat (`004`); одиночная цитата над инпутом; компактный context chip; фикс PDF selection overshoot
- **2026-08-10 — chat UX:** метр контекста, выбор модели (`LLM_MODELS`), KaTeX/markdown, plain-text paste, «Уточнить», ошибки LLM (402)
- **2026-08-10 — full-text chat:** parser + TeX-first; `paper_documents` (`003`); full text in prompt + rolling summary. См. [PARSER.md](./PARSER.md)
- **2026-08-10 — EPIC-08 paper chat:** `chat_messages` (`002`), `GET/POST …/chat`, explain; LLM → 502/503
- Backend: **microservices** behind gateway — identity, catalog, library, annotations, assistant, feed (+ parser, translator, worker). See [SERVICES.md](./SERVICES.md)
- Структура: markdown → `docs/`, Python backend удалён, Go в `backend/`, SQL в `migrations/`
- **HANDOFF.md** — полный контекст проекта для продолжения работы
- PDF через API stream (MinIO порты конфликтовали с Cursor)
- Чипы чата: `стр. N · слова` + клик → прыжок; перевод выделения
- Title при upload PDF; бэкенд на Go в docker-compose
- Идеи свободных заметок — `roadmap.md` §13

## Roadmap этапы

| Этап | Статус |
|------|--------|
| 0 Техпрототип | 🟡 PDF parser ✅ (PyMuPDF default; Docling optional); GROBID ещё нет |
| 1 Ядро библиотеки | 🟡 ~85% |
| 2 Библиография и поиск | ❌ |
| 3 Чтение и AI | 🟡 ридер+заметки ✅, full-text chat ✅, LLM зависит от провайдера |
| 4 Связи / discovery | ❌ |
| 5 Beta | ❌ |

## Как обновить

Скилл `.cursor/skills/project-status/SKILL.md` — сверить код с `iteration-1.md`, обновить таблицы здесь и при крупных сдвигах — секцию в `HANDOFF.md`.
