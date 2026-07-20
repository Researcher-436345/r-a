from __future__ import annotations

import json
import logging
import math
import xml.etree.ElementTree as ET
from datetime import datetime, timezone

import httpx
import redis

from app.core.config import settings
from app.schemas.feed import TrendingPaperOut
from app.services.arxiv import ATOM_NS, canonical_arxiv_id

logger = logging.getLogger(__name__)

CACHE_TTL_SECONDS = 3600
DEFAULT_CATEGORY = "cs.AI"


def _redis_client() -> redis.Redis:
    return redis.Redis.from_url(settings.redis_url, decode_responses=True)


def _cache_key(category: str, limit: int) -> str:
    return f"feed:trending:{category}:{limit}"


def _parse_published(value: str | None) -> datetime:
    if not value:
        return datetime.now(timezone.utc)
    # arXiv: 2026-07-14T18:00:00Z
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return datetime.now(timezone.utc)


def _popularity_score(published_at: datetime) -> int:
    """MVP: newer = higher. Rough relative score for UI."""
    now = datetime.now(timezone.utc)
    age_days = max(0.0, (now - published_at).total_seconds() / 86_400)
    return max(1, int(1000 * math.exp(-age_days / 14)))


def fetch_trending_from_arxiv(category: str, limit: int) -> list[TrendingPaperOut]:
    query = f"cat:{category}"
    url = (
        "https://export.arxiv.org/api/query"
        f"?search_query={query}"
        f"&start=0&max_results={limit}"
        "&sortBy=submittedDate&sortOrder=descending"
    )
    with httpx.Client(timeout=30.0) as client:
        response = client.get(url)
        response.raise_for_status()

    root = ET.fromstring(response.text)
    items: list[TrendingPaperOut] = []

    for entry in root.findall("atom:entry", ATOM_NS):
        id_text = entry.findtext("atom:id", default="", namespaces=ATOM_NS) or ""
        # https://arxiv.org/abs/2205.12446v1
        arxiv_raw = id_text.rstrip("/").split("/")[-1]
        arxiv_id = canonical_arxiv_id(arxiv_raw)

        title = " ".join(
            (entry.findtext("atom:title", default="", namespaces=ATOM_NS) or "").split()
        )
        abstract = entry.findtext("atom:summary", default=None, namespaces=ATOM_NS)
        if abstract:
            abstract = " ".join(abstract.split())

        authors = [
            " ".join((node.findtext("atom:name", default="", namespaces=ATOM_NS) or "").split())
            for node in entry.findall("atom:author", ATOM_NS)
        ]
        authors = [name for name in authors if name]

        published_at = _parse_published(
            entry.findtext("atom:published", default=None, namespaces=ATOM_NS)
        )

        primary_category = category
        cat_node = entry.find("arxiv:primary_category", ATOM_NS)
        if cat_node is not None and cat_node.attrib.get("term"):
            primary_category = cat_node.attrib["term"]

        pdf_url = f"https://arxiv.org/pdf/{arxiv_id}.pdf"
        for link in entry.findall("atom:link", ATOM_NS):
            if link.attrib.get("title") == "pdf" or link.attrib.get("type") == "application/pdf":
                href = link.attrib.get("href")
                if href:
                    pdf_url = href
                    break

        items.append(
            TrendingPaperOut(
                arxiv_id=arxiv_id,
                title=title or f"arXiv:{arxiv_id}",
                abstract=abstract,
                authors=authors,
                published_at=published_at,
                category=primary_category,
                popularity_score=_popularity_score(published_at),
                pdf_url=pdf_url,
                abs_url=f"https://arxiv.org/abs/{arxiv_id}",
            )
        )

    return items


def get_trending_feed(
    *,
    category: str = DEFAULT_CATEGORY,
    limit: int = 20,
) -> tuple[list[TrendingPaperOut], bool]:
    category = (category or DEFAULT_CATEGORY).strip()
    limit = max(1, min(limit, 50))
    key = _cache_key(category, limit)

    try:
        client = _redis_client()
        cached = client.get(key)
        if cached:
            payload = json.loads(cached)
            items = [TrendingPaperOut.model_validate(item) for item in payload]
            return items, True
    except Exception:  # noqa: BLE001
        logger.warning("Redis cache read failed for trending feed", exc_info=True)

    items = fetch_trending_from_arxiv(category, limit)

    try:
        client = _redis_client()
        client.setex(
            key,
            CACHE_TTL_SECONDS,
            json.dumps([item.model_dump(mode="json") for item in items]),
        )
    except Exception:  # noqa: BLE001
        logger.warning("Redis cache write failed for trending feed", exc_info=True)

    return items, False
