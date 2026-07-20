from __future__ import annotations

from dataclasses import dataclass

import httpx


@dataclass
class CrossrefPaper:
    doi: str
    title: str
    abstract: str | None
    authors: list[str]
    year: int | None
    venue: str | None


def normalize_doi(value: str) -> str:
    raw = value.strip()
    raw = re_sub_prefix(raw)
    if not raw.lower().startswith("10."):
        raise ValueError("Invalid DOI")
    return raw.lower()


def re_sub_prefix(value: str) -> str:
    prefixes = (
        "https://doi.org/",
        "http://doi.org/",
        "https://dx.doi.org/",
        "http://dx.doi.org/",
        "doi:",
    )
    lowered = value.strip()
    for prefix in prefixes:
        if lowered.lower().startswith(prefix):
            return lowered[len(prefix) :].strip()
    return lowered.strip()


def fetch_crossref_metadata(doi: str) -> CrossrefPaper:
    url = f"https://api.crossref.org/works/{doi}"
    with httpx.Client(timeout=30.0, headers={"User-Agent": "researcher-mvp/0.1 (mailto:dev@local)"}) as client:
        response = client.get(url)
        if response.status_code == 404:
            raise ValueError("DOI not found")
        response.raise_for_status()
        message = response.json()["message"]

    titles = message.get("title") or []
    title = titles[0] if titles else f"DOI:{doi}"

    abstract = message.get("abstract")
    if isinstance(abstract, str):
        abstract = " ".join(abstract.replace("<jats:p>", " ").replace("</jats:p>", " ").split())

    authors: list[str] = []
    for item in message.get("author") or []:
        given = (item.get("given") or "").strip()
        family = (item.get("family") or "").strip()
        name = " ".join(part for part in (given, family) if part)
        if name:
            authors.append(name)

    year = None
    for key in ("published-print", "published-online", "created"):
        parts = (message.get(key) or {}).get("date-parts") or []
        if parts and parts[0]:
            year = int(parts[0][0])
            break

    venue_list = message.get("container-title") or []
    venue = venue_list[0] if venue_list else None

    return CrossrefPaper(
        doi=doi.lower(),
        title=title,
        abstract=abstract,
        authors=authors,
        year=year,
        venue=venue,
    )
