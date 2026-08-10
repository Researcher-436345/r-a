# PDF Parser (Docling + PyMuPDF fallback)

FastAPI service: `POST /v1/parse` → markdown / plain_text / pages / chunks.

Prefer Docling when available; fall back to PyMuPDF for born-digital PDFs if Docling fails or `PARSER_ENGINE=pymupdf`.
