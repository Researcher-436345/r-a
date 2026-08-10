# PDF Parser + full-text chat

Last refreshed: 2026-08-10

## Model

1. **Parse pipeline** (worker `process_paper_parse`):
   1. If paper has `arxiv_id` → try **arXiv e-print TeX** (`export.arxiv.org/e-print/...`)
   2. If TeX missing / PDF-only / too short → **PDF parser** (`services/parser`, PyMuPDF default; Docling optional)
2. Store `paper_documents` + `paper_chunks` (`engine` = `arxiv_tex` or `pymupdf`/`docling`).
3. **Chat stuffing:** full paper text in LLM prompt (token budget).
4. **Rolling history summary** on chat overflow (`chat_thread_summaries`).

Not in this epic: AlphaXiv-style UI summary; embeddings/RAG; GROBID.

## API

- `GET http://parser:8091/health`
- `POST http://parser:8091/v1/parse` multipart `file` + `ocr=auto|force|off`

## Env

```
PARSER_SERVICE_URL=http://parser:8091
PARSER_ENGINE=auto
PARSER_OCR=auto
PARSER_TIMEOUT_SECONDS=180
LLM_CONTEXT_TOKENS=120000
LLM_REPLY_RESERVE=4000
CHAT_RECENT_KEEP=8
```

## Flow

```
PDF ready → enqueue process_paper_parse → parser → paper_documents.ready
chat → load plain_text + history → fit budget → LLM
```

## Ops

- First `docker compose build parser` is heavy (Docling models).
- CPU parse of 10–20 pages may take 30–120s; worker timeout 180s.
- If parse fails, PDF reader still works; chat falls back to title/abstract until document is ready.
