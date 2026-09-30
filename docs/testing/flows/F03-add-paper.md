# F03 — Добавление статьи

**Приоритет:** P0. **Источник ожиданий:** `docs/HANDOFF.md` §5 (arXiv / DOI / upload, дедуп DOI/arXiv/SHA-256, title из PDF),
`docs/iteration-1.md` (статья в «Хочу прочитать»), `docs/STATUS.md` EPIC-03/12, `catalog/url.go` (from-url: 400 / 422 `not_a_paper` / 422 `not_found` / 502).
**Код:** `tests/integration/f03_add_test.go`.

Четыре способа положить статью в библиотеку: arXiv id, DOI, файл PDF, ссылка. Во всех случаях статья оказывается
в папке «Хочу прочитать» со статусом `unread`, а повторное добавление не создаёт дубликат.

DOI на стенде отвечает поддельный Crossref/OpenAlex (`fakes/scholar/catalog.go`, `DOIs`): `10.5555/fixture.metadata-only`
(без открытого доступа), `10.5555/fixture.open-access-pdf` (OpenAlex знает PDF издателя), `10.5555/fixture.arxiv-twin`
(OpenAlex ведёт на arXiv `2301.00003`), `10.5555/fixture.journal` (это журнал, не статья). Синтетические DOI `10.5555/closed.NNNNN`
(только метаданные) и `10.5555/oa.NNNNN` (PDF через OpenAlex) дают свежую запись на каждый прогон.

| # | Кейс | Given | When | Then | Статус |
|---|---|---|---|---|---|
| F03-01 | arXiv → библиотека | вошедший пользователь | `POST /papers/arxiv` `2301.00001` | 201; заголовок, авторы, abstract из arXiv; `venue: arXiv`; в библиотеке со статусом `unread` в папке «Хочу прочитать» | ✅ |
| F03-02 | arXiv не дублируется | статья добавлена | тот же id как `2301.00001v2`, как URL abs/pdf, как `arXiv:2301.00001`; повторно | тот же `id`; в библиотеке одна строка | ✅ |
| F03-03 | `add_to_library: false` | — | `POST /papers/arxiv` c флагом | 201, статьи нет в библиотеке | ✅ |
| F03-04 | DOI без открытого доступа | Crossref знает DOI, OpenAlex — нет | `POST /papers/doi` | 201; заголовок/авторы/год/venue из Crossref; `latest_version.source: doi`, `status: ready`, без `pdf_key`; `/pdf-url` → 422 `pdf_unavailable` про открытый доступ; в библиотеке | ✅ |
| F03-05 | DOI с PDF через OpenAlex | OpenAlex даёт `pdf_url` издателя | `POST /papers/doi` | 201; `latest_version.source: web_pdf`; PDF скачан байт-в-байт | ✅ |
| F03-06 | DOI, известный как arXiv | OpenAlex ведёт на `arxiv.org/abs/2301.00003` | `POST /papers/doi` | 201; `arxiv_id: 2301.00003`, `source: arxiv` — тот же paper, что и по arXiv | ✅ |
| F03-07 | DOI не дублируется, формы записи | статья по DOI добавлена | `https://doi.org/…`, `doi:…`, ВЕРХНИЙ регистр | тот же `id` | ✅ |
| F03-08 | Ошибки DOI | — | `not-a-doi`; `10.5555/nope`; `10.5555/fixture.journal` | 400; 422 `not_found`; 422 `not_a_paper` | ✅ |
| F03-09 | Загрузка PDF | файл `private-1p.pdf` под именем `my_reading-notes.pdf` | `POST /papers/upload` | 201; `source: upload`, `processing` → `ready`; заголовок из имени файла `my reading notes`; в библиотеке | ✅ |
| F03-10 | Дедуп загрузки по SHA-256 | тот же файл у второго пользователя | `POST /papers/upload` | 201 с тем же `id`; у обоих в библиотеке | ✅ |
| F03-11 | Не PDF | `.txt`; пустой файл | `POST /papers/upload` | 400 «Only PDF files…»; 400 | ✅ |
| F03-12 | HTML под именем `.pdf` `[assumption]` | `not-a-pdf.pdf` | `POST /papers/upload` | 400 — файл без сигнатуры `%PDF-` не принимается | 🔴 **D-13**: принимается, 201 |
| F03-13 | Ссылка: arXiv abs / pdf, doi.org | — | `POST /papers/from-url` | 201 и тот же paper, что по id | ✅ |
| F03-14 | Ссылка: не статья / мусор | — | `arxiv.org/list/cs.AI/recent`; `not a url`; `ftp://…` | 422 `not_a_paper`; 400; 400 | ✅ |
| F03-15 | Ссылка на неизвестную статью arXiv | — | `arxiv.org/abs/2301.99999` | 404/422 с `detail`, не 5xx (та же причина, что D-11) | ✅ |
| F03-16 | Троттлинг добавления | лимит 60/мин на пользователя | 61 × `POST /papers/arxiv` | первые 60 отвечают по существу, 61-й → 429 + `Retry-After` | ✅ |

**Не проверяется:** ссылка на HTML-страницу издателя с `citation_pdf_url` (поддельные хосты классифицируются раньше
запроса страницы); Europe PMC как источник текста (фейк отвечает 404).
