from __future__ import annotations

from uuid import UUID

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile, status
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.deps import get_current_user
from app.db import get_db
from app.models import User, UserLibraryItem
from app.schemas.papers import AddByArxivRequest, AddByDoiRequest, PaperOut, PdfUrlOut
from app.schemas.ai import (
    ChatRequest,
    ChatReply,
    ExplainRequest,
    ExplainReply,
    TranslateRequest,
    TranslateReply,
)
from app.schemas.serializers import serialize_paper
from app.services import llm as llm_service
from app.services import papers as paper_service
from app.services import translate as translate_service
from app.services.storage import presigned_get_url

router = APIRouter(prefix="/papers", tags=["papers"])


@router.post("/arxiv", response_model=PaperOut, status_code=status.HTTP_201_CREATED)
def add_by_arxiv(
    body: AddByArxivRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> PaperOut:
    try:
        paper, _item, _created = paper_service.create_paper_from_arxiv(
            db, current_user.id, body.arxiv_id
        )
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail=f"Failed to fetch arXiv metadata: {exc}",
        ) from exc
    return serialize_paper(paper)


@router.post("/doi", response_model=PaperOut, status_code=status.HTTP_201_CREATED)
def add_by_doi(
    body: AddByDoiRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> PaperOut:
    try:
        paper, _item, _created = paper_service.create_paper_from_doi(db, current_user.id, body.doi)
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail=f"Failed to fetch Crossref metadata: {exc}",
        ) from exc
    return serialize_paper(paper)


@router.post("/upload", response_model=PaperOut, status_code=status.HTTP_201_CREATED)
async def upload_pdf(
    file: UploadFile = File(...),
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> PaperOut:
    if file.content_type not in {"application/pdf", "application/octet-stream"} and not (
        file.filename or ""
    ).lower().endswith(".pdf"):
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="Only PDF files are supported")

    content = await file.read()
    if not content:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="Empty file")
    if len(content) > 50 * 1024 * 1024:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="File too large (max 50MB)")

    try:
        paper, _item = paper_service.create_paper_from_upload(
            db,
            current_user.id,
            filename=file.filename or "paper.pdf",
            content=content,
        )
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail=f"Failed to store PDF: {exc}",
        ) from exc
    return serialize_paper(paper)


@router.get("/{paper_id}", response_model=PaperOut)
def get_paper(
    paper_id: UUID,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> PaperOut:
    paper = paper_service.get_paper(db, paper_id)
    if paper is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    # Access if in user's library
    from sqlalchemy import select

    from app.models import UserLibraryItem

    in_library = db.scalar(
        select(UserLibraryItem.id).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if in_library is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    return serialize_paper(paper)


@router.get("/{paper_id}/pdf-url", response_model=PdfUrlOut)
def get_pdf_url(
    paper_id: UUID,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> PdfUrlOut:
    from sqlalchemy import select

    from app.models import PaperVersionStatus, UserLibraryItem

    in_library = db.scalar(
        select(UserLibraryItem.id).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if in_library is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    paper = paper_service.get_paper(db, paper_id)
    if paper is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    version = paper_service.latest_version(paper)
    if version is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="PDF not available yet")

    if version.pdf_key and str(version.status) == PaperVersionStatus.ready.value:
        return PdfUrlOut(
            url=presigned_get_url(version.pdf_key),
            expires_in=settings.s3_presign_expire_seconds,
            status="ready",
            source="storage",
        )

    if str(version.status) == PaperVersionStatus.failed.value:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=version.error_message or "PDF processing failed",
        )

    # PDF ещё качается воркером — клиент должен поллить
    raise HTTPException(
        status_code=status.HTTP_409_CONFLICT,
        detail="PDF is still processing",
    )


@router.post("/{paper_id}/retry-pdf", response_model=PaperOut)
def retry_pdf(
    paper_id: UUID,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> PaperOut:
    try:
        paper = paper_service.retry_pdf_processing(db, current_user.id, paper_id)
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc
    paper = paper_service.get_paper(db, paper.id)
    assert paper is not None
    return serialize_paper(paper)


@router.post("/{paper_id}/chat", response_model=ChatReply)
def chat(
    paper_id: UUID,
    body: ChatRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> ChatReply:
    from sqlalchemy import select

    in_library = db.scalar(
        select(UserLibraryItem.id).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if in_library is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    paper = paper_service.get_paper(db, paper_id)
    if paper is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    reply = llm_service.chat_about_paper(
        paper,
        body.message,
        body.context_text,
    )
    return ChatReply(reply=reply)


@router.post("/{paper_id}/explain", response_model=ExplainReply)
def explain(
    paper_id: UUID,
    body: ExplainRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> ExplainReply:
    from sqlalchemy import select

    in_library = db.scalar(
        select(UserLibraryItem.id).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if in_library is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    paper = paper_service.get_paper(db, paper_id)
    if paper is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    reply = llm_service.explain_fragment(
        paper,
        body.text,
        body.question,
    )
    return ExplainReply(reply=reply)


@router.post("/{paper_id}/translate", response_model=TranslateReply)
def translate(
    paper_id: UUID,
    body: TranslateRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> TranslateReply:
    from sqlalchemy import select

    in_library = db.scalar(
        select(UserLibraryItem.id).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if in_library is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    paper = paper_service.get_paper(db, paper_id)
    if paper is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not found")

    translation, detected = translate_service.translate_text(
        paper,
        body.text,
        body.target_lang,
    )
    return TranslateReply(translation=translation, detected_source=detected)
