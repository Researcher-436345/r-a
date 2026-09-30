# F07 — Чат по статье

**Приоритет:** P1. **Источник ожиданий:** `docs/STATUS.md` EPIC-08, `docs/HANDOFF.md` §4 «LLM», `docs/PARSER.md` (полный текст в промпте,
маркеры страниц, скользящее резюме), `assistant/http.go`, `assistant/prompt.go`, `assistant/llm.go` (ошибки провайдера → 502/503).
**Код:** `tests/integration/f07_chat_test.go`.

Чат отвечает обычным JSON или стримом SSE (`?stream=1`, события `delta` → `done` | `error`). В промпт уходит карточка
статьи, полный текст с маркерами `<<<p=N>>>`, выделенный фрагмент (`context_text`) и история. Пара «вопрос-ответ»
сохраняется только после успешного ответа; история личная.

| # | Кейс | Given | When | Then | Статус |
|---|---|---|---|---|---|
| F07-01 | Обычный ответ | разобранная статья, fake-llm со скриптом | `POST /chat` | 200 `reply`, `message_id`, `user_message_id`, `context_usage.has_full_paper: true`; провайдер получил системный промпт про `[p.N «…»]`, карточку, полный текст и вопрос | ✅ |
| F07-02 | Стрим | то же | `POST /chat?stream=1` | `delta…` + `done` c `reply`, ids; склейка дельт = `reply` | ✅ |
| F07-03 | История | два вопроса подряд | `GET /chat/messages`; второй запрос к провайдеру | 4 сообщения по порядку user/assistant; провайдер видит первый вопрос и ответ в истории | ✅ |
| F07-04 | Ошибка провайдера не сохраняется | fake-llm → 502 | обычный запрос; стрим | 502 с `detail`; событие `error` без `done`; история не выросла | ✅ |
| F07-05 | Кончился баланс | fake-llm → 402 | `POST /chat` | 502, `detail` про баланс и имя модели | ✅ |
| F07-06 | Приватность | два пользователя, одна статья | `GET /chat/messages` у второго | пусто | ✅ |
| F07-07 | Модели | `LLM_MODELS` = test-model, test-model-alt | `GET /assistant/models`; чат с `model: test-model-alt`; неизвестная | `default: test-model`, 2 элемента; провайдер получил `model: test-model-alt`; 400 | ✅ |
| F07-08 | Выделение как контекст | — | `POST /chat` c `context_text` | в промпте блок «Highlighted passages» с текстом; `context_text` сохранён в сообщении пользователя | ✅ |
| F07-09 | Explain | — | `POST /explain` c `text`/`question`; без `text` | 200 `reply`; провайдер получил «Fragment:» и вопрос; 400 | ✅ |
| F07-10 | Валидация | — | пустой `message`; неизвестное поле; не-JSON | 400 | ✅ |
| F07-11 | Статья без текста | DOI без файла | `POST /chat`; `/chat/context` | провайдер получает «Full paper text is not available yet»; `has_full_paper: false` | ✅ |
| F07-12 | Метр контекста | разобранная статья | `GET /chat/context`; c `model=`; неизвестная модель | `used_tokens > 0`, `limit_tokens: 120000`, `paper_tokens > 0`, `model`; 400 | ✅ |
| F07-13 | Чужая загрузка | — | `/chat`, `/chat/messages`, `/explain` | 404 | ✅ |

**Не проверяется:** 503 «LLM не настроен» (на стенде ключ всегда задан); скользящее резюме истории при переполнении
контекста (нужно >100k токенов истории); обрыв стрима самим провайдером (fake-llm всегда закрывает поток корректно).
