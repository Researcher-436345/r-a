from __future__ import annotations

from uuid import UUID

from fastapi import HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.models import Annotation, UserLibraryItem
from app.schemas.annotations import AnnotationCreateRequest, AnnotationPatchRequest


def _require_library_item(db: Session, user_id: UUID, paper_id: UUID) -> UserLibraryItem:
    item = db.scalar(
        select(UserLibraryItem).where(
            UserLibraryItem.user_id == user_id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if item is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Paper not in library")
    return item


def list_annotations(db: Session, user_id: UUID, paper_id: UUID) -> list[Annotation]:
    _require_library_item(db, user_id, paper_id)
    return list(
        db.scalars(
            select(Annotation)
            .where(Annotation.user_id == user_id, Annotation.paper_id == paper_id)
            .order_by(Annotation.page, Annotation.created_at)
        ).all()
    )


def create_annotation(
    db: Session,
    user_id: UUID,
    paper_id: UUID,
    body: AnnotationCreateRequest,
) -> Annotation:
    _require_library_item(db, user_id, paper_id)
    annotation = Annotation(
        paper_id=paper_id,
        user_id=user_id,
        page=body.page,
        rect=body.rect.model_dump() if body.rect else None,
        selected_text=body.selected_text.strip(),
        note=body.note.strip(),
        color=body.color,
    )
    db.add(annotation)
    db.commit()
    db.refresh(annotation)
    return annotation


def get_annotation(db: Session, user_id: UUID, annotation_id: UUID) -> Annotation:
    annotation = db.get(Annotation, annotation_id)
    if annotation is None or annotation.user_id != user_id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Annotation not found")
    return annotation


def patch_annotation(
    db: Session,
    user_id: UUID,
    annotation_id: UUID,
    body: AnnotationPatchRequest,
) -> Annotation:
    annotation = get_annotation(db, user_id, annotation_id)
    annotation.note = body.note.strip()
    db.commit()
    db.refresh(annotation)
    return annotation


def delete_annotation(db: Session, user_id: UUID, annotation_id: UUID) -> None:
    annotation = get_annotation(db, user_id, annotation_id)
    db.delete(annotation)
    db.commit()
