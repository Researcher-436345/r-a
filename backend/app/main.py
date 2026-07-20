from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.core.config import settings
from app.db import check_database
from app.routers import annotations, auth, feed, health, library, papers
from app.services.storage import ensure_bucket

app = FastAPI(title=settings.app_name)

app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:5173",
        "http://127.0.0.1:5173",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(health.router)
app.include_router(auth.router)
app.include_router(papers.router)
app.include_router(library.router)
app.include_router(annotations.router)
app.include_router(feed.router)


@app.on_event("startup")
def on_startup() -> None:
    try:
        ensure_bucket()
    except Exception:
        # MinIO may still be starting; uploads will retry ensure_bucket.
        pass


@app.get("/")
def root() -> dict[str, str]:
    return {
        "service": settings.app_name,
        "docs": "/docs",
        "health": "/health",
        "db_reachable": str(check_database()).lower(),
    }
