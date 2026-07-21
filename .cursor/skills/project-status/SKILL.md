---
name: project-status
description: >-
  Maintains docs/STATUS.md with what is done vs not done for the Researcher project
  against docs/iteration-1.md and docs/roadmap.md. Use when finishing a feature,
  starting a session, the user asks for project status, progress, готовность, что
  сделано, or asks to update the plan/notes/status doc. Also use on /loop or
  recurring check-ins that mention project status.
---

# Project status doc

## Goal

Keep [STATUS.md](../../../docs/STATUS.md) accurate after work lands. For major shifts (stack, ports, cutover), also refresh [HANDOFF.md](../../../docs/HANDOFF.md). Do not invent progress — verify against the codebase.

## When to run

- User asks «что сделано», «статус», «готовность», «update status»
- After completing an epic/task or merging a meaningful change
- Recurring `/loop` that tracks project progress

## Workflow

1. Read `docs/STATUS.md`, `docs/iteration-1.md` (EPIC tables), skim `docs/roadmap.md` if needed.
2. Verify claims with code:
   - Backend routes: `backend/internal/httpapi/`
   - Schema: `migrations/`
   - Frontend: `frontend/src/features/`, pages
   - Remaining mocks: sidebar projects, Similar tab, etc.
3. Update `docs/STATUS.md`:
   - Fix epic/stage ✅ / 🟡 / ❌
   - Add a short bullet under **Недавние изменения** (date + what)
   - Bump **Последнее обновление**
4. If newcomers need new start instructions: update `docs/HANDOFF.md`.
5. Keep both docs short — tables + changelog, no essay.
6. If the user asked to commit/push status notes: commit `docs/STATUS.md` / `docs/HANDOFF.md` only. Warn if `.git` is missing.

## Status legend

- ✅ done and usable end-to-end
- 🟡 partial (API without UI, blocked by env, mocks remain)
- ❌ not started

## Do not

- Mark EPIC-08 AI as ✅ while LLM is geo-blocked / key missing
- Mark EPIC-05/09/10 done without routes + UI
- Rewrite `docs/iteration-1.md` unless the user asks to change the plan itself
