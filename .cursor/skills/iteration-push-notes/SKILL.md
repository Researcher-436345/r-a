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

Produce a single file **`docs/ITERATION_PUSH.md`** that is detailed enough for another developer (or agent) to continue the project without prior chat context.

## When

- Before `git push` (including when the push hook says notes are stale)
- User asks: «обнови заметки для пуша», «iteration notes», «что писать в handoff»

## Output file

Write/overwrite: `docs/ITERATION_PUSH.md` (create `docs/` if needed).

First lines **must** include:

```markdown
# Iteration push notes

Last refreshed: YYYY-MM-DD
Branch: <current branch>
```

`Last refreshed` date = today (local). The push hook checks this date.

## Workflow

1. Detect layout: `backend/` (Go), `frontend/`, `docs/`.
2. Read existing: `docs/HANDOFF.md`, `docs/STATUS.md`, `docs/iteration-1.md` if present.
3. Verify against code (do not invent ✅):
   - Routes: `backend/internal/httpapi/`
   - Compose: `docker-compose.yml`
   - Frontend: `frontend/src/...`
4. Write `docs/ITERATION_PUSH.md` using the template below.
5. Tell the user to commit and push.

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
- Keep under ~250 lines; link to `docs/HANDOFF.md` / `docs/STATUS.md` for long history.
- After writing, remind: `git add docs/ITERATION_PUSH.md && git commit && git push`.
