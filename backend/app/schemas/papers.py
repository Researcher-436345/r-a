from __future__ import annotations

from datetime import datetime
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field

from app.models import PaperVersionStatus, ReadingStatus


class AuthorOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: UUID
    name: str


class PaperVersionOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: UUID
    source: str
    status: PaperVersionStatus
    pdf_key: str | None = None
    size_bytes: int | None = None
    error_message: str | None = None


class PaperOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: UUID
    title: str
    abstract: str | None = None
    year: int | None = None
    venue: str | None = None
    doi: str | None = None
    arxiv_id: str | None = None
    authors: list[AuthorOut] = []
    latest_version: PaperVersionOut | None = None
    created_at: datetime


class LibraryItemOut(BaseModel):
    id: UUID
    status: ReadingStatus
    favorite: bool
    added_at: datetime
    paper: PaperOut


class LibraryListOut(BaseModel):
    items: list[LibraryItemOut]
    page: int
    limit: int
    total: int


class AddByArxivRequest(BaseModel):
    arxiv_id: str = Field(min_length=3, max_length=200)


class AddByDoiRequest(BaseModel):
    doi: str = Field(min_length=3, max_length=255)


class PdfUrlOut(BaseModel):
    url: str
    expires_in: int
    status: str = "ready"
    source: str = "storage"


class LibraryPatchRequest(BaseModel):
    status: ReadingStatus | None = None
    favorite: bool | None = None
