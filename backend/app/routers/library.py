from __future__ import annotations

from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query, Response, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.core.deps import get_current_user
from app.db import get_db
from app.models import ReadingStatus, User, UserLibraryItem
from app.schemas.papers import LibraryItemOut, LibraryListOut, LibraryPatchRequest
from app.schemas.serializers import serialize_library_item
from app.services import papers as paper_service

router = APIRouter(prefix="/library", tags=["library"])


@router.get("", response_model=LibraryListOut)
def list_library(
    page: int = Query(1, ge=1),
    limit: int = Query(20, ge=1, le=100),
    status_filter: ReadingStatus | None = Query(None, alias="status"),
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> LibraryListOut:
    rows, total = paper_service.list_library(
        db,
        current_user.id,
        page=page,
        limit=limit,
        status=status_filter,
    )
    return LibraryListOut(
        items=[serialize_library_item(item, paper) for item, paper in rows],
        page=page,
        limit=limit,
        total=total,
    )


@router.patch("/{paper_id}", response_model=LibraryItemOut)
def patch_library_item(
    paper_id: UUID,
    body: LibraryPatchRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> LibraryItemOut:
    item = db.scalar(
        select(UserLibraryItem).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if item is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Library item not found")

    if body.status is not None:
        item.status = body.status
    if body.favorite is not None:
        item.favorite = body.favorite
    db.commit()

    paper = paper_service.get_paper(db, paper_id)
    assert paper is not None
    return serialize_library_item(item, paper)


@router.delete("/{paper_id}", status_code=status.HTTP_204_NO_CONTENT, response_class=Response)
def delete_library_item(
    paper_id: UUID,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
) -> Response:
    item = db.scalar(
        select(UserLibraryItem).where(
            UserLibraryItem.user_id == current_user.id,
            UserLibraryItem.paper_id == paper_id,
        )
    )
    if item is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Library item not found")
    db.delete(item)
    db.commit()
    return Response(status_code=status.HTTP_204_NO_CONTENT)
