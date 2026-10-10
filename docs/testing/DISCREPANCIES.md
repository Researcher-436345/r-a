# Журнал расхождений

Сюда попадает всё, где ожидаемое поведение (по документам или здравому смыслу) не совпало с тем, что делает код.
Тест под код не переписывается и не отключается: он падает, пока здесь не принято решение.

**Статусы:** `гипотеза` (замечено при разведке, тестом не подтверждено) → `подтверждено` (тест написан и
падает) → решение: `баг` / `изменить спеку` / `отложить` → `закрыто`.

| # | Сценарий | Ожидание | Факт | Статус | Решение |
|---|---|---|---|---|---|
| D-01 | стенд | API доступен на порту из документации (8080: HANDOFF, README, `.env.example`) | `docker-compose.yml` публикует gateway на 8081 | подтверждено (стенд собран на 8081) | |
| D-02 | стенд/F2 | `cp .env.example .env` даёт рабочую почту в Mailpit (HANDOFF §3) | `MAIL_ENABLED=false`, `SMTP_HOST=localhost` → письма в stdout identity | подтверждено (по файлу) | |
| D-03 | F6-09 | невалидный `color` заметки → 400 | `color` не валидируется; >16 символов → 500 `value too long for type character varying(16)` | подтверждено, падает `TestF06_AnnotationValidation/colour_longer_than_16` | |
| D-04 | F5-12 | `POST /library/{id}` без тела добавляет в папку по умолчанию | пустое тело → 400 «Invalid JSON body» | подтверждено, падает `TestF05_SaveWithoutBody` | |
| D-05 | F5-09 | невалидный `status` в фильтре `GET /library` → 400, как у PATCH | фильтр не валидируется: `status=bogus` → 200 и пустой список | подтверждено, падает `TestF05_ListFiltersAndPaging` | |
| D-06 | F6-12 | `source_chat_message_id` принимается только для своего сообщения | принадлежность пользователю не проверяется: чужой id сохраняется, 201 | подтверждено, падает `TestF06_AnnotationLinkedToChatMessage` | |
| D-07 | F11 | загруженный PDF остаётся приватным (backend/README) | пользователь проверяется по любой версии, гость — по latest; upload с добавленной публичной версией читаем всем вошедшим | гипотеза; базовая приватность подтверждена зелёным `TestF11_ForeignUploadIsInvisible` (в т.ч. после find-fulltext), смешанный случай «upload + публичная версия» на стенде не воспроизведён | |
| D-08 | F11 | internal-эндпоинты защищены `X-Internal-Token` | `/internal/papers/{id}/access` токен не проверяет; parser (8091) и websearch (8092) опубликованы на хост, websearch без токена при пустом `INTERNAL_TOKEN` | гипотеза | |
| D-09 | F7 (live) | ответ чата не обрезается на окне провайдера | `LLM_CONTEXT_TOKENS=120000` при окне 32k у маршрутизируемой модели (ITERATION_PUSH «Pitfalls») | гипотеза | |
| D-11 | F1-07 | arXiv не знает статьи → 404/422 с понятным текстом | `POST /papers/arxiv/open` (и `/papers/arxiv`) с несуществующим id → **500** `the paper was not found in open sources: arXiv has no paper …` (`catalog/http.go` не распознаёт `errUnsupportedArticleURL`) | подтверждено, падает `TestF01_OpenArxivBadIds/unknown_id` | |
| D-12 | F2-11 | «Выйти на всех остальных» оставляет текущее устройство в системе, даже если отозванный телефон потом откроет приложение `[assumption]` | refresh с явно отозванной cookie считается кражей токена → отзыв всех сессий, включая текущую (`identity/http.go` reuse detection не отличает `revoked_at` без `replaced_by`) | подтверждено, падает `TestF02_ActiveSessions` | |
| D-13 | F3-12 | HTML-файл под именем `.pdf` отклоняется на загрузке `[assumption]` | upload проверяет только расширение/Content-Type: файл без `%PDF-` принимается (201), воркер помечает версию `ready`, парсер потом падает | подтверждено, падает `TestF03_UploadRejectsHTMLNamedPDF` | |
| D-14 | F11-04 | access-токен с верной подписью, но несуществующим `sub` → 401 `[assumption]` (например, удалённый пользователь с ещё живым токеном) | gateway проверяет только подпись; сервисы доверяют `X-User-Id` и падают на первом же запросе к БД: `GET /library` → **500** (FK `library_folders_user_id_fkey`) | подтверждено, падает `TestF11_ForgedTokens/unknown_user,_right_secret` | |
| D-15 | F1-01, F1-02 | `GET /feed/trending` без токена → 200, непустой список со `arxiv_id` и `title` (F01-guest.md, HANDOFF §10, iteration-1 TASK-07-01) | после `ddf72f7` дефолтный `lang=ru` читает только заранее подготовленный снимок; снимок пишет цикл `WarmDescriptions`, который без `FEED_TRANSLATION_API_KEY`/`LLM_API_KEY` выключен совсем (`feed briefs disabled`), отката на живую ленту нет → **503** `feed_preparing` бессрочно; с `lang=en` та же лента отдаёт 200 | подтверждено, падают `TestF01_TrendingFeedWithoutLogin`, `TestF01_TrendingFeedIsCached` | |
| D-16 | F12-01, F12-02 | `GET /feed/trending?category=cs.LG` / `?category=cs.CV` → 200 или 502 (F12-degradation.md; `category` — свободный параметр, iteration-1 TASK-07-01) | `ddf72f7`: при дефолтном `lang=ru` любая категория кроме `cs.AI` → **400** «Prepared feed is available for cs.AI»; живая лента по другим категориям доступна только с явным `lang=en` | подтверждено, падают `TestF12_ArxivDown`, `TestF12_RedisDown` | |
| D-17 | F3-09 | загрузка `private-1p.pdf` под именем `my_reading-notes.pdf` → заголовок `my reading notes` из имени файла | код прав, набор связан порядком: `f01_guest_test.go:184` раньше грузит те же байты как `private-notes.pdf`, дедуп по SHA-256 (F03-10) возвращает уже существующую статью с заголовком `private notes`. В полном прогоне красный, при `RUN=TestF03` на чистом стенде зелёный | подтверждено, падает `TestF03_UploadPDF` (дефект набора, не кода) | |
| D-18 | стенд | `make stand-up` поднимает стенд одной командой | команда падает примерно в половине циклов `stand-down` → `stand-up` (5 из 11): `Error response from daemon: No such container: <id>` или `Container r-a-test-catalog-1  Error`, exit 1/2; повторный `make stand-up` всегда проходит, пауза после `stand-down` не помогает. Docker Desktop 27.4.0 / Compose 2.31.0, причина не локализована | подтверждено (воспроизводится, 5 из 11 циклов) | |
| D-10 | F12-04 | документ не остаётся `pending` при падении parser; обзор не отвечает 425 вечно | `TestF12_ParserDown` зелёный: при остановленном parser документ становится `failed` с текстом ошибки, чат идёт по аннотации, `POST /summary` → 200 | закрыто | не расхождение |

## Замечания по фронту (вне текущего плана, для сведения)

- Библиотека не показывает статус обработки и не обновляется сама.
- Сохранённые заметки не рисуются как подсветки на PDF после перезагрузки.
- Перевод всегда в `ru`; при выключенном мгновенном переводе перевести вручную нельзя.
- Нет кнопки «отправить письмо ещё раз», хотя `POST /auth/resend-verification` есть.
- `uploadPdf` идёт мимо общего API-клиента: нет refresh на 401, не применяется loopback-фикс URL.
- Нет кнопки остановить ответ в поиске.
- Similar-таб и сайдбар проектов — моки.
