# Assistant Service (Article QA)

A standalone Go service that answers user questions about a research article using
an LLM. It is a stateless orchestrator: it does not own data, but combines two
upstream services with an LLM.

```
             ┌────────────────┐
   question  │                │  GET article text
 ─────────►  │   assistant    ├───────────────────►  Article Service
   SSE       │    service     │
 ◄─────────  │                │  GET/POST messages
   answer    │                ├───────────────────►  Chat Service
             └───────┬────────┘
                     │ streaming chat completion
                     ▼
                  OpenAI (or OpenAI-compatible gateway)
```

On each question the service:

1. Fetches the article (by `articleId`) from the **Article Service** and the chat history (by `chatId`) from the **Chat Service**, concurrently.
2. Builds a grounded prompt (system rules + full article + history + question).
3. Streams the LLM answer back to the caller via **Server-Sent Events (SSE)**.
4. Persists the user question and the assistant answer to the Chat Service.

The whole article is placed into the model context (no RAG in this version).

## Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check → `{"status":"ok"}` |
| `GET` | `/v1/chats/{chatId}/messages` | Return stored chat history (JSON) |
| `POST` | `/v1/chats/{chatId}/messages` | Ask a question; streams the answer as SSE |

**POST body:**

```json
{ "articleId": "qgf-flow-policies", "content": "Explain the core idea" }
```

**SSE events:**

- `event: delta` — `{"text":"..."}` incremental answer chunks
- `event: done` — `{"messageId":"...","content":"full answer"}` on completion
- `event: error` — `{"error":"..."}` if a failure occurs after streaming started

Errors before streaming starts (validation, upstream unavailable) return a normal
HTTP status (`400` / `502`) with a JSON body `{"error":"..."}`.

## Configuration (environment)

| Variable | Description | Default |
|---|---|---|
| `PORT` | HTTP port | `8080` |
| `OPENAI_API_KEY` | API key (**required**) | — |
| `OPENAI_BASE_URL` | Base URL for OpenAI-compatible gateways | SDK default |
| `OPENAI_MODEL` | Model id | `gpt-4o` |
| `LLM_MAX_TOKENS` | Max completion tokens | `1024` |
| `LLM_TEMPERATURE` | Sampling temperature | `0.2` |
| `ARTICLE_SERVICE_URL` | Article Service base URL (**required**) | — |
| `CHAT_SERVICE_URL` | Chat Service base URL (**required**) | — |
| `REQUEST_TIMEOUT` | Overall per-request budget | `60s` |
| `UPSTREAM_TIMEOUT` | Timeout for Article/Chat calls | `10s` |

## Assumed upstream contracts

These services do not exist yet; the clients target the following REST/JSON shapes
(swap the client implementations in `internal/article` / `internal/chat` when the
real specs land — the `qa` orchestration depends only on the interfaces).

**Article Service**

```
GET {ARTICLE_SERVICE_URL}/articles/{articleId}
200 -> {"id","title","authors","content"}   # content = full plain text
```

**Chat Service**

```
GET  {CHAT_SERVICE_URL}/chats/{chatId}/messages
200 -> [{"id","role","content","createdAt"}]  # role: user|assistant

POST {CHAT_SERVICE_URL}/chats/{chatId}/messages
body {"role","content"} -> {"id","role","content","createdAt"}
```

## Layout

```
cmd/assistant/         entry point, config wiring, graceful shutdown
internal/config/       env loading + validation
internal/domain/       shared types (Article, Message)
internal/article/      Article Service HTTP client
internal/chat/         Chat Service HTTP client
internal/llm/          OpenAI streaming wrapper + prompt builder
internal/qa/           orchestration (ArticleFetcher, ChatStore, LLMStreamer)
internal/httpapi/      router, handlers, SSE, middleware
```

## Configuration file (.env)

On startup the service loads a `.env` file from the working directory (if
present). Real environment variables always take precedence, so a shell
`export` or `docker run -e` overrides `.env`.

```bash
cp .env.example .env   # then fill in OPENAI_API_KEY
```

`.env` is gitignored — never commit real secrets. `.env.example` is the
template.

## Run

With a `.env` in place, no flags are needed:

```bash
go run ./cmd/assistant
```

Or pass everything via the environment instead:

```bash
export OPENAI_API_KEY=sk-...
export ARTICLE_SERVICE_URL=http://localhost:9100
export CHAT_SERVICE_URL=http://localhost:9100
go run ./cmd/assistant
```

Ask a question and watch the stream:

```bash
curl -N -X POST localhost:8080/v1/chats/c1/messages \
  -H 'Content-Type: application/json' \
  -d '{"articleId":"qgf-flow-policies","content":"Explain the core idea in simple terms"}'
```

## Test & build

```bash
go test ./...
go vet ./...
go build ./...
```

## Docker

### Docker Compose (recommended)

`docker-compose.yml` builds the image and loads `.env` automatically via
`env_file`, so no flags are needed:

```bash
docker compose up --build
```

The compose file also maps `host.docker.internal` to the host, so upstream
services running on your machine are reachable from the container (see the note
below).

### Plain docker

```bash
docker build -t assistant .

# load your .env (the file is excluded from the image, so pass it at run time)
docker run --rm -p 8080:8080 --env-file .env assistant

# or pass variables explicitly
docker run --rm -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  -e ARTICLE_SERVICE_URL=http://article:9001 \
  -e CHAT_SERVICE_URL=http://chat:9002 \
  assistant
```

Note: inside the container `localhost` is the container itself. To reach an
upstream service running on the **host** machine (e.g. the local mock), set the
URLs to `http://host.docker.internal:<port>` in `.env`. When all services run in
the same Docker network, address them by service name (e.g. `http://article:9001`).
