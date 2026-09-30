# План тестирования Researcher

**Обновлено:** 2026-10-01
**Статус:** мастер-план утверждён; декомпозиция по итерациям — в `flows/`.

## 1. Зачем

Автотестов, проходящих пользовательский сценарий целиком, в проекте нет. Есть 26 юнит-файлов в Go
(identity, утилиты catalog, промпты assistant), pytest у `services/websearch`, один node-скрипт у фронта.
Нет CI, нет интеграционных тестов с реальными Postgres/Redis/MinIO, воркер PDF не покрыт.

Цель — проверять **ожидаемое поведение продукта** по сценариям пользователя, а не закреплять текущее поведение
кода. Расхождение теста с кодом — это «баг или уточнение спеки», а не повод править тест
(см. [DISCREPANCIES.md](./DISCREPANCIES.md)).

## 2. Принципы

1. **Оракул — спецификация, не код.** Источники ожидаемого поведения по приоритету:
   `docs/iteration-1.md` (user story + DoD) → `docs/HANDOFF.md` §4/§10 → `docs/STATUS.md`,
   `docs/SERVER_RECOVERY.md`, `docs/PARSER.md`, `docs/SERVICES.md`, `backend/README.md` →
   `frontend/examples/v3/*.dc.html` → `frontend/scripts/README.md`. Где спека молчит, ожидание формулируется
   в кейсе явно и помечается `[assumption]`.
2. **Чёрный ящик снаружи, детерминизм внутри.** Тесты ходят только через gateway (`:8081`) теми же запросами,
   что шлёт фронт. Браузер не используется. LLM, Perplexity, arXiv/Crossref/OpenAlex/Europe PMC и SMTP
   заменяются управляемыми фейками. Parser, translator, worker, Postgres, Redis, MinIO — настоящие.
3. **Кейс = Given / When / Then + источник ожидания.** Упавший из-за расхождения кейс не переписывается под
   код: заводится запись в `DISCREPANCIES.md` с решением (баг / изменить спеку / отложить).
4. **Два профиля:** `mocked` (по умолчанию, детерминированный, для CI) и `live` (реальные ключи LLM и реальный
   arXiv; редкий ручной smoke, только happy path).

## 3. Виды тестов и что от них получаем

| Вид | Что проверяет | Что даёт | Инструмент |
|---|---|---|---|
| **Интеграционные (основа)** | сценарий пользователя целиком через настоящий стек в докере | ответ на вопрос «работает ли продукт»; ловит поломки, которые видит пользователь | Go-пакет `tests/integration`, тег `integration`, `testify/require` |
| **Юнит (точечно)** | одну функцию в изоляции, много вариантов входа | быстрая проверка сложной логики, вскрытой сценарием (разбор ссылок, подсчёт токенов, склейка обрезанного ответа) | существующие `go test ./...`, `pytest` |

Осознанно **не проверяется**: отрисовка PDF, попап выделения, редиректы гостя в UI, настройки в localStorage.
При необходимости добавляется Playwright поверх того же стенда.

## 4. Структура в коде

```
tests/                      # отдельный Go-модуль, бэкенд не импортирует (чёрный ящик)
  integration/              # один файл на сценарий, //go:build integration
    main_test.go            # TestMain: ждёт /health стенда; helpers guest/user/onlyCall
    f01_guest_test.go  f02_auth_test.go  f03_add_test.go  f04_pdf_test.go  f05_library_test.go
    f06_translate_notes_test.go  f07_chat_test.go  f08_summary_test.go  f09_search_test.go
    f11_access_test.go  f12_degradation_test.go
  client/                   # «искусственный пользователь»: Session (cookie + access-токен, как вкладка
                            # браузера), auth, papers (arXiv/DOI/URL/upload, ожидания PDF и текста),
                            # library (папки), annotations, chat, summary, search, sse, mailpit,
                            # fakes (скрипты fake-llm, правила и журнал fake-scholar), infra (Redis/БД),
                            # compose (stop/start контейнеров — только F12)
  fakes/llm, fakes/scholar  # двойники; fakes/cmd/* — бинарники; fakes/Dockerfile
  fixtures/                 # PDF + gen_pdf.py; embed через пакет fixtures
docker-compose.test.yml     # overlay стенда
Makefile                    # make test-integration | stand-up | test-integration-run | stand-down
```

Пример (реальный тест из F01):

```go
func TestF01_GuestReadsArxivPaper(t *testing.T) {
    g := guest(t)
    paper, resp := g.OpenArxiv("2301.00001")
    require.Equal(t, 201, resp.Status, resp)
    ready := g.WaitForPdfReady(paper.ID, 90*time.Second)   // 409 pdf_processing → 200
    require.Equal(t, 200, ready.Status, ready)
    require.Equal(t, fixtures.PDF("paper-5p.pdf"), g.PDF(paper.ID).Body)
}
```

Каждый тест создаёт своего пользователя со случайным email, поэтому тесты независимы. Утверждения —
`testify/require`. Тест, упирающийся в расхождение, **падает** и остаётся красным до решения по записи D-NN
в `DISCREPANCIES.md`; в комментарии к проверке стоит ссылка на номер. Никаких skip-пометок в коде (решение
Глеба): красный набор честнее, чем скрытые исключения.

## 5. Стенд, фейки, фикстуры

**Стенд** — `docker-compose.test.yml` поверх основного compose, проект `r-a-test`, порты на хосте 1xxxx
(gateway 18081, Mailpit 18025, Postgres 15433, Redis 16379, fake-llm 18094, fake-scholar 18093), поэтому dev-стек
может оставаться запущенным. `env_file: .env` базового compose отключён: все переменные заданы в overlay явно,
ключи разработчика на стенд не попадают. Фронт не поднимается. Код бэкенда стенд не меняет.

**Фейки** — двойники внешних сервисов; сервер не отличает их от настоящих:

- **fake-llm** (`tests/fakes/llm`) — OpenAI-compatible `/v1/chat/completions`, обычный и стриминговый. Адрес
  подставляется штатными переменными `LLM_BASE_URL` / `WEBSEARCH_LLM_BASE_URL`. Записывает все входящие запросы
  (`GET /_control/requests?match=…`) — тест проверяет, что именно ушло в LLM. Сценарий ответа задаётся
  `POST /_control/script` (`match` — подстрока в сообщениях, `reply`, `status` для ошибок, `finish_reason`,
  задержки, `times`, `empty` — пустой ответ, `sources` — `url_citation`-аннотации, как у Perplexity); без сценария
  отвечает `[fake-llm] <последнее сообщение пользователя>`.
- **fake-scholar** (`tests/fakes/scholar`) — arXiv API (`/api/query`), PDF (`/pdf/{id}`), e-print, страница
  `/abs`, OpenAlex `/works`, Crossref/Europe PMC (404). Адреса в бэкенде зашиты, поэтому подмена идёт на уровне
  сети: контейнер имеет алиасы `arxiv.org`, `export.arxiv.org`, `api.openalex.org`, `api.crossref.org`, …,
  слушает :80 и :443 с сертификатом от собственного CA, а CA подсовывается Go-сервисам через `SSL_CERT_FILE`
  (общий том `test_certs`). Подсеть стенда — `203.0.113.0/24` (документационная, «публичная»), чтобы защита
  бэкенда от приватных адресов пропускала фейк. Каталог статей — `tests/fakes/scholar/catalog.go`
  (`2301.0000{1..5}` здоровые, `2399.99901` PDF → 503, `2399.99902` PDF → HTML) плюс **синтетические id** на каждый
  прогон: `2350.NNNNN` здоровый, `2351.NNNNN` PDF → 503, `2352.NNNNN` PDF → HTML (`client.NewArxivID`). Crossref/OpenAlex:
  `DOIs` в том же файле (`10.5555/fixture.*`) и синтетические `10.5555/closed.NNNNN` (без открытого доступа) /
  `10.5555/oa.NNNNN` (PDF издателя через OpenAlex, файл `paper-oa-1p.pdf`). Отказы и задержки по заказу:
  `POST /_control/rules` `{path_prefix, status, body, delay_ms, times}`; журнал запросов `GET /_control/requests?path_prefix=`.
- **Mailpit** — уже в compose; письма читаются через `GET /api/v1/search?query=to:"…"` и `/api/v1/message/{id}`,
  токены verify/reset берутся из ссылок в письме.

**Фикстуры** (`tests/fixtures`, генератор `gen_pdf.py`): `paper-5p.pdf` (на 3-й странице
`HYPOTHESIS-ON-PAGE-THREE`), `paper-1p.pdf`, `private-1p.pdf` (не совпадает с arXiv-фикстурами, иначе дедуп
по sha256 привяжет загрузку к публичной статье), `not-a-pdf.txt`, `not-a-pdf.pdf` (HTML).

**Служебные приёмы** (`tests/client/infra.go`, `compose.go`): сброс счётчиков троттлинга в Redis перед регистрацией
(иначе 5/15 мин на IP остановят набор); «перемотка времени» правкой `expires_at` в БД; статус `paper_documents`
(воспроизвести «текст ещё парсится» без гонки с парсером); сброс `version_id` у кэша обзора (то, что делает
перепарсинг); чтение `paper_documents.status` для D-10; `docker compose stop/start/restart` контейнеров стенда —
только в F12. Это единственные места, где тесты обходят API; сценарии используют их осознанно.

Что фейки дают, чего иначе нельзя: сценарии ошибок по заказу («arXiv недоступен», «у AI кончился баланс»,
«ответ оборвался»); проверка того, что сервер отправил в LLM; повторяемость без сети и ключей; ноль платных
запросов. Чего не проверяют: вменяемость реального провайдера и неизменность формата arXiv — профиль `live`.

## 6. Карта сценариев

Приоритет: P0 — без этого продукт не работает, P1 — ключевая ценность, P2 — вторичное.

| # | Сценарий | Приоритет | Кратко | Источник ожидания |
|---|---|---|---|---|
| F1 | Гость | P0 | лента trending; `arxiv/open` не трогает библиотеки; `GET /papers/{id}`, pdf, перевод — публичны; библиотека/заметки/чат/обзор/поиск → 401; чужой upload → 404 | backend/README «Guest access», SERVER_RECOVERY, `identity/public.go` |
| F2 | Регистрация, вход, сессии | P0 | register → письмо → verify → автологин; логин до verify → 403; refresh-ротация и reuse → отзыв всех; forgot/reset; throttle 429; enumeration-защита; одноразовые токены с TTL | HANDOFF §4, §10; `identity/http_test.go` |
| F3 | Добавление статьи | P0 | arXiv / DOI / upload / from-url → статья в «Хочу прочитать», unread; дедуп по arXiv/DOI/sha256; DOI без OA → карточка без файла; ошибки 400/422/502/429; title из PDF | HANDOFF §5, iteration-1, STATUS EPIC-03/12 |
| F4 | Обработка PDF и полного текста | P0 | processing → ready → parse → `has_full_text`; 409 `pdf_processing` во время; `/pdf` отдаёт те же байты; failed → 422 + читаемое сообщение; retry-pdf; find-fulltext; parse не зависает в pending | PARSER.md, HANDOFF §4, `catalog/http.go` |
| F5 | Библиотека и папки | P1 | системные папки; create/delete, дубль 409; удаление папки → «Другое», read; перенос в системную папку меняет статус; удаление статьи — только своя ссылка | `006_library_folders.sql`, `library/store.go` |
| F6 | Перевод и заметки | P1 | перевод: языки, 400/413/429/502/503, стрим, гость; заметки: CRUD, только свои, PATCH только `note`, 400 на невалидные, связь с сообщением чата | HANDOFF §5, backend/README «Translation», `annotations/http.go` |
| F7 | Чат по статье | P1 | стрим и обычный ответ; в промпт ушёл полный текст с маркерами страниц; пара сообщений сохраняется только при успехе; приватность; модели; `context_text`; ошибки 402/502/503; explain | STATUS EPIC-08, HANDOFF §4 LLM |
| F8 | Обзор статьи | P1 | 404 до генерации; все секции; кеш общий; 425 при парсинге; 409 при параллельной генерации; `stale`/`force`; `lang`; дозапрос обрезанного; пустой не кешируется | ITERATION_PUSH, PARSER.md §5, `008` |
| F9 | Поиск и deep research | P1 | первое сообщение создаёт чат; события стрима; только ссылки на статьи; from-url из результата; список/удаление; чужой → 404; >20 000 → 400; ошибка провайдера | SERVICES.md, `searchapi/http.go` |
| F10 | Настройки UI | — | отложено: живут только во фронте | — |
| F11 | Границы доступа | P0 | чужое → 404; upload приватен; `X-User-Id` отбрасывается; internal-эндпоинты снаружи недоступны; JSON-лимит и unknown fields; CORS | backend/README, `gateway/handler_test.go` |
| F12 | Деградация | P2 | Redis down → fail-open; LLM 502/503; translator 429; parser down → failed, чат по абстракту; дедуп enqueue; перезапуск воркера | HANDOFF §4, `throttle.go`, `queue.go` |

Подробные кейсы каждого сценария — в `flows/F{n}-*.md` (появляются по итерациям).

## 7. Итерации

| Итерация | Содержание | Результат |
|---|---|---|
| **A. Фундамент** | `docker-compose.test.yml`, fake-llm, fake-scholar, Mailpit-хелпер, `tests/integration`, фикстуры, `make test-integration`; F1, F2 | стенд одной командой; гость и auth покрыты |
| **B. Контент** | F3, F4, F5 | «добавить → обработать → библиотека», включая failed/retry |
| **C. AI** | F6, F7, F8 | перевод, заметки, чат, обзор на fake-llm |
| **D. Поиск, сквозное** | F9, F11, F12; `live`-smoke; CI (`go test`, `pytest`, integration mocked) | полный набор |

Цикл каждой итерации: (1) `flows/F{n}-*.md` с кейсами → ревью → (2) автоматизация → (3) записи в
`DISCREPANCIES.md` → (4) решение по каждой.

## 8. Запуск

```sh
make test-integration                 # полный цикл: стенд → набор → стенд погашен (с удалением томов)
make stand-up                         # поднять стенд и оставить (первый раз собирает образы, несколько минут)
make test-integration-run             # прогнать набор по живому стенду (~10 с на F01–F02, ~12 мин весь набор)
make test-integration-run RUN=TestF02 # только один сценарий
make test-integration-run RUN='TestF05|TestF06'   # несколько (regexp; `$` в Makefile надо удваивать)
make test-integration-run GO_TEST_FLAGS='-count=1 -timeout 40m'   # без -v: только итог и упавшие тесты
make stand-logs SVC=worker            # логи сервиса стенда
make stand-down
```

По умолчанию `go test -v` с отфильтрованными строками `=== RUN`: каждый тест печатает `--- PASS` или `--- FAIL`
сразу по завершении, так что видно, где сейчас прогон.

Требуется Docker Compose ≥ 2.24 (`!override` в overlay) и Go ≥ 1.25 на хосте для самого набора.

Набор можно гонять повторно по живому стенду: сценарии, которым нужна свежая статья, минтят синтетические arXiv id
и DOI, а тексты для fake-llm помечают уникальным `client.Nonce()`. После пересборки `fake-scholar` контейнеры
`catalog`, `worker`, `feed` надо перезапустить: они читают CA фейка один раз при первом TLS-запросе.
Профиль `live` (реальные ключи, реальный arXiv) — итерация D.

## 9. Состояние

| Итерация | Статус | Итог |
|---|---|---|
| A. Фундамент: стенд, фейки, F1–F2 | ✅ 2026-10-01 | 26 тестов, 24 ✅, 2 🔴 по D-11 и D-12 — ждут решения ([flows/](./flows/)) |
| B. Контент: F3–F5 | ✅ 2026-10-01 | 36 тестов, 33 ✅, 3 🔴 по D-04, D-05, D-13 |
| C. AI: F6–F8 | ✅ 2026-10-01 | 33 теста, 31 ✅, 2 🔴 по D-03, D-06; D-09 (окно провайдера) — только в `live` |
| D. Поиск, сквозное: F9, F11, F12 | ✅ 2026-10-01 | 17 тестов, 16 ✅, 1 🔴 по D-14; D-10 закрыт (не воспроизводится) |
| D. `live`-smoke и CI | ⏳ | не начато |

Всего 107 тестовых функций (плюс подтесты), 99 ✅ и 8 🔴 — каждый красный привязан к записи в
[DISCREPANCIES.md](./DISCREPANCIES.md). Полный прогон по живому стенду — около 12 минут.
