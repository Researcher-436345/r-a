from fastapi import APIRouter
from sqlalchemy import text

from app.db import check_database, engine

router = APIRouter(tags=["health"])


def check_migrations() -> str:
    try:
        with engine.connect() as connection:
            row = connection.execute(text("SELECT version_num FROM alembic_version")).first()
        return "ok" if row else "empty"
    except Exception:
        return "error"


@router.get("/health")
def health() -> dict[str, str]:
    db_ok = check_database()
    migrations = check_migrations() if db_ok else "skipped"
    status = "ok" if db_ok and migrations == "ok" else "degraded"
    return {
        "status": status,
        "db": "ok" if db_ok else "error",
        "migrations": migrations,
    }
