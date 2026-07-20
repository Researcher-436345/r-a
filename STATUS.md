# Project Status — Researcher

> Краткий чеклист. **Полный контекст для нового человека:** [`HANDOFF.md`](./HANDOFF.md).  
> Обновлять скиллом `project-status` после значимых изменений.

**Последнее обновление:** 2026-07-20

## Итерация 1 — эпики

| Эпик | Приоритет | Статус | Комментарий |
|------|-----------|--------|-------------|
| EPIC-01 Инфра | P0 | ✅ | Docker Compose: Go API/worker + Alembic migrate |
| EPIC-02 Auth | P0 | ✅ | register/login/refresh, JWT (Go) |
| EPIC-03 Статьи | P0 | ✅ | upload / arXiv / DOI, asynq worker; title из PDF |
| EPIC-04 Библиотека + PDF | P0 | ✅ | library, ридер; PDF через `GET /papers/{id}/pdf` (blob) |
| EPIC-12 Дедуп | P1 | ✅ | unique DOI/arXiv, SHA-256 |
| EPIC-06 Заметки | P1 | ✅ | CRUD annotations + UI |
| EPIC-07 Trending | P1 | ✅ | feed + Redis |
| EPIC-11 Фронт без моков | P0 | 🟡 | сайдбар проектов и Similar — моки |
| EPIC-08 AI | P1 | 🟡 | chat/explain/translate; LLM из РФ часто недоступен |
| EPIC-05 Проекты | P2 | ❌ | не начато |
| EPIC-09 Web-search | P2 | ❌ | не начато |
| EPIC-10 Теги | P3 | ❌ | нет |

## Недавние изменения (changelog)

- **HANDOFF.md** — полный контекст проекта для продолжения работы
- PDF через API stream (MinIO порты конфликтовали с Cursor)
- Чипы чата: `стр. N · слова` + клик → прыжок; перевод выделения
- Title при upload PDF; бэкенд на Go в docker-compose
- Идеи свободных заметок — `roadmap.md` §13

## Roadmap этапы

| Этап | Статус |
|------|--------|
| 0 Техпрототип | 🟡 без GROBID |
| 1 Ядро библиотеки | 🟡 ~85% |
| 2 Библиография и поиск | ❌ |
| 3 Чтение и AI | 🟡 ридер+заметки ✅, AI нестабилен |
| 4 Связи / discovery | ❌ |
| 5 Beta | ❌ |

## Как обновить

Скилл `.cursor/skills/project-status/SKILL.md` — сверить код с `iteration-1.md`, обновить таблицы здесь и при крупных сдвигах — секцию в `HANDOFF.md`.
