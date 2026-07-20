from __future__ import annotations

from fastapi import APIRouter, Depends, Query

from app.core.deps import get_current_user
from app.models import User
from app.schemas.feed import TrendingFeedOut
from app.services import feed as feed_service

router = APIRouter(prefix="/feed", tags=["feed"])


@router.get("/trending", response_model=TrendingFeedOut)
def trending(
    category: str = Query("cs.AI", min_length=2, max_length=64),
    limit: int = Query(20, ge=1, le=50),
    _current_user: User = Depends(get_current_user),
) -> TrendingFeedOut:
    items, cached = feed_service.get_trending_feed(category=category, limit=limit)
    return TrendingFeedOut(items=items, category=category, cached=cached)
