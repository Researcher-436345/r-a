from __future__ import annotations

import re
from io import BytesIO

from pypdf import PdfReader


def _clean_title(value: str | None) -> str | None:
    if not value:
        return None
    cleaned = re.sub(r"\s+", " ", value).strip()
    if len(cleaned) < 3:
        return None
    # отсекаем мусор вроде filename.pdf / Untitled
    lowered = cleaned.lower()
    if lowered in {"untitled", "document", "microsoft word", "pdf"}:
        return None
    if cleaned.lower().endswith(".pdf"):
        cleaned = cleaned[:-4].strip()
    return cleaned or None


def extract_title_from_pdf(content: bytes, fallback: str) -> str:
    """Достаёт title из PDF metadata или с первой страницы; иначе fallback (имя файла)."""
    try:
        reader = PdfReader(BytesIO(content))
    except Exception:  # noqa: BLE001
        return fallback

    try:
        meta = reader.metadata
        meta_title = _clean_title(getattr(meta, "title", None) if meta else None)
        if meta_title:
            return meta_title
    except Exception:  # noqa: BLE001
        pass

    try:
        if not reader.pages:
            return fallback
        text = reader.pages[0].extract_text() or ""
    except Exception:  # noqa: BLE001
        return fallback

    lines = [re.sub(r"\s+", " ", line).strip() for line in text.splitlines()]
    lines = [line for line in lines if len(line) >= 8]

    for line in lines[:8]:
        # эвристика: заголовок обычно не слишком длинный и не «Authors: …»
        if len(line) > 180:
            continue
        if re.match(r"^(abstract|introduction|authors?|arxiv|doi)\b", line, re.I):
            continue
        if re.match(r"^\d+$", line):
            continue
        cleaned = _clean_title(line)
        if cleaned:
            return cleaned

    return fallback
