from __future__ import annotations

import json
import logging
import os
import re
import time
from collections.abc import AsyncIterator
from datetime import date, datetime, timezone
from typing import Any, Literal
from urllib.parse import urlparse

import httpx
from fastapi import Depends, FastAPI, Header, HTTPException, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

ResearchMode = Literal["web", "deep"]

app = FastAPI(title="researcher-websearch", version="0.1.0")
logger = logging.getLogger("uvicorn.error")

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
EVENT_DISCOVERY_TIMEOUT_SECONDS = float(os.getenv("EVENT_DISCOVERY_TIMEOUT_SECONDS", "360"))
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


class DiscoveredEvent(BaseModel):
    id: str = Field(min_length=2, max_length=100)
    title: str = Field(min_length=2, max_length=160)
    summary: str = Field(min_length=40, max_length=360)
    start_date: str
    end_date: str
    city: str = Field(max_length=120)
    country: str = Field(max_length=120)
    format: Literal["in_person", "online", "hybrid"]
    kind: Literal["conference", "meetup"]
    region: Literal["ru", "global"]
    topics: list[str] = Field(default_factory=list, max_length=8)
    url: str
    registration_url: str | None = None
    source_url: str
    featured: bool = False


class KnownEvent(BaseModel):
    id: str = Field(min_length=2, max_length=100)
    title: str = Field(min_length=2, max_length=160)
    start_date: str
    url: str


class EventDiscoveryRequest(BaseModel):
    known_events: list[KnownEvent] = Field(default_factory=list, max_length=100)


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


def extract_json_object(value: str) -> dict[str, Any]:
    text = value.strip()
    if text.startswith("```"):
        first_newline = text.find("\n")
        last_fence = text.rfind("```")
        if first_newline >= 0 and last_fence > first_newline:
            text = text[first_newline + 1 : last_fence].strip()
    start = text.find("{")
    end = text.rfind("}")
    if start < 0 or end <= start:
        raise ValueError("event discovery returned no JSON object")
    parsed = json.loads(text[start : end + 1])
    if not isinstance(parsed, dict):
        raise ValueError("event discovery returned invalid JSON")
    return parsed


EVENT_MONTH_MARKERS = {
    1: ("january", "jan", "январ"),
    2: ("february", "feb", "феврал"),
    3: ("march", "mar", "март"),
    4: ("april", "apr", "апрел"),
    5: ("may", "май", "мая"),
    6: ("june", "jun", "июн"),
    7: ("july", "jul", "июл"),
    8: ("august", "aug", "август"),
    9: ("september", "sep", "sept", "сентябр"),
    10: ("october", "oct", "октябр"),
    11: ("november", "nov", "ноябр"),
    12: ("december", "dec", "декабр"),
}


def canonical_event_source_url(value: str) -> str:
    parsed = urlparse(value.strip())
    host = (parsed.hostname or "").lower().removeprefix("www.")
    path = parsed.path.rstrip("/") or "/"
    return f"{host}{path}"


def cited_date_texts(annotations: list[Any]) -> dict[str, list[str]]:
    result: dict[str, list[str]] = {}
    for annotation in annotations:
        if not isinstance(annotation, dict) or annotation.get("type") != "url_citation":
            continue
        citation = annotation.get("url_citation")
        if not isinstance(citation, dict):
            continue
        url = str(citation.get("url") or "")
        content = str(citation.get("content") or "").lower()
        key = canonical_event_source_url(url)
        if key and content:
            result.setdefault(key, []).append(content)
    return result


def citation_mentions_date(content: str, value: date) -> bool:
    year = str(value.year)
    day = str(value.day)
    numeric = re.search(
        rf"(?:\b{year}[-./]0?{value.month}[-./]0?{value.day}\b|"
        rf"\b0?{value.day}[-./]0?{value.month}[-./]{year}\b)",
        content,
    )
    if numeric:
        return True
    has_year = re.search(rf"\b{year}\b", content) is not None
    has_day = re.search(rf"\b{day}(?:st|nd|rd|th)?\b", content) is not None
    has_month = any(marker in content for marker in EVENT_MONTH_MARKERS[value.month])
    return has_year and has_day and has_month


def citation_confirms_event_dates(event: DiscoveredEvent, annotations: list[Any]) -> bool:
    try:
        start = date.fromisoformat(event.start_date)
        end = date.fromisoformat(event.end_date)
    except ValueError:
        return False
    texts = cited_date_texts(annotations).get(canonical_event_source_url(event.source_url), [])
    return any(
        citation_mentions_date(content, start) and citation_mentions_date(content, end)
        for content in texts
    )


def validate_discovered_events(
    raw_items: list[Any],
    annotations: list[Any] | None = None,
    *,
    require_date_citation: bool = False,
) -> tuple[list[dict[str, Any]], int]:
    items: list[dict[str, Any]] = []
    rejected = 0
    for raw_item in raw_items[:50]:
        try:
            event = DiscoveredEvent.model_validate(raw_item)
            if require_date_citation and not citation_confirms_event_dates(
                event, annotations or []
            ):
                rejected += 1
                continue
            items.append(event.model_dump())
        except (TypeError, ValueError):
            rejected += 1
    return items, rejected


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


def event_discovery_body(
    today: date | None = None,
    known_events: list[KnownEvent] | None = None,
) -> dict[str, Any]:
    current_date = (today or date.today()).isoformat()
    known = [item.model_dump() for item in (known_events or [])]
    known_json = json.dumps(known, ensure_ascii=False, separators=(",", ":"))
    prompt = f"""
Today is {current_date}. Build a broad, practical calendar of upcoming technology
events. Search official organizer websites and official event pages. The known
catalog is included below. Prioritize events missing from it; revisit a known
event only when an official source confirms materially newer dates or links.

KNOWN_CATALOG_JSON:
{known_json}

PRIORITY 1 — RUSSIA AND RUSSIAN-SPEAKING TECH COMMUNITY (broad coverage):
- Find useful conferences and public meetups in AI, ML, data, backend, frontend,
  mobile, DevOps, cloud, infrastructure, security, product and engineering.
- Cover Moscow, Saint Petersburg and other Russian cities, plus online events
  intended for Russian-speaking engineers.
- Check official calendars and event pages of Yandex, VK Tech, Sber, T-Bank,
  Ozon Tech, AvitoTech, MTS, Kaspersky, Positive Technologies, Selectel, ITMO,
  HSE, Skoltech and major independent communities and conference organizers.
- Specifically consider BigTechNight, Turbo ML Conf, Yandex events and recaps,
  Practical ML Conf, HighLoad++, AI Journey, TeamLead Conf, Data Fest and major
  local AI/ML, security and engineering meetups.

PRIORITY 2 — GLOBAL EVENTS (flagships only):
- Include only internationally significant research or industry conferences,
  such as NeurIPS, ICML, ICLR, CVPR, ACL, EMNLP, AAAI, KDD, SIGIR, The Web
  Conference, NVIDIA GTC, KubeCon, AWS re:Invent, Google Cloud Next, Microsoft
  Build/Ignite, Black Hat and DEF CON when exact future dates are confirmed.

SEARCH RULES:
- Use varied queries and do not repeat a failed or rate-limited query.
- Return only events whose end_date is today or later.
- An exact date must be confirmed by an official organizer page. Never infer a
  recurring event's date from a previous year or an unofficial aggregator.
- source_url must be the exact official page whose web-search citation visibly
  contains the claimed event dates. A generic organizer homepage, an event page
  without a published date, or a model assumption is not evidence; omit the
  event in all three cases.
- Return at most 50 unique event editions, sorted by start_date.
- One conference edition is one item. Never return satellites, venues, tracks,
  workshops, tutorials or individual conference days as separate events. Merge
  multiple locations into the main conference record.
- If the search limit is reached, immediately return all already verified items
  as valid JSON; do not explain the limitation and do not keep retrying.

UNIFIED OUTPUT STYLE:
- title: official event name, without marketing suffixes.
- summary: exactly one neutral, informative Russian sentence, about 100–240
  characters, ending with punctuation; do not repeat dates, city or country.
- city and country: Russian-language names. For a multi-location event, join
  locations consistently with ` · `.
- region: `ru` for events in Russia or primarily for the Russian-speaking tech
  community; `global` for all international events.
- format, kind and region must use only the enum values from the schema.
- url and source_url must point to official pages; registration_url is the
  official registration page or null.

Return only one JSON object with this exact shape:
{{"items":[{{"id":"lowercase-slug-year","title":"...","summary":"Russian summary",
"start_date":"YYYY-MM-DD","end_date":"YYYY-MM-DD","city":"...","country":"...",
"format":"in_person|online|hybrid","kind":"conference|meetup","region":"ru|global",
"topics":["..."],"url":"https://...","registration_url":null,
"source_url":"https://...","featured":false}}]}}
""".strip()
    body = {
        "model": LLM_MODEL,
        "messages": [
            {
                "role": "system",
                "content": "You verify event dates using web search and output strict JSON only.",
            },
            {"role": "user", "content": prompt},
        ],
        "response_format": {"type": "json_object"},
        "temperature": 0.1,
        "max_completion_tokens": 12_000,
        "usage": {"include": True},
        "stop_server_tools_when": [
            {"type": "step_count_is", "step_count": 24},
            {"type": "max_cost", "max_cost_in_dollars": 0.08},
        ],
        "stream": False,
        "tools": [
            {
                "type": "openrouter:web_search",
                "parameters": {
                    "engine": "exa",
                    "max_results": 5,
                    "max_total_results": 50,
                    "search_context_size": "low",
                },
            },
        ],
    }
    return body


@app.post("/v1/events/discover", dependencies=[Depends(require_internal_token)])
async def discover_events(request: EventDiscoveryRequest) -> dict[str, Any]:
    if not LLM_API_KEY:
        raise HTTPException(
            status_code=503,
            detail="event discovery is not configured: add WEBSEARCH_LLM_API_KEY or LLM_API_KEY",
        )
    body = event_discovery_body(known_events=request.known_events)
    headers = {"Authorization": f"Bearer {LLM_API_KEY}", "Content-Type": "application/json"}
    if "openrouter.ai" in LLM_BASE_URL:
        headers["HTTP-Referer"] = LLM_HTTP_REFERER
        headers["X-Title"] = LLM_APP_TITLE
    timeout = httpx.Timeout(EVENT_DISCOVERY_TIMEOUT_SECONDS, connect=20.0)
    started = time.monotonic()
    logger.info("event discovery started: model=%s", LLM_MODEL)
    try:
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.post(
                f"{LLM_BASE_URL}/chat/completions", headers=headers, json=body
            )
    except httpx.TimeoutException as exc:
        elapsed = time.monotonic() - started
        logger.warning("event discovery timed out after %.1fs", elapsed)
        raise HTTPException(status_code=504, detail="event discovery provider timed out") from exc
    except httpx.RequestError as exc:
        logger.warning("event discovery request failed: %s", type(exc).__name__)
        raise HTTPException(status_code=502, detail="event discovery provider is unavailable") from exc
    if response.status_code // 100 != 2:
        logger.warning("event discovery provider returned status=%d", response.status_code)
        raise HTTPException(status_code=502, detail="event discovery provider failed")
    try:
        provider_payload = response.json()
    except (TypeError, ValueError, json.JSONDecodeError) as exc:
        logger.warning("event discovery provider returned non-JSON data")
        raise HTTPException(status_code=502, detail="event discovery returned invalid data") from exc
    usage = provider_payload.get("usage") or {}
    tool_usage = usage.get("server_tool_use_details") or usage.get("server_tool_use") or {}
    logger.info(
        "event discovery provider completed: elapsed=%.1fs total_tokens=%s cost=%s searches=%s",
        time.monotonic() - started,
        usage.get("total_tokens", "unknown"),
        usage.get("cost", "unknown"),
        tool_usage.get("web_search_requests", "unknown"),
    )
    try:
        message = provider_payload["choices"][0]["message"]
        content = message["content"]
        raw_items = extract_json_object(str(content)).get("items", [])
        if not isinstance(raw_items, list):
            raise ValueError("items must be a list")
        items, rejected = validate_discovered_events(
            raw_items,
            message.get("annotations") or [],
            require_date_citation=True,
        )
        if raw_items and not items:
            raise ValueError("all discovered events failed validation")
    except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
        logger.warning("event discovery returned an invalid response")
        raise HTTPException(status_code=502, detail="event discovery returned invalid data") from exc
    logger.info(
        "event discovery parsed: accepted=%d rejected=%d",
        len(items),
        rejected,
    )
    return {
        "items": items,
        "generated_at": datetime.now(timezone.utc).isoformat(),
    }
