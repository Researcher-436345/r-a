from __future__ import annotations

import json
import os
from collections.abc import AsyncIterator
from datetime import date
from typing import Any, Literal
from urllib.parse import urlparse

import httpx
from fastapi import Depends, FastAPI, Header, HTTPException, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

ResearchMode = Literal["web", "deep"]

app = FastAPI(title="researcher-websearch", version="0.1.0")

LLM_BASE_URL = os.getenv(
    "WEBSEARCH_LLM_BASE_URL", "https://api.proxyapi.ru/openrouter/v1"
).rstrip("/")
LLM_API_KEY = os.getenv("WEBSEARCH_LLM_API_KEY", "") or os.getenv("LLM_API_KEY", "")
LLM_MODEL = os.getenv("WEBSEARCH_LLM_MODEL", "deepseek/deepseek-v4-flash").strip()
DEEP_LLM_MODEL = os.getenv(
    "WEBSEARCH_DEEP_LLM_MODEL", "perplexity/sonar-deep-research"
).strip()
LLM_TIMEOUT_SECONDS = float(os.getenv("WEBSEARCH_TIMEOUT_SECONDS", "180"))
DEEP_LLM_TIMEOUT_SECONDS = float(os.getenv("WEBSEARCH_DEEP_TIMEOUT_SECONDS", "600"))
LLM_HTTP_REFERER = os.getenv("LLM_HTTP_REFERER", "http://localhost:5173")
LLM_APP_TITLE = os.getenv("LLM_APP_TITLE", "Researcher")
INTERNAL_TOKEN = os.getenv("INTERNAL_TOKEN", "")

RESEARCH_SYSTEM_PROMPT = """
Ты — экспертный исследовательский ассистент с доступом к актуальному веб-поиску. 
Помогай пользователю находить и изучать научные статьи, направления исследований, методы и конкурирующие подходы.

Требования к исследованию:
- Сначала определи реальный исследовательский вопрос пользователя, важные термины и связанные подтемы.
- Весь ворфлоу и поиск делай на английском языке, а отвечай на том языке, на котором говорит пользователь.
- Выполни достаточно широкий поиск, чтобы выявить основные подходы, а затем проверь важные утверждения по первичным источникам.
- Отдавай приоритет рецензируемым статьям, оригинальным препринтам, страницам издательств, официальным страницам проектов, датасетам, бенчмаркам и документации. 
- Используй вторичные источники только для полезного дополнительного контекста.
- Текущая дата: {current_date}. Отдавай приоритет свежим работам, вышедшим за последние 1-3 месяца относительно этой даты, и только потом ищи более ранние работы, если пользователь ничего не говорит о датах.
- Указывай даты публикации и явно отличай рецензируемые работы от препринтов.
- Перепроверяй важные и потенциально спорные утверждения. 
- Описывай разногласия, ограничения, отрицательные результаты и неопределённость; не выдавай слабые доказательства за установленный факт.
- Никогда не выдумывай статьи, авторов, даты, метрики, цитаты, DOI, arXiv ID или URL. Если доказательств недостаточно, скажи об этом прямо.
- Приводи конкретные научные статьи и работы, непосредственно относящиеся к теме пользователя, а не только общее описание области.
- Для каждой рекомендуемой работы дай краткий обзор: название с прямой ссылкой на оригинал, авторов и год публикации при наличии, центральную идею, 
  использованный метод или подход, основные заявленные результаты и объяснение релевантности вопросу пользователя. 
- Отмечай существенные ограничения и статус работы — рецензируемая публикация или препринт.
- Отвечай на языке последнего сообщения пользователя.
- Не раскрывай скрытые рассуждения или внутреннюю цепочку мыслей. При необходимости показывай только краткие выводы о процессе поиска.

Требования к ответу:
- Верни только законченный ответ в Markdown.
- Давай именно статьи по порядку, а не просто другие различные факты.
- Используй ясную иерархию заголовков, короткие абзацы, **жирное выделение**, цитаты и Markdown-таблицы, когда они улучшают восприятие.
- Не используй маркированные или нумерованные списки, больше используй абзацы, не делай большие заголовки.
- Оформляй источники как кликабельные Markdown-ссылки с понятными названиями: [название статьи или источника](https://...).
- Размещай ссылки на источники рядом с фактическими, актуальными и количественными утверждениями. Не выводи непроверяемые голые URL.
- Заверши ответ таблицей с найденными работами и ссылками на уникальные источники на языке ответа, если API-цитаты уже не образуют эквивалентный список ссылок.
- Дай содержательный синтез, а не простой перечень результатов.
- Объясни различия между подходами, условия их применимости и границы имеющихся доказательств, это можно добавлять в таблицу.
"""


class SearchMessage(BaseModel):
    role: Literal["user", "assistant"]
    content: str = Field(min_length=1, max_length=100_000)


class SearchRequest(BaseModel):
    messages: list[SearchMessage] = Field(min_length=1, max_length=40)
    mode: ResearchMode = "web"


class Source(BaseModel):
    title: str
    url: str
    domain: str
    published_at: str | None = None


def require_internal_token(x_internal_token: str | None = Header(default=None)) -> None:
    if INTERNAL_TOKEN and x_internal_token != INTERNAL_TOKEN:
        raise HTTPException(status_code=401, detail="invalid internal token")


def system_prompt(mode: ResearchMode, current_date: date | None = None) -> str:
    prompt_date = current_date or date.today()
    if mode == "deep":
        suffix = """
Режим глубокого исследования:
- Проведи полный многоэтапный поиск и сопоставь несколько независимых первичных источников.
- Подготовь очень подробный исследовательский отчёт. Не сокращай материал ради краткости: полнота и глубина важнее объёма ответа.
- Последовательно раскрой контекст и терминологию, основные подходы и методологии, доказательства и результаты, сравнение работ, противоречия, ограничения и открытые вопросы.
- Подкрепляй источниками каждое существенное фактическое утверждение и явно отделяй подтверждённые выводы от интерпретаций.
- Заверши содержательными выводами и конкретными рекомендациями для дальнейшего чтения.
""".strip()
    else:
        suffix = """
Режим веб-поиска:
- Ответь эффективно и по существу, проверив наиболее важные утверждения и добавив прямые ссылки на источники.
- Дай небольшой, но подробный и ёмкий ответ.
""".strip()
    prompt = RESEARCH_SYSTEM_PROMPT.format(current_date=prompt_date.isoformat())
    return f"{prompt}\n\n{suffix}"


def model_for_mode(mode: ResearchMode) -> str:
    return DEEP_LLM_MODEL if mode == "deep" else LLM_MODEL


def timeout_for_mode(mode: ResearchMode) -> float:
    return DEEP_LLM_TIMEOUT_SECONDS if mode == "deep" else LLM_TIMEOUT_SECONDS


def normalize_source(raw: Any) -> Source | None:
    if isinstance(raw, dict) and raw.get("type") == "url_citation":
        raw = raw.get("url_citation")
    if isinstance(raw, str):
        url = raw.strip()
        title = ""
        published_at = None
    elif isinstance(raw, dict):
        url = str(raw.get("url") or raw.get("link") or "").strip()
        title = str(raw.get("title") or raw.get("name") or "").strip()
        published_at = raw.get("published_at") or raw.get("published_date") or raw.get("date")
        published_at = str(published_at) if published_at else None
    else:
        return None
    parsed = urlparse(url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        return None
    domain = parsed.hostname or parsed.netloc
    return Source(title=title or domain, url=url, domain=domain, published_at=published_at)


def merge_sources(current: list[Source], values: list[Any]) -> None:
    seen = {source.url for source in current}
    for value in values:
        source = normalize_source(value)
        if source is not None and source.url not in seen:
            current.append(source)
            seen.add(source.url)


def has_linked_sources_section(answer: str) -> bool:
    lowered = answer.lower()
    has_heading = any(
        heading in lowered
        for heading in ("\n## sources", "\n## источники", "\n## источники и литература")
    )
    return has_heading and "](" in answer


def markdown_sources(sources: list[Source]) -> str:
    if not sources:
        return ""
    lines = ["", "", "## Sources", ""]
    for index, source in enumerate(sources, start=1):
        title = source.title.replace("[", "").replace("]", "")
        lines.append(f"{index}. [{title}]({source.url})")
    return "\n".join(lines)


def sse(event: str, payload: Any) -> str:
    return f"event: {event}\ndata: {json.dumps(payload, ensure_ascii=False)}\n\n"


def provider_body(payload: SearchRequest) -> dict[str, Any]:
    body: dict[str, Any] = {
        "model": model_for_mode(payload.mode),
        "messages": [
            {"role": "system", "content": system_prompt(payload.mode)},
            *(message.model_dump() for message in payload.messages),
        ],
        "stream": True,
    }
    if payload.mode == "web":
        body["tools"] = [
            {
                "type": "openrouter:web_search",
                "parameters": {
                    "engine": "auto",
                    "max_results": 5,
                    "max_total_results": 20,
                },
            },
            {
                "type": "openrouter:web_fetch",
                "parameters": {
                    "max_uses": 10,
                    "max_content_tokens": 50_000,
                },
            },
        ]
    else:
        body["reasoning"] = {"effort": "high", "summary": "concise"}
    return body


def reasoning_summary_fragments(chunk: dict[str, Any]) -> list[str]:
    fragments: list[str] = []
    for choice in chunk.get("choices") or []:
        delta = choice.get("delta") or {}
        for detail in delta.get("reasoning_details") or []:
            if not isinstance(detail, dict) or detail.get("type") != "reasoning.summary":
                continue
            summary = detail.get("summary")
            if isinstance(summary, str) and summary:
                fragments.append(summary)
    return fragments


async def provider_stream(payload: SearchRequest) -> AsyncIterator[tuple[str, Any]]:
    if not LLM_API_KEY:
        raise RuntimeError("веб-поиск не настроен: добавь WEBSEARCH_LLM_API_KEY или LLM_API_KEY")
    model = model_for_mode(payload.mode)
    if not model:
        variable = "WEBSEARCH_DEEP_LLM_MODEL" if payload.mode == "deep" else "WEBSEARCH_LLM_MODEL"
        raise RuntimeError(f"веб-поиск не настроен: добавь {variable}")

    headers = {"Authorization": f"Bearer {LLM_API_KEY}", "Content-Type": "application/json"}
    if "openrouter.ai" in LLM_BASE_URL:
        headers["HTTP-Referer"] = LLM_HTTP_REFERER
        headers["X-Title"] = LLM_APP_TITLE
    body = provider_body(payload)
    timeout = httpx.Timeout(timeout_for_mode(payload.mode), connect=20.0)
    progress_parts: list[str] = []
    async with httpx.AsyncClient(timeout=timeout) as client:
        async with client.stream(
            "POST", f"{LLM_BASE_URL}/chat/completions", headers=headers, json=body
        ) as response:
            if response.status_code // 100 != 2:
                detail = (await response.aread())[:8192].decode("utf-8", errors="replace").strip()
                raise RuntimeError(f"web search provider returned {response.status_code}: {detail or response.reason_phrase}")
            async for line in response.aiter_lines():
                line = line.strip()
                if not line or line.startswith(":"):
                    continue
                if line.startswith("data:"):
                    line = line.removeprefix("data:").strip()
                if line == "[DONE]":
                    break
                try:
                    chunk = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if payload.mode == "deep":
                    for summary in reasoning_summary_fragments(chunk):
                        progress_parts.append(summary)
                        yield "progress", "".join(progress_parts)[-4_000:]
                choices = chunk.get("choices") or []
                delta = ""
                if choices:
                    delta = str((choices[0].get("delta") or {}).get("content") or "")
                if delta:
                    yield "delta", delta
                raw_sources: list[Any] = []
                raw_sources.extend(chunk.get("citations") or [])
                raw_sources.extend(chunk.get("search_results") or [])
                for choice in choices:
                    raw_sources.extend((choice.get("delta") or {}).get("annotations") or [])
                    raw_sources.extend((choice.get("message") or {}).get("annotations") or [])
                if raw_sources:
                    yield "sources", raw_sources


@app.get("/health")
def health() -> dict[str, Any]:
    return {
        "status": "ok",
        "service": "websearch",
        "model": LLM_MODEL,
        "models": {"web": LLM_MODEL, "deep": DEEP_LLM_MODEL},
        "version": "0.1.0",
    }


@app.post("/v1/search/stream", dependencies=[Depends(require_internal_token)])
async def search_stream(body: SearchRequest, request: Request) -> StreamingResponse:
    async def events() -> AsyncIterator[str]:
        answer_parts: list[str] = []
        sources: list[Source] = []
        try:
            async for event, value in provider_stream(body):
                if await request.is_disconnected():
                    return
                if event == "delta":
                    answer_parts.append(value)
                    yield sse("delta", {"content": value})
                elif event == "progress":
                    yield sse("progress", {"content": value})
                elif event == "sources":
                    previous_count = len(sources)
                    merge_sources(sources, value)
                    if body.mode == "deep" and len(sources) > previous_count:
                        yield sse(
                            "source_progress",
                            {
                                "count": len(sources),
                                "sources": [source.model_dump() for source in sources[-3:]],
                            },
                        )
            answer = "".join(answer_parts)
            if not has_linked_sources_section(answer):
                suffix = markdown_sources(sources)
                if suffix:
                    answer_parts.append(suffix)
                    yield sse("delta", {"content": suffix})
            if not "".join(answer_parts).strip():
                raise RuntimeError("web search provider returned an empty response")
            yield sse("sources", {"sources": [source.model_dump() for source in sources]})
            yield sse("done", {"status": "ok"})
        except Exception as exc:  # noqa: BLE001
            yield sse("error", {"detail": str(exc)})

    return StreamingResponse(
        events(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache, no-transform", "X-Accel-Buffering": "no"},
    )
