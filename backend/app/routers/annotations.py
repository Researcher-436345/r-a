from __future__ import annotations

from uuid import UUID

from fastapi import APIRouter, Depends, Response, status
from sqlalchemy.orm import Session

from app.core.deps import get_current_user
from app.db import get_db
from app.models import User
from app.schemas.annotations import (
    AnnotationCreateRequest,
    AnnotationOut,
    AnnotationPatchRequest,
)
from app.services import annotations as annotation_service

router = APIRouter(tags=["annotations"])


@router.get("/papers/{paper_id}/annotations", response_model=list[AnnotationOut])
def list_paper_annotations(
    paper_id: UUID,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> list[AnnotationOut]:
    return annotation_service.list_annotations(db, current_user.id, paper_id)


@router.post(
    "/papers/{paper_id}/annotations",
    response_model=AnnotationOut,
    status_code=status.HTTP_201_CREATED,
)
def create_paper_annotation(
    paper_id: UUID,
    body: AnnotationCreateRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> AnnotationOut:
    return annotation_service.create_annotation(db, current_user.id, paper_id, body)


@router.patch("/annotations/{annotation_id}", response_model=AnnotationOut)
def patch_annotation(
    annotation_id: UUID,
    body: AnnotationPatchRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> AnnotationOut:
    return annotation_service.patch_annotation(db, current_user.id, annotation_id, body)


@router.delete("/annotations/{annotation_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_annotation(
    annotation_id: UUID,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> Response:
    annotation_service.delete_annotation(db, current_user.id, annotation_id)
    return Response(status_code=status.HTTP_204_NO_CONTENT)
