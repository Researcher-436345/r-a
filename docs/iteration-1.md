    # Итерация 1 — рабочий прототип Researcher

    Документ описывает первую итерацию разработки: что хотим получить, зачем это нужно и какие конкретные задачи для этого делаем.

Связанные материалы: [roadmap.md](./roadmap.md), [HANDOFF.md](./HANDOFF.md) (актуальный статус и как продолжать), [STATUS.md](./STATUS.md), фронтенд в [r-a/frontend](./r-a/frontend/).

    ---

    ## Что мы хотим видеть в первой итерации

    Сейчас у нас есть **UI-прототип** — главная с лентой и Ask AI, PDF-ридер, сайдбар с проектами, панель ассистента и заметок. Всё это работает на **моках**: данные захардкожены в `api.ts`, `sidebar.tsx`, `reader-data.ts`. Бэкенда нет.

    **Итерация 1** — это переход от «красивой вёрстки» к **продукту, который можно поднять локально и пройти руками от начала до конца**.

    Мы хотим, чтобы любой из команды (и позже — тестовый пользователь) мог:

    1. Зарегистрироваться и войти в систему.
    2. Добавить научную статью — загрузив PDF или указав arXiv ID / DOI.
    3. Увидеть статью в **личной библиотеке** с автоматически подтянутыми метаданными (название, авторы, abstract).
    4. Открыть статью в **PDF-ридере** и прочитать её в браузере.
    5. **Выделить фрагмент текста** и сохранить его как заметку, привязанную к статье.
    6. **Задать вопрос AI** по этой статье и получить ответ в её контексте.
    7. На главной увидеть **ленту свежих статей** (trending) и добавить понравившуюся в библиотеку.

    Это не полный MVP из [roadmap.md](./roadmap.md). Мы сознательно откладываем BibTeX-импорт, экспорт библиографии, ГОСТ/APA, GROBID, полнотекстовый поиск, антиабьюз и мобильную версию. Итерация 1 доказывает **один сквозной сценарий**: сохранить → прочитать → заметить → спросить AI.

    ### Главный сценарий (user story)

    > Я регистрируюсь, добавляю статью с arXiv, она появляется в моей библиотеке. Открываю PDF, выделяю абзац и пишу заметку. Спрашиваю ассистента «объясни основную идею» — получаю ответ, опирающийся на эту статью. На главной вижу свежие работы по AI и могу сохранить любую к себе.

    ### Критерий завершения (Definition of Done)

    Итерация 1 считается завершённой, когда **все пятеро** без помощи разработчика:

    - выполняют `docker compose up` и `npm run dev` в `r-a/frontend`;
    - проходят сценарий из user story выше;
    - данные сохраняются в PostgreSQL и переживают перезапуск;
    - PDF хранится в object storage и открывается через signed URL.

    ---

    ## Что мы получим по итогам итерации

    ### Для пользователя (видимый результат)

    | # | Результат | Описание |
    |---|-----------|----------|
    | 1 | Регистрация и вход | Страницы `/login`, `/register`, сессия через JWT |
    | 2 | Личная библиотека | Список сохранённых статей со статусом чтения |
    | 3 | Добавление статей | Upload PDF, arXiv ID, DOI |
    | 4 | Карточка статьи | Название, авторы, abstract, год — из arXiv / Crossref |
    | 5 | PDF-ридер | Просмотр PDF в браузере (PDF.js), уже есть во фронте |
    | 6 | Заметки из выделения | Выделил текст → заметка с привязкой к странице |
    | 7 | AI-ассистент по статье | Чат и explain по выделенному фрагменту (DeepSeek / Kimi) |
    | 8 | Лента trending | Главная с реальными статьями, сортировка по свежести |
    | 9 | Проекты (дерево) | Группировка статей: проект → подпроект → статья |
    | 10 | «Хочу прочитать» | Отметка на карточке статьи в ленте и библиотеке |

    ### Для команды (технический результат)

    | # | Результат | Описание |
    |---|-----------|----------|
    | 1 | Docker Compose | Postgres, Redis, MinIO, API, worker — один `docker compose up` |
    | 2 | FastAPI backend | REST API с JWT-авторизацией |
    | 3 | Схема БД + миграции | Alembic, таблицы users, papers, library, annotations, projects |
    | 4 | Object storage | PDF в S3-совместимом хранилище (MinIO локально) |
    | 5 | Async worker | Celery: скачивание PDF, метаданные, статусы обработки |
    | 6 | Фронт без моков | `r-a/frontend` подключён к реальному API |
    | 7 | `.env.example` | Документированные переменные для локального запуска |

    ### Приоритеты, если не успеваем

    | Приоритет | Что входит | Без этого итерация не закрыта |
    |-----------|------------|-------------------------------|
    | **P0** | Auth, upload, library, PDF viewer | Да |
    | **P1** | Annotations, trending, LLM chat/explain | Нет, но сильно режет ценность |
    | **P2** | Projects tree, web-search | Нет |
    | **P3** | Теги, статусы чтения (фильтры) | Нет |

    ---

    ## Что не входит в итерацию 1

    | Отложено | Почему | Когда |
    |----------|--------|-------|
    | Антиабьюз, квоты, email verification | Пока нет внешних пользователей | Итерация 2–3 |
    | GROBID, OCR, извлечение references | Долго, не блокирует сквозной сценарий | Итерация 2 |
    | BibTeX / RIS импорт и экспорт | Этап 2 roadmap | Итерация 2 |
    | ГОСТ, APA, IEEE | Нужен citeproc, не блокирует чтение | Итерация 2 |
    | Полнотекстовый и семантический поиск | Нужен объём данных и GROBID | Итерация 2–3 |
    | Похожие статьи (вкладка Similar) | Discovery v2 | Итерация 3 |
    | Deep Research | Явно вне первого релиза | Итерация 4+ |
    | Мобильная версия | Десктоп-first | Итерация 5 |

    ---

    ## Распределение по команде (5 человек)

    Не все обязаны быть на каждой задаче. С вайбкодом разумно делить по **потокам** — у каждого свой вертикальный кусок, стыки по API-контракту.

    | Поток | Фокус | Эпики | Роль |
    |-------|-------|-------|------|
    | **A** | Инфра + auth (бэк) | EPIC-01, EPIC-02 (API) | DevOps / бэкенд |
    | **B** | Статьи + worker + дедуп | EPIC-03, EPIC-12 | Бэкенд |
    | **C** | Библиотека + проекты (бэк) | EPIC-04, EPIC-05 (API) | Бэкенд |
    | **D** | Фронтенд-интеграция | EPIC-11, UI для EPIC-02, 04, 05 | Фронт / fullstack |
    | **E** | Ридер: заметки + лента + AI | EPIC-06, EPIC-07, EPIC-08, EPIC-09 | Fullstack |

    **Правило стыков:** бэкенд отдаёт эндпоинт → пример `curl` в чат → D или E подключают UI в тот же день.

    **Если кто-то part-time:** потоки A + B можно слить на одного сильного бэкендера; E отдаёт trending (EPIC-07) в C, а AI (EPIC-08) делает позже.

    ---

    ## Эпики и задачи

    ### EPIC-01. Инфраструктура и локальный запуск

    **Зачем:** без единого способа поднять систему команда тратит время на «у меня не работает» вместо разработки.

    **Результат:** `docker compose up` поднимает все сервисы; API отвечает на `/health`.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-01-01 | Docker Compose | Сервисы: PostgreSQL 16, Redis, MinIO, FastAPI, Celery worker. Сеть и volumes. | P0 |
    | TASK-01-02 | Скелет FastAPI | Структура проекта: `app/main.py`, `routers/`, `models/`, `services/`, `schemas/`. CORS для фронта на `:5173`. | P0 |
    | TASK-01-03 | Alembic + первая миграция | Подключение к Postgres, автогенерация миграций, команда `alembic upgrade head` в README. | P0 |
    | TASK-01-04 | Health-check | `GET /health` → `{ "status": "ok", "db": "ok" }`. | P0 |
    | TASK-01-05 | `.env.example` | Все переменные: `DATABASE_URL`, `REDIS_URL`, `S3_*`, `JWT_SECRET`, `LLM_API_KEY`. | P0 |
    | TASK-01-06 | README по запуску | Пошагово: clone → `docker compose up` → `npm run dev` → открыть `localhost:5173`. | P0 |

    **Зависимости:** нет. Блокирует все остальные эпики.

    **Ответственный поток:** A.

    ---

    ### EPIC-02. Аутентификация и сессии

    **Зачем:** библиотека, заметки и PDF — личные данные. Без `user_id` нельзя изолировать пользователей и безопасно отдавать файлы.

    **Результат:** пользователь регистрируется, логинится, все приватные запросы требуют JWT.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-02-01 | Таблица `users` | Поля: `id` (UUID), `email` (unique), `password_hash`, `created_at`, `updated_at`. | P0 |
    | TASK-02-02 | `POST /auth/register` | Body: `{ "email", "password" }`. Хеширование bcrypt. Ошибка 409 при дубликате email. | P0 |
    | TASK-02-03 | `POST /auth/login` | Body: `{ "email", "password" }`. Ответ: `{ "access_token", "refresh_token", "token_type": "bearer" }`. | P0 |
    | TASK-02-04 | `POST /auth/refresh` | Обновление access token по refresh token. | P0 |
    | TASK-02-05 | JWT middleware | Dependency `get_current_user` на всех приватных роутах. 401 без токена. | P0 |
    | TASK-02-06 | Страница `/register` | Форма email + password. Валидация на клиенте. Редирект в библиотеку после успеха. | P0 |
    | TASK-02-07 | Страница `/login` | Форма email + password. Сохранение токена (localStorage или httpOnly cookie). | P0 |
    | TASK-02-08 | Protected routes | Без токена → редирект на `/login`. TanStack Router `beforeLoad` guard. | P0 |
    | TASK-02-09 | API client | Модуль `src/shared/api/client.ts`: base URL из `VITE_API_URL`, автоподстановка `Authorization` header. | P0 |

    **Зависимости:** EPIC-01.

    **Ответственный поток:** A (бэк), D (фронт).

    ---

    ### EPIC-03. Добавление и хранение статей

    **Зачем:** ядро продукта. Если статью нельзя добавить и сохранить — остальное бессмысленно.

    **Результат:** пользователь загружает PDF или указывает arXiv/DOI → статья создаётся в системе с метаданными.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-03-01 | Таблицы `papers`, `paper_versions`, `authors`, `paper_authors` | См. [roadmap.md §3](./roadmap.md). Минимум полей для карточки. | P0 |
    | TASK-03-02 | `POST /papers` — upload PDF | `multipart/form-data`, поле `file`. Сохранение в MinIO, запись `paper_version` со статусом `processing`. | P0 |
    | TASK-03-03 | `POST /papers` — arXiv | Body: `{ "arxiv_id": "2606.11087" }` или URL. Скачивание PDF с arXiv API. | P0 |
    | TASK-03-04 | `POST /papers` — DOI | Body: `{ "doi": "10.xxxx/..." }`. Метаданные из Crossref; PDF — если open access, иначе карточка без файла. | P0 |
    | TASK-03-05 | Celery worker: обработка PDF | Задача `process_paper`: SHA-256, обновление статуса `ready` / `failed`. | P0 |
    | TASK-03-06 | Метаданные из arXiv | Парсинг title, authors, abstract, published date при добавлении по arXiv ID. | P0 |
    | TASK-03-07 | Метаданные из Crossref | При добавлении по DOI: title, authors, journal, year, abstract (если есть). | P0 |
    | TASK-03-08 | Таблица `user_library_items` | Связь user ↔ paper. Поля: `status` (unread/reading/read), `favorite`, `added_at`. | P0 |
    | TASK-03-09 | UI: форма добавления статьи | Модалка или страница: drag-and-drop PDF, поле arXiv/DOI, кнопка «Добавить». | P0 |

    **Зависимости:** EPIC-01, EPIC-02.

    **Ответственный поток:** B (бэк), D (форма добавления).

    ---

    ### EPIC-04. Библиотека и просмотр PDF

    **Зачем:** пользователь должен не только добавить, но и вернуться к статье — это главная ценность хранения.

    **Результат:** список статей в библиотеке; клик → PDF открывается в ридере.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-04-01 | `GET /library` | Query: `page`, `limit`, `status`. Ответ: список `library_items` с вложенной карточкой paper. | P0 |
    | TASK-04-02 | `GET /papers/{id}` | Полная карточка: метаданные, авторы, статус обработки, статус в библиотеке. | P0 |
    | TASK-04-03 | `GET /papers/{id}/pdf-url` | Signed URL на PDF в MinIO (TTL 15 мин). 404 если PDF нет. | P0 |
    | TASK-04-04 | `DELETE /library/{paper_id}` | Удаление из личной библиотеки (не удаляет глобальный paper). | P1 |
    | TASK-04-05 | `PATCH /library/{paper_id}` | Обновление `status`, `favorite`. | P1 |
    | TASK-04-06 | UI: страница библиотеки | Список карточек (переиспользовать `PaperCard`). Фильтр по статусу. | P0 |
    | TASK-04-07 | UI: ридер на реальном PDF | `ReaderPdfViewer` загружает PDF по signed URL. Роут `/reader/$paperId`. | P0 |
    | TASK-04-08 | UI: кнопка «Хочу прочитать» | Toggle `favorite` на карточке в ленте и библиотеке. | P1 |

    **Зависимости:** EPIC-03.

    **Ответственный поток:** C (бэк), D (фронт).

    ---

    ### EPIC-05. Организация: проекты и иерархия

    **Зачем:** без структуры библиотека превращается в свалку. В UI сайдбара уже нарисовано дерево проектов (Notion-like).

    **Результат:** статьи группируются в проекты; дерево в сайдбаре живое.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-05-01 | Таблица `projects` | Поля: `id`, `user_id`, `parent_id` (nullable), `name`, `sort_order`. `parent_id = null` → корневой проект. | P2 |
    | TASK-05-02 | Таблица `project_items` | Связь `project_id` ↔ `paper_id`, `sort_order`. | P2 |
    | TASK-05-03 | `GET /projects` | Дерево проектов пользователя с вложенными items (названия статей). | P2 |
    | TASK-05-04 | `POST /projects` | Body: `{ "name", "parent_id"? }`. Создание проекта или подпроекта. | P2 |
    | TASK-05-05 | `POST /projects/{id}/items` | Body: `{ "paper_id" }`. Добавить статью в проект. | P2 |
    | TASK-05-06 | `DELETE /projects/{id}/items/{paper_id}` | Убрать статью из проекта (не из библиотеки). | P2 |
    | TASK-05-07 | UI: sidebar из API | Заменить `homeSidebarProjects` / `readerSidebarProjects` на данные из `GET /projects`. | P2 |
    | TASK-05-08 | UI: drag-and-drop или контекстное меню | «Добавить в проект» из карточки статьи. MVP: выпадающий список проектов. | P3 |

    **Зависимости:** EPIC-04.

    **Ответственный поток:** C (бэк), D (фронт).

    ---

    ### EPIC-06. Заметки из выделения в ридере

    **Зачем:** по [survey-analysis.md](./survey-analysis.md) люди много просматривают и мало глубоко читают. Заметки на выделениях — мост между «просмотрел» и «разобрался». UI вкладки Notes уже есть в `ReaderChatPanel`.

    **Результат:** выделил текст в PDF → заметка сохранилась → видна после перезагрузки.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-06-01 | Таблица `annotations` | Поля: `id`, `paper_id`, `user_id`, `page`, `rect` (JSON: x, y, w, h), `selected_text`, `note`, `color`, `created_at`. | P1 |
    | TASK-06-02 | `GET /papers/{id}/annotations` | Список заметок текущего пользователя по статье. | P1 |
    | TASK-06-03 | `POST /papers/{id}/annotations` | Body: `{ "page", "rect", "selected_text", "note" }`. | P1 |
    | TASK-06-04 | `PATCH /annotations/{id}` | Редактирование текста заметки. | P2 |
    | TASK-06-05 | `DELETE /annotations/{id}` | Удаление заметки. | P2 |
    | TASK-06-06 | UI: выделение текста в PDF | При выделении — попап «Добавить заметку» с полем ввода. | P1 |
    | TASK-06-07 | UI: вкладка Notes | `ReaderChatPanel` → вкладка Notes: данные из API вместо `readerNotes`. | P1 |

    **Зависимости:** EPIC-04 (ридер на реальном PDF).

    **Ответственный поток:** E.

    ---

    ### EPIC-07. Лента discovery (trending)

    **Зачем:** главная — входная точка (`AskAiBox` + `TrendingPapers`). Лента даёт повод зайти в продукт до того, как у пользователя своя библиотека.

    **Результат:** на главной реальные статьи; можно добавить в библиотеку в один клик.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-07-01 | `GET /feed/trending` | Query: `category` (default `cs.AI`), `limit` (default 20). Источник: arXiv API. | P1 |
    | TASK-07-02 | Сортировка / score | MVP: сортировка по дате публикации. Эвристика popularity: `submittedDate` + category weight. | P1 |
    | TASK-07-03 | Кеш в Redis | TTL 1 час, чтобы не ддосить arXiv при каждом открытии главной. | P2 |
    | TASK-07-04 | UI: заменить моки | `fetchTrendingPapers()` в `api.ts` → реальный `GET /feed/trending`. | P1 |
    | TASK-07-05 | UI: «Добавить в библиотеку» | Кнопка на `PaperCard` → `POST /papers` с arXiv ID → обновление списка. | P1 |

    **Зависимости:** EPIC-03 (добавление по arXiv).

    **Ответственный поток:** E.

    ---

    ### EPIC-08. AI по статье

    **Зачем:** AI — дифференциатор продукта, но только когда есть статья и контекст. Иначе это ChatGPT в обёртке.

    **Результат:** в ридере можно спросить про статью; explain по выделенному фрагменту.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-08-01 | Абстракция LLM-провайдера | Интерфейс `LLMProvider` с реализациями DeepSeek и Kimi. Выбор через env `LLM_PROVIDER`. | P1 |
    | TASK-08-02 | Таблица `chat_messages` | Поля: `id`, `paper_id`, `user_id`, `role` (user/assistant), `content`, `created_at`. | P1 |
    | TASK-08-03 | `POST /papers/{id}/chat` | Body: `{ "message": "..." }`. Сервер собирает контекст (title + abstract + история) → LLM → сохраняет ответ. | P1 |
    | TASK-08-04 | `POST /papers/{id}/explain` | Body: `{ "text": "выделенный фрагмент", "question"? }`. Короткий explain без полной истории. | P1 |
    | TASK-08-05 | Контекст для LLM | MVP: title + abstract из `papers`. Если есть извлечённый текст — добавить первые N токенов. | P1 |
    | TASK-08-06 | UI: чат в ридере | `ReaderChatPanel` → вкладка Assistant: отправка сообщений, отображение ответов. | P1 |
    | TASK-08-07 | UI: explain из выделения | При выделении текста — кнопка «Спросить AI» → `POST /papers/{id}/explain`. | P1 |

    **Как работает `POST /papers/{id}/chat`:**

    ```
    Клиент отправляет:
    POST /papers/abc-123/chat
    Authorization: Bearer <JWT>
    Body: { "message": "В чём основная идея?" }

    Сервер:
    1. Проверяет JWT → user_id
    2. Проверяет, что статья в библиотеке пользователя
    3. Загружает из БД: title, abstract, историю chat_messages
    4. Формирует prompt → вызывает LLM API
    5. Сохраняет ответ в chat_messages
    6. Возвращает: { "reply": "...", "message_id": "..." }
    ```

    Клиент **не отправляет PDF и не отправляет полный текст статьи** — только вопрос. Контекст сервер подтягивает сам.

    **Зависимости:** EPIC-04.

    **Ответственный поток:** E (бэк), D (чат UI в ридере — совместно с EPIC-11).

    ---

    ### EPIC-09. Web-search (упрощённый)

    **Зачем:** в UI уже есть переключатель Web / Deep Research в `AskAiBox`. На итерации 1 — базовый поиск, чтобы поле не было пустышкой.

    **Результат:** запрос в Ask-box с режимом Web возвращает результаты.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-09-01 | `POST /search/web` | Body: `{ "query": "..." }`. Внешний API: Tavily или Serper. | P2 |
    | TASK-09-02 | UI: подключить AskAiBox | Отправка формы → `POST /search/web` → отображение результатов (список ссылок + сниппеты). | P2 |
    | TASK-09-03 | «Сохранить в библиотеку» из результата | Если в результате есть DOI/arXiv — кнопка добавления. | P3 |

    **Зависимости:** EPIC-03.

    **Ответственный поток:** E.

    **Fallback:** если не успеваем — stub: поиск только по arXiv API вместо полноценного web-search.

    ---

    ### EPIC-10. Теги и статусы чтения

    **Зачем:** лёгкая организация без полноценных проектов. Уже заложено в модель `user_library_items`.

    **Результат:** фильтр библиотеки по статусу; опциональные теги на статьях.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-10-01 | Статусы чтения в UI | Dropdown на карточке: unread → reading → read. `PATCH /library/{paper_id}`. | P3 |
    | TASK-10-02 | Таблицы `tags`, `library_item_tags` | Теги per user, many-to-many со статьями в библиотеке. | P3 |
    | TASK-10-03 | `GET/POST/DELETE` для тегов | CRUD тегов, привязка к library item. | P3 |
    | TASK-10-04 | UI: теги на карточке | Chips с тегами, автокомплит при добавлении. | P3 |

    **Зависимости:** EPIC-04.

    **Ответственный поток:** кто успеет (flex).

    ---

    ### EPIC-11. Интеграция фронтенда с API

    **Зачем:** сквозная задача — убрать все моки и связать существующий UI с бэкендом.

    **Результат:** ни один экран не использует захардкоженные данные.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-11-01 | `VITE_API_URL` в `.env` | Локально: `http://localhost:8000`. Прокси в `vite.config.ts` (опционально). | P0 |
    | TASK-11-02 | React Query hooks | `useLibrary()`, `usePaper(id)`, `useTrending()`, `useAnnotations(id)`, `useProjects()`. | P0 |
    | TASK-11-03 | Удалить моки | Убрать `mockPapers` из `api.ts`, статику из `sidebar.tsx`, `reader-data.ts`. | P0 |
    | TASK-11-04 | Обработка ошибок | Toast / inline error при 401, 404, 500. Редирект на login при 401. | P1 |
    | TASK-11-05 | Loading states | Skeleton / spinner на карточках и в ридере пока грузятся данные. | P1 |
    | TASK-11-06 | Роутинг | `/reader/$paperId` вместо статичного `/reader`. Параметр из URL. | P0 |

    **Зависимости:** параллельно со всеми эпиками, финализируется в конце.

    **Ответственный поток:** D.

    ---

    ### EPIC-12. Базовая дедупликация

    **Зачем:** одна статья с одним DOI/arXiv не должна плодить дубликаты в системе и в S3.

    **Результат:** повторное добавление той же статьи → она попадает в библиотеку, а не создаёт новую запись.

    | ID | Задача | Детали | Приоритет |
    |----|--------|--------|-----------|
    | TASK-12-01 | Unique index на `papers.doi` и `papers.arxiv_id` | Nullable, но уникальные когда заполнены. | P1 |
    | TASK-12-02 | Логика при добавлении | Если paper с таким DOI/arXiv существует → создать только `user_library_item`, не новый paper. | P1 |
    | TASK-12-03 | Дедуп по SHA-256 | Если загруженный PDF совпадает по хешу с существующим → переиспользовать `paper_version`. | P2 |

    **Зависимости:** EPIC-03.

    **Ответственный поток:** B.

    ---

    ## Модель данных (минимум для итерации 1)

    ```
    users
    id, email, password_hash, created_at

    papers                          ← глобальные, shared между пользователями
    id, title, abstract, year, doi, arxiv_id, created_at

    paper_versions
    id, paper_id, pdf_key, sha256, source, status, created_at

    authors
    id, name, normalized_name

    paper_authors
    paper_id, author_id, position

    user_library_items              ← личная библиотека
    id, user_id, paper_id, status, favorite, added_at

    projects
    id, user_id, parent_id, name, sort_order

    project_items
    project_id, paper_id, sort_order

    annotations
    id, paper_id, user_id, page, rect, selected_text, note, color, created_at

    chat_messages
    id, paper_id, user_id, role, content, created_at

    tags                            ← P3
    id, user_id, name

    library_item_tags               ← P3
    library_item_id, tag_id
    ```

    ---

    ## API endpoints (сводка)

    | Метод | Путь | Эпик | Приоритет |
    |-------|------|------|-----------|
    | GET | `/health` | EPIC-01 | P0 |
    | POST | `/auth/register` | EPIC-02 | P0 |
    | POST | `/auth/login` | EPIC-02 | P0 |
    | POST | `/auth/refresh` | EPIC-02 | P0 |
    | POST | `/papers` | EPIC-03 | P0 |
    | GET | `/papers/{id}` | EPIC-04 | P0 |
    | GET | `/papers/{id}/pdf-url` | EPIC-04 | P0 |
    | GET | `/library` | EPIC-04 | P0 |
    | PATCH | `/library/{paper_id}` | EPIC-04 | P1 |
    | DELETE | `/library/{paper_id}` | EPIC-04 | P1 |
    | GET | `/projects` | EPIC-05 | P2 |
    | POST | `/projects` | EPIC-05 | P2 |
    | POST | `/projects/{id}/items` | EPIC-05 | P2 |
    | GET | `/papers/{id}/annotations` | EPIC-06 | P1 |
    | POST | `/papers/{id}/annotations` | EPIC-06 | P1 |
    | GET | `/feed/trending` | EPIC-07 | P1 |
    | POST | `/papers/{id}/chat` | EPIC-08 | P1 |
    | POST | `/papers/{id}/explain` | EPIC-08 | P1 |
    | POST | `/search/web` | EPIC-09 | P2 |

    ---

    ## План по неделям

    ### Неделя 1 — фундамент

    | Поток | Задачи |
    |-------|--------|
    | A | TASK-01-* (Docker, FastAPI, миграции), TASK-02-01..05 (auth API) |
    | B | Ожидает схему → TASK-03-01 (таблицы papers) |
    | C | Ожидает auth → черновик EPIC-04 (схема library) |
    | D | TASK-02-06..09 (login/register UI, API client), TASK-11-01 |
    | E | API-контракты, OpenAPI-черновик, помощь A с README |

    **Milestone:** можно залогиниться, API отвечает.

    ### Неделя 2 — ядро

    | Поток | Задачи |
    |-------|--------|
    | A | Code review, помощь по интеграции |
    | B | TASK-03-02..08 (upload, worker, metadata), TASK-12-* (dedup) |
    | C | TASK-04-01..03 (library API), TASK-04-05 (PATCH library) |
    | D | TASK-03-09, TASK-04-06..07 (UI библиотеки и ридера) |
    | E | TASK-07-01..02 (trending backend) |

    **Milestone:** добавил статью → вижу в библиотеке → открываю PDF.

    ### Неделя 3 — чтение и организация

    | Поток | Задачи |
    |-------|--------|
    | B | Помощь E / фиксы worker |
    | C | TASK-05-* (projects API) |
    | D | TASK-05-07 (sidebar), TASK-11-03 (убрать моки) |
    | E | TASK-06-* (annotations E2E), TASK-07-04..05 (trending UI) |

    **Milestone:** заметки работают, лента живая.

    ### Неделя 4 — AI и финализация

    | Поток | Задачи |
    |-------|--------|
    | A | Деплой, фиксы docker, помощь с E2E |
    | B | Фиксы papers pipeline |
    | C | Фиксы library / projects |
    | D | TASK-08-06..07 (wire chat UI), TASK-11-04..05 (errors, loading) |
    | E | TASK-08-* (LLM chat/explain), TASK-09-* (web-search, если успеваем) |
    | Все | E2E прогон user story, фиксы, README |

    **Milestone:** полный сценарий из Definition of Done.

    ---

    ## Стек

    | Слой | Технология | Комментарий |
    |------|------------|-------------|
    | Frontend | React, TypeScript, Vite | Уже есть в `r-a/frontend` |
    | Routing | TanStack Router | Уже есть |
    | Data fetching | TanStack Query | Уже есть |
    | PDF | PDF.js | Уже есть |
    | Backend | FastAPI (Python) | С нуля |
    | БД | PostgreSQL 16 | |
    | Очередь | Redis + Celery | |
    | Storage | MinIO (локально) | S3-совместимый |
    | LLM | DeepSeek / Kimi API | Абстракция провайдера |
    | Метаданные | arXiv API, Crossref | |

    ---

    ## Риски итерации 1

    | Риск | Митигация |
    |------|-----------|
    | Бэкенд не готов — фронт блокирован | Договориться на OpenAPI-контракт в день 1; фронт может начать с mock server |
    | PDF плохо рендерится | PDF.js уже работает в прототипе; тестировать на 5–10 разных PDF |
    | LLM дорого / медленно | Лимит длины контекста (abstract only на MVP); один провайдер |
    | arXiv rate limit | Кеш trending в Redis |
    | Команда не успевает | Резать P2/P3: projects, web-search, теги |
