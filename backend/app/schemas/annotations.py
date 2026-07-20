from __future__ import annotations

from datetime import datetime
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class AnnotationRect(BaseModel):
    x: float
    y: float
    w: float
    h: float


class AnnotationCreateRequest(BaseModel):
    page: int = Field(ge=1)
    rect: AnnotationRect | None = None
    selected_text: str = Field(min_length=1, max_length=10000)
    note: str = Field(default="", max_length=10000)
    color: str = Field(default="#facc15", max_length=16)


class AnnotationPatchRequest(BaseModel):
    note: str = Field(default="", max_length=10000)


class AnnotationOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: UUID
    paper_id: UUID
    page: int
    rect: dict | None = None
    selected_text: str
    note: str
    color: str
    created_at: datetime
    updated_at: datetime
