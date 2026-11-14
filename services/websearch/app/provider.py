from __future__ import annotations

import asyncio
import json
import re
from collections.abc import AsyncIterator
from typing import Any
from urllib.parse import quote, urlparse

import httpx
import anyio

from .metrics import LLM_HTTP, LLM_TOOL_CALLS
from .settings import SearchProfile, SearchSettings


def request_body(
    messages: list[dict[str, str]],
    instructions: str,
    profile: SearchProfile,
    domains: tuple[str, ...],
) -> dict[str, Any]:

    tool: dict[str, Any] = {"type": "web_search"}
    if domains:
        tool["filters"] = {"allowed_domains": list(domains)}

    return {
        "model": profile.model,
        "instructions": instructions,
        "input": messages,
        "tools": [tool],
        "tool_choice": "required",
        "include": ["web_search_call.action.sources"],
        "reasoning": {"effort": profile.reasoning_effort, "summary": "auto"},
        "max_tool_calls": profile.max_tool_calls,
        "max_output_tokens": profile.max_output_tokens,
        "background": profile.background,
        "store": profile.background,
        "stream": True,
    }


async def read_events(response: httpx.Response) -> AsyncIterator[dict[str, Any]]:

    parts: list[str] = []

    async for line in response.aiter_lines():
        if line.startswith("data:"):
            parts.append(line.removeprefix("data:").lstrip())
        elif line.startswith(":"):
            yield {"type": "keepalive"}
        elif not line and parts:
            yield json.loads("\n".join(parts))
            parts.clear()

    if parts:
        yield json.loads("\n".join(parts))


def cited_text(text: str, annotations: list[dict[str, Any]]) -> str:
    """Заменить размеченные API цитаты ссылками; индексы относятся к исходному тексту."""

    replacements: list[tuple[int, int, str]] = []

    for annotation in annotations:
        if annotation.get("type") != "url_citation":
            continue

        url = annotation["url"]
        parsed = urlparse(url)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            continue

        start, end = annotation["start_index"], annotation["end_index"]
        if not 0 <= start < end <= len(text):
            continue

        original = text[start:end]
        if re.fullmatch(r"\[[^\]]+\]\(https?://[^\s]+\)", original):
            continue

        title = annotation.get("title") or parsed.hostname or "Источник"
        title = title.replace("[", "").replace("]", "")
        safe_url = url.replace("(", "%28").replace(")", "%29")
        replacements.append((start, end, f"[{title}]({safe_url})"))

    boundary = len(text)

    for start, end, replacement in sorted(replacements, reverse=True):
        if end <= boundary:
            text = text[:start] + replacement + text[end:]
            boundary = start

    return text


def final_result(response: dict[str, Any]) -> tuple[str, list[dict[str, Any]]]:

    text_parts: list[str] = []
    sources: list[dict[str, Any]] = []

    for item in response.get("output", []):
        if item["type"] == "web_search_call":
            sources.extend(item.get("action", {}).get("sources", []))
        elif item["type"] == "message":
            for part in item.get("content", []):
                if part["type"] == "output_text":
                    annotations = part.get("annotations", [])
                    sources.extend(annotation for annotation in annotations if annotation["type"] == "url_citation")
                    text_parts.append(cited_text(part["text"], annotations))

    return "\n\n".join(text_parts), sources


def check_completion(response: dict[str, Any]) -> None:

    status = response["status"]
    if status == "completed":
        return

    if status == "incomplete":
        reason = response.get("incomplete_details", {}).get("reason", "unknown")
        raise RuntimeError(f"OpenAI research incomplete: {reason}")

    message = (response.get("error") or {}).get("message") or status
    raise RuntimeError(f"OpenAI research failed: {message}")


async def response_request(settings: SearchSettings, response_id: str, *, cancel: bool = False) -> dict[str, Any]:

    if not settings.api_key:
        raise RuntimeError("задайте OPENAI_API_KEY")

    url = f"{settings.base_url}/responses/{quote(response_id, safe='')}"
    headers = {"Authorization": f"Bearer {settings.api_key}"}

    async with httpx.AsyncClient(headers=headers, timeout=10) as client:
        response = await client.post(f"{url}/cancel") if cancel else await client.get(url)
        response.raise_for_status()

        return response.json()


async def stream_response(
    settings: SearchSettings,
    profile: SearchProfile,
    body: dict[str, Any],
    *,
    transport: httpx.AsyncBaseTransport | None = None,
) -> AsyncIterator[tuple[str, Any]]:

    if not settings.api_key:
        raise RuntimeError("веб-поиск не настроен: задайте OPENAI_API_KEY")
    if not profile.model:
        variable = "WEBSEARCH_DEEP_LLM_MODEL" if profile.background else "WEBSEARCH_LLM_MODEL"
        raise RuntimeError(f"выберите LLM_MODEL или {variable} в settings.py или env")

    feature = "deep_research" if profile.background else "search"
    headers = {"Authorization": f"Bearer {settings.api_key}", "Accept": "text/event-stream"}
    timeout = httpx.Timeout(profile.timeout, connect=20.0, read=min(profile.timeout, 90))
    response_id: str | None = None
    cursor: int | None = None
    completed = False
    progress = ""
    attempts = 0

    async with httpx.AsyncClient(headers=headers, timeout=timeout, transport=transport) as client:
        try:
            async with asyncio.timeout(profile.timeout):
                while True:
                    method, url, options = "POST", f"{settings.base_url}/responses", {"json": body}

                    if response_id:
                        method = "GET"
                        url = f"{settings.base_url}/responses/{quote(response_id, safe='')}"
                        options = {"params": {"stream": "true", "starting_after": cursor}}

                    try:
                        async with client.stream(method, url, **options) as upstream:
                            LLM_HTTP.labels(profile.model, feature, str(upstream.status_code)).inc()
                            if upstream.is_error:
                                await upstream.aread()
                                raise RuntimeError(f"OpenAI research API returned HTTP {upstream.status_code}")

                            async for event in read_events(upstream):
                                event_type = event["type"]
                                sequence = event.get("sequence_number")
                                if event_type.startswith("response.") and sequence is not None:
                                    if cursor is not None and sequence <= cursor:
                                        continue
                                    cursor = sequence

                                if event_type == "response.created":
                                    response_id = event["response"]["id"] if profile.background else None
                                    yield "response", {"id": event["response"]["id"]}
                                elif event_type == "response.output_text.delta":
                                    yield "delta", event["delta"]
                                elif event_type == "keepalive":
                                    yield "ping", None
                                elif event_type == "response.reasoning_summary_text.delta":
                                    progress = (progress + event["delta"])[-4000:]
                                    yield "progress", progress
                                elif event_type == "response.web_search_call.searching":
                                    yield "progress", "Ищу и проверяю научные источники."
                                elif event_type == "response.output_text.annotation.added":
                                    yield "sources", [event["annotation"]]
                                elif event_type == "response.output_item.done":
                                    item = event["item"]
                                    if item["type"] == "web_search_call":
                                        action = item.get("action", {})
                                        LLM_TOOL_CALLS.labels(profile.model, feature, action.get("type", "unknown")).inc()
                                        yield "sources", action.get("sources", [])
                                elif event_type in {"response.completed", "response.failed", "response.incomplete"}:
                                    result = event["response"]
                                    yield "usage", result.get("usage")
                                    check_completion(result)
                                    answer, sources = final_result(result)
                                    if not answer.strip():
                                        raise RuntimeError("OpenAI research returned an empty response")
                                    completed = True
                                    yield "sources", sources
                                    yield "replace", answer
                                    return
                                elif event_type == "error":
                                    raise RuntimeError(f"OpenAI research stream error: {event.get('message', 'unknown')}")

                        raise httpx.ReadError("OpenAI stream ended before completion")
                    except (httpx.TransportError, json.JSONDecodeError):
                        if not response_id or cursor is None or attempts >= settings.stream_retries:
                            raise
                        attempts += 1
                        yield "progress", "Восстанавливаю соединение с исследованием."
                        await asyncio.sleep(min(attempts, 3))
        finally:
            if response_id and not completed:
                # Закрытие нашего запроса должно остановить платную фоновую задачу.
                with anyio.CancelScope(shield=True):
                    try:
                        await client.post(
                            f"{settings.base_url}/responses/{quote(response_id, safe='')}/cancel",
                            timeout=10,
                        )
                    except httpx.HTTPError:
                        pass
