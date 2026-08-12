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
  `deepseek/deepseek-v4-flash`.
- `WEBSEARCH_DEEP_LLM_MODEL` — модель глубокого исследования; по умолчанию
  `perplexity/sonar-deep-research`.
- `WEBSEARCH_TIMEOUT_SECONDS` — таймаут запроса к провайдеру.
- `INTERNAL_TOKEN` — токен для внутренних запросов из `searchapi`.

Для локального запуска значения задаются в корневом `.env`. Актуальный пример
находится в `.env.example`.
