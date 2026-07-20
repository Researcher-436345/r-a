from __future__ import annotations

import logging

import httpx

from app.core.config import settings
from app.models import Paper, PaperAuthor

logger = logging.getLogger(__name__)


def _paper_context(paper: Paper) -> str:
    authors = [
        link.author.name
        for link in sorted(paper.authors, key=lambda item: item.position)
        if isinstance(link, PaperAuthor) and link.author is not None
    ]
    author_line = ", ".join(authors) if authors else "unknown"
    abstract = (paper.abstract or "").strip() or "No abstract available."
    year = str(paper.year) if paper.year else "unknown"

    return (
        f"Title: {paper.title}\n"
        f"Authors: {author_line}\n"
        f"Year: {year}\n"
        f"Abstract:\n{abstract}"
    )


def _openai_compatible_headers() -> dict[str, str]:
    headers = {
        "Authorization": f"Bearer {settings.llm_api_key}",
        "Content-Type": "application/json",
    }
    if "openrouter.ai" in settings.llm_base_url:
        headers["HTTP-Referer"] = settings.llm_http_referer
        headers["X-Title"] = settings.llm_app_title
    return headers


def _provider_label() -> str:
    base_url = settings.llm_base_url.lower()
    if "aitunnel.ru" in base_url:
        return "AITunnel"
    if "openrouter.ai" in base_url:
        return "OpenRouter"
    if "deepseek.com" in base_url:
        return "DeepSeek"
    return "LLM API"


def _format_http_error(provider: str, exc: httpx.HTTPStatusError) -> str:
    detail = ""
    try:
        payload = exc.response.json()
        if isinstance(payload.get("error"), dict):
            detail = payload["error"].get("message", "") or payload["error"].get("detail", "")
        elif isinstance(payload.get("error"), str):
            detail = payload["error"]
        elif isinstance(payload.get("detail"), str):
            detail = payload["detail"]
    except Exception:  # noqa: BLE001
        detail = ""
    lowered = detail.lower()
    if "security policy" in lowered or "location is not supported" in lowered:
        detail = (
            f"{detail}. OpenRouter и прямой Gemini из РФ часто недоступны — "
            "используй AITunnel: LLM_BASE_URL=https://api.aitunnel.ru/v1"
        )
    suffix = f": {detail}" if detail else ""
    return f"Ошибка {provider}{suffix}"


def _request_openai_compatible(system_prompt: str, user_prompt: str) -> str:
    if not settings.llm_api_key:
        return (
            "LLM не настроен. Добавь `LLM_API_KEY` в `.env`. "
            "Из РФ: зарегистрируйся на aitunnel.ru, пополни баланс и вставь ключ AITunnel."
        )

    url = f"{settings.llm_base_url.rstrip('/')}/chat/completions"
    headers = _openai_compatible_headers()
    payload = {
        "model": settings.llm_model,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
        "temperature": 0.2,
    }

    provider = _provider_label()

    try:
        with httpx.Client(timeout=float(settings.llm_timeout_seconds)) as client:
            response = client.post(url, headers=headers, json=payload)
            response.raise_for_status()
            data = response.json()
    except httpx.HTTPStatusError as exc:
        logger.exception("LLM request failed")
        return _format_http_error(provider, exc)
    except httpx.HTTPError as exc:
        logger.exception("LLM request failed")
        return f"Ошибка {provider}: {exc}"

    choices = data.get("choices") or []
    if not choices:
        return "LLM API вернул пустой ответ."

    message = choices[0].get("message") or {}
    content = message.get("content")
    if isinstance(content, str) and content.strip():
        return content.strip()

    return "LLM API вернул ответ без текста."


def _request_gemini(system_prompt: str, user_prompt: str) -> str:
    if not settings.llm_api_key:
        return "Gemini не настроен. Добавь `LLM_API_KEY` из Google AI Studio в `.env`."

    base_url = settings.llm_base_url.rstrip("/")
    model = settings.llm_model
    url = f"{base_url}/models/{model}:generateContent"
    payload = {
        "system_instruction": {
            "parts": [{"text": system_prompt}],
        },
        "contents": [
            {
                "role": "user",
                "parts": [{"text": user_prompt}],
            }
        ],
        "generationConfig": {
            "temperature": 0.2,
        },
    }

    try:
        with httpx.Client(timeout=float(settings.llm_timeout_seconds)) as client:
            response = client.post(
                url,
                params={"key": settings.llm_api_key},
                json=payload,
            )
            response.raise_for_status()
            data = response.json()
    except httpx.HTTPStatusError as exc:
        logger.exception("Gemini request failed")
        detail = ""
        try:
            detail = exc.response.json().get("error", {}).get("message", "")
        except Exception:  # noqa: BLE001
            detail = ""
        suffix = f": {detail}" if detail else ""
        return f"Ошибка Gemini API{suffix}"
    except httpx.HTTPError as exc:
        logger.exception("Gemini request failed")
        return f"Ошибка Gemini API: {exc}"

    candidates = data.get("candidates") or []
    if not candidates:
        return "Gemini вернул пустой ответ."

    parts = (
        candidates[0]
        .get("content", {})
        .get("parts", [])
    )
    text_chunks = [part.get("text", "") for part in parts if isinstance(part, dict)]
    text = "".join(chunk for chunk in text_chunks if chunk).strip()
    if text:
        return text

    finish_reason = candidates[0].get("finishReason")
    if finish_reason:
        return f"Gemini вернул ответ без текста (`{finish_reason}`)."
    return "Gemini вернул ответ без текста."


def _request_llm(system_prompt: str, user_prompt: str) -> str:
    if settings.llm_provider == "gemini":
        return _request_gemini(system_prompt, user_prompt)
    return _request_openai_compatible(system_prompt, user_prompt)


def chat_about_paper(paper: Paper, message: str, context_text: str | None = None) -> str:
    paper_context = _paper_context(paper)
    extra_context = (context_text or "").strip()

    system_prompt = (
        "You are a helpful research assistant. "
        "Answer in the same language as the user. "
        "Ground your answer in the provided paper metadata and highlighted passages. "
        "If context is insufficient, say so plainly and avoid inventing details."
    )
    user_prompt = (
        f"Paper context:\n{paper_context}\n\n"
        f"Highlighted passages:\n{extra_context or 'None provided.'}\n\n"
        f"User question:\n{message.strip()}"
    )
    return _request_llm(system_prompt, user_prompt)


def explain_fragment(paper: Paper, text: str, question: str | None = None) -> str:
    paper_context = _paper_context(paper)
    ask = (question or "").strip() or "Explain this fragment simply and in context."

    system_prompt = (
        "You are a helpful research assistant. "
        "Explain selected paper fragments clearly, briefly, and accurately. "
        "Answer in the same language as the user."
    )
    user_prompt = (
        f"Paper context:\n{paper_context}\n\n"
        f"Fragment:\n{text.strip()}\n\n"
        f"Question:\n{ask}"
    )
    return _request_llm(system_prompt, user_prompt)
