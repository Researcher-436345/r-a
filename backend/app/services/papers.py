from __future__ import annotations

import hashlib
import re
import uuid
from uuid import UUID

from sqlalchemy import Select, func, select
from sqlalchemy.orm import Session, selectinload

from app.models import (
    Author,
    Paper,
    PaperAuthor,
    PaperVersion,
    PaperVersionStatus,
    ReadingStatus,
    UserLibraryItem,
)
from app.services import arxiv as arxiv_service
from app.services import crossref as crossref_service
from app.services.storage import upload_bytes
from app.services.pdf_meta import extract_title_from_pdf


def normalize_author_name(name: str) -> str:
    return re.sub(r"\s+", " ", name.strip().lower())


def get_or_create_author(db: Session, name: str) -> Author:
    normalized = normalize_author_name(name)
    author = db.scalar(select(Author).where(Author.normalized_name == normalized))
    if author is None:
        author = Author(name=name.strip(), normalized_name=normalized)
        db.add(author)
        db.flush()
    return author


def attach_authors(db: Session, paper: Paper, author_names: list[str]) -> None:
    for position, name in enumerate(author_names):
        author = get_or_create_author(db, name)
        db.add(PaperAuthor(paper_id=paper.id, author_id=author.id, position=position))


def add_to_library(db: Session, user_id: UUID, paper_id: UUID) -> UserLibraryItem:
    item = db.scalar(
        select(UserLibraryItem).where(
            UserLibraryItem.user_id == user_id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if item is not None:
        return item

    item = UserLibraryItem(user_id=user_id, paper_id=paper_id, status=ReadingStatus.unread)
    db.add(item)
    db.flush()
    return item


def latest_version(paper: Paper) -> PaperVersion | None:
    if not paper.versions:
        return None
    return sorted(paper.versions, key=lambda item: item.version_number, reverse=True)[0]


def paper_query() -> Select[tuple[Paper]]:
    return select(Paper).options(
        selectinload(Paper.authors).selectinload(PaperAuthor.author),
        selectinload(Paper.versions),
    )


def get_paper(db: Session, paper_id: UUID) -> Paper | None:
    return db.scalar(paper_query().where(Paper.id == paper_id))


def create_paper_from_arxiv(db: Session, user_id: UUID, arxiv_input: str) -> tuple[Paper, UserLibraryItem, bool]:
    arxiv_id = arxiv_service.canonical_arxiv_id(arxiv_service.normalize_arxiv_id(arxiv_input))
    existing = db.scalar(paper_query().where(Paper.arxiv_id == arxiv_id))
    if existing is not None:
        item = add_to_library(db, user_id, existing.id)
        db.commit()
        paper = get_paper(db, existing.id)
        assert paper is not None
        return paper, item, False

    meta = arxiv_service.fetch_arxiv_metadata(arxiv_id)
    paper = Paper(
        title=meta.title,
        abstract=meta.abstract,
        year=meta.year,
        arxiv_id=meta.arxiv_id,
        venue="arXiv",
    )
    db.add(paper)
    db.flush()
    attach_authors(db, paper, meta.authors)

    version = PaperVersion(
        paper_id=paper.id,
        version_number=1,
        source="arxiv",
        source_url=meta.pdf_url,
        status=PaperVersionStatus.processing,
    )
    db.add(version)
    item = add_to_library(db, user_id, paper.id)
    db.commit()

    paper = get_paper(db, paper.id)
    assert paper is not None
    version = latest_version(paper)
    assert version is not None

    from app.worker.tasks import process_arxiv_pdf

    process_arxiv_pdf.delay(str(version.id))
    return paper, item, True


def create_paper_from_doi(db: Session, user_id: UUID, doi_input: str) -> tuple[Paper, UserLibraryItem, bool]:
    doi = crossref_service.normalize_doi(doi_input)
    existing = db.scalar(paper_query().where(Paper.doi == doi))
    if existing is not None:
        item = add_to_library(db, user_id, existing.id)
        db.commit()
        paper = get_paper(db, existing.id)
        assert paper is not None
        return paper, item, False

    meta = crossref_service.fetch_crossref_metadata(doi)
    paper = Paper(
        title=meta.title,
        abstract=meta.abstract,
        year=meta.year,
        doi=meta.doi,
        venue=meta.venue,
    )
    db.add(paper)
    db.flush()
    attach_authors(db, paper, meta.authors)

    version = PaperVersion(
        paper_id=paper.id,
        version_number=1,
        source="doi",
        source_url=f"https://doi.org/{meta.doi}",
        status=PaperVersionStatus.ready,
    )
    db.add(version)
    item = add_to_library(db, user_id, paper.id)
    db.commit()

    paper = get_paper(db, paper.id)
    assert paper is not None
    return paper, item, True


def create_paper_from_upload(
    db: Session,
    user_id: UUID,
    filename: str,
    content: bytes,
) -> tuple[Paper, UserLibraryItem]:
    title_fallback = filename.rsplit(".", 1)[0].replace("_", " ").strip() or "Uploaded PDF"
    title = extract_title_from_pdf(content, title_fallback)
    sha256 = hashlib.sha256(content).hexdigest()

    existing_version = db.scalar(select(PaperVersion).where(PaperVersion.sha256 == sha256))
    if existing_version is not None:
        item = add_to_library(db, user_id, existing_version.paper_id)
        db.commit()
        paper = get_paper(db, existing_version.paper_id)
        assert paper is not None
        return paper, item

    paper = Paper(title=title)
    db.add(paper)
    db.flush()

    key = f"papers/{paper.id}/{uuid.uuid4().hex}.pdf"
    upload_bytes(key, content)

    version = PaperVersion(
        paper_id=paper.id,
        version_number=1,
        source="upload",
        pdf_key=key,
        sha256=sha256,
        size_bytes=len(content),
        status=PaperVersionStatus.processing,
    )
    db.add(version)
    item = add_to_library(db, user_id, paper.id)
    db.commit()

    paper = get_paper(db, paper.id)
    assert paper is not None
    version = latest_version(paper)
    assert version is not None

    from app.worker.tasks import finalize_uploaded_pdf

    finalize_uploaded_pdf.delay(str(version.id))
    return paper, item


def retry_pdf_processing(db: Session, user_id: UUID, paper_id: UUID) -> Paper:
    item = db.scalar(
        select(UserLibraryItem).where(
            UserLibraryItem.user_id == user_id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if item is None:
        raise ValueError("Paper not in library")

    paper = get_paper(db, paper_id)
    if paper is None:
        raise ValueError("Paper not found")

    version = latest_version(paper)
    if version is None:
        raise ValueError("No PDF version found")

    if version.status == PaperVersionStatus.ready and version.pdf_key:
        return paper

    version.status = PaperVersionStatus.processing
    version.error_message = None
    db.commit()

    if version.source == "arxiv":
        from app.worker.tasks import process_arxiv_pdf

        process_arxiv_pdf.delay(str(version.id))
    elif version.source == "upload" and version.pdf_key:
        from app.worker.tasks import finalize_uploaded_pdf

        finalize_uploaded_pdf.delay(str(version.id))
    else:
        raise ValueError("Cannot retry this PDF source")

    return paper


def list_library(
    db: Session,
    user_id: UUID,
    *,
    page: int = 1,
    limit: int = 20,
    status: ReadingStatus | None = None,
) -> tuple[list[tuple[UserLibraryItem, Paper]], int]:
    filters = [UserLibraryItem.user_id == user_id]
    if status is not None:
        filters.append(UserLibraryItem.status == status)

    total = db.scalar(select(func.count()).select_from(UserLibraryItem).where(*filters)) or 0

    rows = db.execute(
        select(UserLibraryItem, Paper)
        .join(Paper, Paper.id == UserLibraryItem.paper_id)
        .options(
            selectinload(Paper.authors).selectinload(PaperAuthor.author),
            selectinload(Paper.versions),
        )
        .where(*filters)
        .order_by(UserLibraryItem.added_at.desc())
        .offset((page - 1) * limit)
        .limit(limit)
    ).all()

    return [(item, paper) for item, paper in rows], int(total)
