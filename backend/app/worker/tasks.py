from __future__ import annotations

import hashlib
import logging
import uuid
from uuid import UUID

from app.db import SessionLocal
from app.models import PaperVersion, PaperVersionStatus
from app.services import arxiv as arxiv_service
from app.services.storage import download_bytes, upload_bytes
from app.worker.celery_app import celery_app

logger = logging.getLogger(__name__)


@celery_app.task(name="process_arxiv_pdf")
def process_arxiv_pdf(version_id: str) -> None:
    db = SessionLocal()
    try:
        version = db.get(PaperVersion, UUID(version_id))
        if version is None:
            return

        version.status = PaperVersionStatus.processing
        db.commit()

        if not version.source_url:
            version.status = PaperVersionStatus.failed
            version.error_message = "Missing source URL"
            db.commit()
            return

        content = arxiv_service.download_pdf(version.source_url)
        key = f"papers/{version.paper_id}/{uuid.uuid4().hex}.pdf"
        upload_bytes(key, content)

        version.pdf_key = key
        version.sha256 = hashlib.sha256(content).hexdigest()
        version.size_bytes = len(content)
        version.status = PaperVersionStatus.ready
        version.error_message = None
        db.commit()
    except Exception as exc:  # noqa: BLE001
        logger.exception("Failed to process arXiv PDF for %s", version_id)
        db.rollback()
        version = db.get(PaperVersion, UUID(version_id))
        if version is not None:
            version.status = PaperVersionStatus.failed
            version.error_message = str(exc)[:1000]
            db.commit()
    finally:
        db.close()


@celery_app.task(name="finalize_uploaded_pdf")
def finalize_uploaded_pdf(version_id: str) -> None:
    db = SessionLocal()
    try:
        version = db.get(PaperVersion, UUID(version_id))
        if version is None or not version.pdf_key:
            return

        content = download_bytes(version.pdf_key)
        version.sha256 = hashlib.sha256(content).hexdigest()
        version.size_bytes = len(content)
        version.status = PaperVersionStatus.ready
        version.error_message = None
        db.commit()
    except Exception as exc:  # noqa: BLE001
        logger.exception("Failed to finalize upload %s", version_id)
        db.rollback()
        version = db.get(PaperVersion, UUID(version_id))
        if version is not None:
            version.status = PaperVersionStatus.failed
            version.error_message = str(exc)[:1000]
            db.commit()
    finally:
        db.close()
