---
name: iteration-push-notes
description: >-
  Builds or refreshes ITERATION_PUSH.md with a detailed current-iteration handoff
  for Researcher so the next person can start from that file. Use before git push,
  when the user asks to update push notes / handoff for the iteration, when a push
  hook blocks because notes are stale, or when preparing develop / develop-aleksandr
  for GitHub (r-a).
---

# Iteration push notes

## Goal

Produce a single file **`ITERATION_PUSH.md`** at the repo root that is detailed enough for another developer (or agent) to continue the project without prior chat context.

## When

- Before `git push` (including when the push hook says notes are stale)
- User asks: «обнови заметки для пуша», «iteration notes», «что писать в handoff»

## Output file

Write/overwrite: `ITERATION_PUSH.md` (repo root — the directory that contains `docker-compose.yml` and `frontend/` or `r-a/frontend/`).

First lines **must** include:

```markdown
# Iteration push notes

Last refreshed: YYYY-MM-DD
Branch: <current branch>
```

`Last refreshed` date = today (local). The push hook checks this date.

## Workflow

1. Detect layout:
   - Monorepo in `r-a`: `frontend/`, `backend-go/`, …
   - Or parent `researcher/` with nested `r-a/frontend/`
2. Read existing: `HANDOFF.md`, `STATUS.md`, `iteration-1.md` (epic tables), `GO_MIGRATION.md` if present.
3. Verify against code (do not invent ✅):
   - Routes: `backend-go/internal/httpapi/` or `backend/app/routers/`
   - Compose: `docker-compose.yml` (which image is api/worker)
   - Frontend: reader PDF path, auth, library
4. Write `ITERATION_PUSH.md` using the template below (fill with real facts).
5. Tell the user to commit `ITERATION_PUSH.md` (and related docs) then `git push`.

## Template (fill in; keep headings)

```markdown
# Iteration push notes

Last refreshed: YYYY-MM-DD
Branch: …
Repo: Researcher-436345/r-a (or actual remote)

## One-liner
What the product is and what this push contains.

## How to run
Exact commands + ports (API, Vite, MinIO, Postgres, Redis).

## Done this iteration / currently working
Bullet list of working features with paths.

## Not done / known gaps
Epics or bugs still open.

## Architecture snapshot
Backend language, worker, DB, storage, PDF serving path, auth.

## Pitfalls
MinIO ports vs Cursor, LLM geo (RU), nested git if any.

## Suggested next tasks
3–7 concrete next steps (prefer P0/P1 from STATUS).

## API surface (if changed)
List endpoints touched or the full private API summary.

## Files to look at first
5–12 paths.
```

## Rules

- Prefer evidence from the repo over chat memory.
- Russian is OK (project language); keep structure scannable.
- Do not put secrets (API keys, `.env` values) in the file.
- Keep under ~250 lines; link to `HANDOFF.md` / `STATUS.md` for long history.
- After writing, remind: `git add ITERATION_PUSH.md && git commit && git push`.
