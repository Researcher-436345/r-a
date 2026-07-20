from __future__ import annotations

from datetime import datetime
from pydantic import BaseModel, Field


class TrendingPaperOut(BaseModel):
    arxiv_id: str
    title: str
    abstract: str | None = None
    authors: list[str] = Field(default_factory=list)
    published_at: datetime
    category: str
    popularity_score: int
    pdf_url: str
    abs_url: str


class TrendingFeedOut(BaseModel):
    items: list[TrendingPaperOut]
    category: str
    cached: bool = False
