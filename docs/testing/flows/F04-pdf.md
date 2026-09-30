# F04 — Обработка PDF и полного текста

**Приоритет:** P0. **Источник ожиданий:** `docs/PARSER.md`, `docs/HANDOFF.md` §4, `catalog/http.go` (контракт B3: `pdf_processing` / `pdf_unavailable`),
`cmd/worker/main.go`.
**Код:** `tests/integration/f04_pdf_test.go`.

После добавления статьи воркер скачивает PDF (`processing` → `ready` | `failed`), затем парсер извлекает текст
(`paper_documents`: `pending` → `ready` | `failed`), после чего `has_full_text: true` и чат/обзор видят полный текст.
Ридер опрашивает `/pdf-url`: 409 `pdf_processing` — ждать, всё остальное — конец ожидания.

Синтетические id фейка (`2350.NNNNN` здоровый, `2351.NNNNN` издатель отвечает 503, `2352.NNNNN` отдаёт HTML) дают
свежую статью на каждый прогон, поэтому сценарии сбоев не зависят от того, что уже лежит в базе стенда.

| # | Кейс | Given | When | Then | Статус |
|---|---|---|---|---|---|
| F04-01 | processing → ready → текст | свежая статья arXiv | добавить; опрашивать `/pdf-url`; ждать `has_full_text` | во время скачивания 409 `pdf_processing`; потом 200 `status: ready`; `has_full_text: true`; `/chat/context` показывает `has_full_paper: true`, `paper_tokens > 0` | ✅ |
| F04-02 | Текст по страницам | статья разобрана | чат с вопросом (fake-llm) | в промпт ушёл текст с маркерами страниц `<<<p=N>>>` и фразой `HYPOTHESIS-ON-PAGE-THREE` (пятистраничная фикстура умещается в один чанк, поэтому маркер один) | ✅ |
| F04-03 | Издатель отдаёт 503 | `2351.*` | добавить; ждать `failed`; `/pdf-url`; карточка | версия `failed` с `error_message`, где назван хост; `/pdf-url` → 422 `pdf_unavailable` с тем же текстом; `/pdf` → 422 | ✅ |
| F04-04 | Повтор после починки | издатель ответил 503 по правилу фейка, правило снято | `POST /retry-pdf` | 200, версия снова `processing`, затем `ready`, PDF байт-в-байт; текст разобран | ✅ |
| F04-05 | HTML вместо PDF | `2352.*` | добавить; ждать `failed`; `retry-pdf`; `find-fulltext` | `error_message` «did not return a PDF»; retry → 200 и снова `failed`; find-fulltext → 422 `not_found` | ✅ |
| F04-06 | retry на готовом PDF | статья `ready` | `POST /retry-pdf` | 200 без новой загрузки (фейк не получил новых запросов `/pdf/`) | ✅ |
| F04-07 | retry без источника | статья по DOI без файла | `POST /retry-pdf` | 422 `pdf_unavailable`, в `detail` совет про find-fulltext | ✅ |
| F04-08 | find-fulltext | DOI без открытого доступа; DOI с PDF | `POST /find-fulltext` | 422 `not_found`; 200 и PDF уже на месте | ✅ |
| F04-09 | Загрузка: processing → ready → текст | upload `paper-5p.pdf` под другим именем невозможен (дедуп) → `private-1p.pdf` | ждать | `ready`, `/pdf` = загруженные байты, `has_full_text: true` | ✅ |
| F04-10 | Ошибки маршрутов | — | `/pdf-url`, `/pdf`, `/retry-pdf` для несуществующего uuid и для не-uuid | 404 | ✅ |

**D-10** (парсер упал → документ не должен зависнуть в `pending`) проверяется в F12 — там останавливается контейнер парсера.
