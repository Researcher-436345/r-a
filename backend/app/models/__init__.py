"""Импортируйте сюда все модели — Alembic читает metadata через Base."""

from app.models.annotation import Annotation
from app.models.base import Base
from app.models.paper import (
    Author,
    Paper,
    PaperAuthor,
    PaperVersion,
    PaperVersionStatus,
    ReadingStatus,
    UserLibraryItem,
)
from app.models.user import User

__all__ = [
    "Base",
    "User",
    "Paper",
    "PaperVersion",
    "PaperVersionStatus",
    "Author",
    "PaperAuthor",
    "UserLibraryItem",
    "ReadingStatus",
    "Annotation",
]
