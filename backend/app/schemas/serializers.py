from __future__ import annotations

from app.models import Paper, PaperAuthor, UserLibraryItem
from app.schemas.papers import AuthorOut, LibraryItemOut, PaperOut, PaperVersionOut
from app.services.papers import latest_version


def serialize_paper(paper: Paper) -> PaperOut:
    authors = [
        AuthorOut(id=link.author.id, name=link.author.name)
        for link in sorted(paper.authors, key=lambda item: item.position)
        if isinstance(link, PaperAuthor) and link.author is not None
    ]
    version = latest_version(paper)
    return PaperOut(
        id=paper.id,
        title=paper.title,
        abstract=paper.abstract,
        year=paper.year,
        venue=paper.venue,
        doi=paper.doi,
        arxiv_id=paper.arxiv_id,
        authors=authors,
        latest_version=PaperVersionOut.model_validate(version) if version else None,
        created_at=paper.created_at,
    )


def serialize_library_item(item: UserLibraryItem, paper: Paper) -> LibraryItemOut:
    return LibraryItemOut(
        id=item.id,
        status=item.status,
        favorite=item.favorite,
        added_at=item.added_at,
        paper=serialize_paper(paper),
    )
