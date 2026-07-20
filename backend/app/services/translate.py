from __future__ import annotations

import logging

import httpx

from app.core.config import settings
from app.services import llm as llm_service
from app.models import Paper

logger = logging.getLogger(__name__)


def _translate_mymemory(text: str, target_lang: str) -> tuple[str, str | None]:
    """Бесплатный fallback без гео-блоков LLM (MyMemory)."""
    langpair = f"autodetect|{target_lang}"
    with httpx.Client(timeout=20.0) as client:
        response = client.get(
            "https://api.mymemory.translated.net/get",
            params={"q": text[:450], "langpair": langpair},
        )
        response.raise_for_status()
        data = response.json()
    translated = (data.get("responseData") or {}).get("translatedText") or ""
    detected = None
    matches = data.get("matches") or []
    if matches and isinstance(matches[0], dict):
        detected = matches[0].get("source")
    if not translated.strip():
        raise RuntimeError("Empty translation from MyMemory")
    return translated.strip(), detected


def translate_text(paper: Paper, text: str, target_lang: str = "ru") -> tuple[str, str | None]:
    """
    Перевод выделения.
    Сначала пробуем LLM (если ключ есть), иначе MyMemory.
    """
    cleaned = text.strip()
    if not cleaned:
        return "", None

    if settings.llm_api_key:
        reply = llm_service.explain_fragment(
            paper,
            cleaned,
            question=(
                f"Translate the fragment into {target_lang}. "
                "Return only the translation, no quotes or commentary."
            ),
        )
        # если LLM вернул ошибку провайдера — уйдём в fallback
        lowered = reply.lower()
        if reply and not lowered.startswith("ошибка") and "не настроен" not in lowered:
            return reply.strip(), None

    try:
        return _translate_mymemory(cleaned, target_lang)
    except Exception as exc:  # noqa: BLE001
        logger.exception("Translation failed")
        return f"Не удалось перевести: {exc}", None
