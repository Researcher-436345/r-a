# websearch

Внутренний FastAPI-сервис для web-search и deep-research. Он отвечает за вызовы
провайдера, системные промпты, нормализацию потоковых ответов и структурированные
источники.

## API

- `GET /health` — проверка состояния сервиса.
- `POST /v1/search/stream` — внутренний SSE-endpoint веб-поиска.

## Настройка

Сервис настраивается переменными окружения:

- `WEBSEARCH_LLM_BASE_URL` — адрес OpenRouter через ProxyAPI; по умолчанию
  `https://api.proxyapi.ru/openrouter/v1`.
- `WEBSEARCH_LLM_API_KEY` — ключ API провайдера.
- `WEBSEARCH_LLM_MODEL` — модель веб-поиска; по умолчанию
  `perplexity/sonar-pro` (встроенный поиск, источники приходят в `annotations`).
  Инструменты `openrouter:web_search`/`web_fetch` добавляются только при прямом
  `https://openrouter.ai/api/v1`: ProxyAPI их отвергает с 400, `plugins`/`:online`
  игнорирует, поэтому за прокси нужна модель с собственным поиском.
- `WEBSEARCH_DOMAINS` — до 10 доменов через запятую для `search_domain_filter`
  моделей Perplexity; по умолчанию только источники, которые каталог умеет открыть
  в ридере (arxiv.org, doi.org, semanticscholar.org, aclanthology.org,
  proceedings.mlr.press, openaccess.thecvf.com, proceedings.neurips.cc,
  papers.nips.cc, ojs.aaai.org).
  Без фильтра на «с чего начать» приходят блоги и уроки, которые нельзя открыть
  в ридере.
- `WEBSEARCH_DEEP_LLM_MODEL` — модель глубокого исследования; по умолчанию
  `perplexity/sonar-deep-research`.
- `WEBSEARCH_TIMEOUT_SECONDS` — таймаут запроса к провайдеру.
- `WEBSEARCH_DEEP_TIMEOUT_SECONDS` — отдельный таймаут deep-research; по
  умолчанию 600 секунд.
- `INTERNAL_TOKEN` — токен для внутренних запросов из `searchapi`.

Для локального запуска значения задаются в корневом `.env`. Актуальный пример
находится в `.env.example`.

В режиме web запрос передаёт модели серверные инструменты `openrouter:web_search`
и `openrouter:web_fetch`. В режиме deep эти инструменты не передаются:
`sonar-deep-research` выполняет многоэтапный поиск самостоятельно и не
поддерживает OpenRouter tool use.
Краткие `reasoning.summary` из deep-потока передаются в интерфейс как временный
прогресс и не сохраняются в итоговом ответе или истории диалога.

## Ссылки в ответах

Каждая найденная статья открывается в ридере приложения, поэтому ссылки
ограничены страницами конкретных статей.

- `is_non_paper_url()` решает только по URL, без сетевых запросов, что страница —
  оглавление, список, поиск, профиль, вход, справка или главная страница:
  корень и `list|archive|catchup|year|search|a|login|help` на arxiv.org,
  хосты `info.|blog.|status.|search.arxiv.org`; корень, `venues`, `venue`,
  `group` на OpenReview; корень, `topic`, `search`, `product`, `author`, `venue`
  на Semantic Scholar; корень, `journal`, `browse` на ScienceDirect и тома
  `/vol/N` любого издателя; годовые страницы NeurIPS; всё на
  `openaccess.thecvf.com` вне `/content*` (меню, программы, дни конференции);
  корень, `events`, `venues`, `volumes` в ACL Anthology; тома PMLR; корень
  любого сайта. Те же классы проверяют `not_a_paper` в
  `backend/internal/modules/catalog/indexpages.go` и
  `frontend/src/features/papers/link-kind.ts` — меняйте списки вместе.
- Такие адреса не попадают ни в события `sources`/`source_progress`, ни в
  дописываемый раздел `## Sources`. Раздел дописывается, только если ответ что-то
  цитирует (`[n]` или URL) и в собственном разделе источников модели нет ни одной
  ссылки на статью, поэтому приветствия остаются без списка источников.
- Текст ответа уходит дельтами, а `searchapi` сохраняет их склейку, поэтому
  ссылки внутри текста здесь не переписываются: фронтенд показывает ссылки на
  не-статьи как явные внешние.
- Системный промпт требует ссылку на страницу конкретной статьи (arXiv abs или
  DOI), точное оригинальное название в тексте ссылки, arXiv ID только из
  результата поиска для этой статьи (иначе DOI или страница Semantic Scholar) и
  короткий ответ без ссылок на приветствия и разговоры не об исследованиях.
