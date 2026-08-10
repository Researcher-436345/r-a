from __future__ import annotations

import os
import re
import tempfile
from typing import Any, Literal

import fitz
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse

OCRMode = Literal["auto", "force", "off"]
EngineMode = Literal["auto", "docling", "pymupdf"]

app = FastAPI(title="researcher-parser", version="0.1.0")

ENGINE = os.getenv("PARSER_ENGINE", "auto").strip().lower() or "auto"
DEFAULT_OCR = os.getenv("PARSER_OCR", "auto").strip().lower() or "auto"
MAX_UPLOAD_BYTES = int(os.getenv("PARSER_MAX_UPLOAD_BYTES", str(51 << 20)))
CHARS_PER_TOKEN = 4
CHUNK_TARGET_TOKENS = 1000


def estimate_tokens(text: str) -> int:
    return max(1, (len(text) + CHARS_PER_TOKEN - 1) // CHARS_PER_TOKEN)


def chunk_pages(pages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    chunks: list[dict[str, Any]] = []
    buf: list[str] = []
    start_page = 1
    end_page = 1
    idx = 0

    def flush() -> None:
        nonlocal buf, start_page, end_page, idx
        text = "\n\n".join(x for x in buf if x.strip()).strip()
        if not text:
            buf = []
            return
        idx += 1
        chunks.append(
            {
                "id": f"c{idx:04d}",
                "page_start": start_page,
                "page_end": end_page,
                "section": f"pages {start_page}-{end_page}",
                "text": text,
                "token_estimate": estimate_tokens(text),
            }
        )
        buf = []

    for page in pages:
        page_no = int(page["page"])
        text = (page.get("text") or "").strip()
        if not text:
            continue
        if not buf:
            start_page = page_no
        provisional = "\n\n".join(buf + [text])
        if buf and estimate_tokens(provisional) > CHUNK_TARGET_TOKENS:
            flush()
            start_page = page_no
            buf = [text]
        else:
            buf.append(text)
        end_page = page_no
    flush()
    return chunks


def parse_pymupdf(path: str) -> dict[str, Any]:
    doc = fitz.open(path)
    try:
        pages: list[dict[str, Any]] = []
        plain_parts: list[str] = []
        for i, page in enumerate(doc, start=1):
            text = page.get_text("text") or ""
            text = re.sub(r"[ \t]+\n", "\n", text)
            text = re.sub(r"\n{3,}", "\n\n", text).strip()
            pages.append({"page": i, "text": text, "markdown": text})
            if text:
                plain_parts.append(text)
        plain = "\n\n".join(plain_parts).strip()
        return {
            "engine": "pymupdf",
            "ocr_used": False,
            "page_count": doc.page_count,
            "language": None,
            "title_hint": (doc.metadata or {}).get("title") or None,
            "markdown": plain,
            "plain_text": plain,
            "pages": pages,
            "chunks": chunk_pages(pages),
            "figures": [],
            "tables": [],
            "warnings": [],
        }
    finally:
        doc.close()


def text_density(result: dict[str, Any]) -> float:
    pages = max(1, int(result.get("page_count") or 1))
    return len(result.get("plain_text") or "") / pages


def parse_docling(path: str, ocr: OCRMode) -> dict[str, Any]:
    from docling.datamodel.base_models import InputFormat
    from docling.datamodel.pipeline_options import PdfPipelineOptions
    from docling.document_converter import DocumentConverter, PdfFormatOption

    pipeline = PdfPipelineOptions()
    # auto: let Docling decide; force/off set explicitly
    if ocr == "force":
        pipeline.do_ocr = True
    elif ocr == "off":
        pipeline.do_ocr = False

    converter = DocumentConverter(
        format_options={InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline)}
    )
    conv = converter.convert(path)
    document = conv.document
    markdown = document.export_to_markdown()
    plain = document.export_to_text() if hasattr(document, "export_to_text") else markdown

    pages: list[dict[str, Any]] = []
    # Best-effort page split from markdown form feeds
    try:
        for i, page in enumerate(getattr(document, "pages", []) or [], start=1):
            # Docling page objects vary by version; fall back to whole doc
            page_text = ""
            if hasattr(page, "export_to_text"):
                page_text = page.export_to_text() or ""
            pages.append({"page": i, "text": page_text, "markdown": page_text})
    except Exception:
        pages = []

    if not pages:
        # Split plain text roughly by form feed if present, else single page
        parts = re.split(r"\f+", plain) if plain else [markdown]
        pages = [
            {"page": i, "text": p.strip(), "markdown": p.strip()}
            for i, p in enumerate(parts, start=1)
            if p.strip()
        ]
        if not pages:
            pages = [{"page": 1, "text": plain or markdown, "markdown": markdown}]

    warnings: list[str] = []
    ocr_used = bool(getattr(pipeline, "do_ocr", False))
    return {
        "engine": "docling",
        "ocr_used": ocr_used,
        "page_count": len(pages),
        "language": None,
        "title_hint": None,
        "markdown": markdown or plain,
        "plain_text": plain or markdown,
        "pages": pages,
        "chunks": chunk_pages(pages),
        "figures": [],
        "tables": [],
        "warnings": warnings,
    }


def parse_pdf(path: str, engine: EngineMode, ocr: OCRMode) -> dict[str, Any]:
    warnings: list[str] = []
    preferred = engine
    if preferred == "auto":
        preferred = "docling"

    if preferred == "docling":
        try:
            result = parse_docling(path, ocr)
            # If auto OCR and density is very low, retry with OCR forced once
            if ocr == "auto" and text_density(result) < 200 and not result.get("ocr_used"):
                try:
                    ocr_result = parse_docling(path, "force")
                    if text_density(ocr_result) > text_density(result):
                        ocr_result["warnings"] = list(ocr_result.get("warnings") or []) + [
                            "ocr forced due to low text density"
                        ]
                        return ocr_result
                    result["warnings"] = list(result.get("warnings") or []) + [
                        "low text density; OCR retry did not improve yield"
                    ]
                except Exception as exc:  # noqa: BLE001
                    result["warnings"] = list(result.get("warnings") or []) + [
                        f"ocr retry failed: {exc}"
                    ]
            return result
        except Exception as exc:  # noqa: BLE001
            if engine == "docling":
                raise HTTPException(status_code=422, detail=f"docling parse failed: {exc}") from exc
            warnings.append(f"docling unavailable, fallback pymupdf: {exc}")

    result = parse_pymupdf(path)
    result["warnings"] = list(result.get("warnings") or []) + warnings
    if ocr == "force":
        result["warnings"].append("pymupdf engine ignores OCR force; install/use docling for OCR")
    return result


@app.get("/health")
def health() -> dict[str, Any]:
    return {"status": "ok", "engine": ENGINE, "version": "0.1.0"}


@app.post("/v1/parse")
async def parse(
    file: UploadFile = File(...),
    ocr: str = Form(DEFAULT_OCR),
    paper_id: str | None = Form(None),
    max_pages: int | None = Form(None),
) -> JSONResponse:
    _ = paper_id  # reserved for logs
    ocr_mode = (ocr or DEFAULT_OCR).strip().lower()
    if ocr_mode not in ("auto", "force", "off"):
        raise HTTPException(status_code=400, detail="ocr must be auto|force|off")

    raw = await file.read()
    if not raw:
        raise HTTPException(status_code=400, detail="empty file")
    if len(raw) > MAX_UPLOAD_BYTES:
        raise HTTPException(status_code=413, detail="PDF too large")
    if not (file.filename or "").lower().endswith(".pdf") and not raw.startswith(b"%PDF"):
        raise HTTPException(status_code=400, detail="expected a PDF file")

    with tempfile.NamedTemporaryFile(suffix=".pdf", delete=True) as tmp:
        tmp.write(raw)
        tmp.flush()
        try:
            result = parse_pdf(tmp.name, ENGINE if ENGINE in ("auto", "docling", "pymupdf") else "auto", ocr_mode)  # type: ignore[arg-type]
        except HTTPException:
            raise
        except Exception as exc:  # noqa: BLE001
            raise HTTPException(status_code=422, detail=f"parse failed: {exc}") from exc

    if max_pages and max_pages > 0:
        pages = result.get("pages") or []
        if len(pages) > max_pages:
            result["pages"] = pages[:max_pages]
            result["page_count"] = max_pages
            plain = "\n\n".join(p.get("text") or "" for p in result["pages"]).strip()
            result["plain_text"] = plain
            result["markdown"] = plain
            result["chunks"] = chunk_pages(result["pages"])
            result["warnings"] = list(result.get("warnings") or []) + [f"truncated to {max_pages} pages"]

    return JSONResponse(result)
