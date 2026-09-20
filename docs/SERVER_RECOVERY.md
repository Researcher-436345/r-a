# ru-vmmedium source recovery

The `codex/recover-ru-vmmedium` branch preserves a source snapshot copied on
2026-09-20 from `/home/e.drobyazko/r-a` on `ru-vmmedium`. That directory had no
`.git`, and the landing/summary source files were absent from every available
origin branch. The recovered `docs/ITERATION_PUSH.md` names
`feature/paper-summary`, but that branch was not available on origin; the
original commit and branch cannot be verified from Git.

The recovery branch starts at `e4d7a08` solely to allow a three-way merge with the
subsequent guest-access change `6a29dde`; this is a reconstruction, not the
server's original Git history.

The snapshot includes the landing page, paper summaries, URL/full-text discovery,
PDF processing improvements, associated tests and production container setup.
Environment files, credentials, user data, dependencies and generated build files
were excluded. Existing repository files absent from the source archive were
preserved.

Source archive SHA-256:
`0309aaf0a88bdee6cdbcc2b1d63669b40f3b386ed78d67c370e369370b84595a`

Running images observed when recovering the source:

- Frontend: `sha256:ce7e47fc5e6b11b5319074bbc4ad6f4f9dbda322af01c6031e7f87dd049e41c1`
- Gateway: `sha256:16476eb7b92d378a3475c41ab5dc8056f812d97dbb65554aff56b26b6bf3bdef`

Both images were built on 2026-09-15. Image timestamps do not establish a Git
revision. Recovery and merge do not deploy changes to the server.

## Merge into main

The merge retains guest browsing from `6a29dde`. Landing is controlled by
`VITE_ENABLE_LANDING=false`; active sessions remain off by default. Both flags
are forwarded through the production frontend Docker build. AI overviews require
sign-in on explicit tab selection; guest PDF loading never requests personal
notes, chat history or summaries. Translation remains public.

Validation on 2026-09-20:

- Frontend production build passed.
- All Go packages passed `go test ./...`; catalog/gateway tests were rerun after
  adding summary/full-text access checks and per-guest resolve limiting.
- Websearch: 119 pytest tests passed.
- Production Compose configuration validated.
- Migration 008 applied successfully to the local development database.
- Real local API: feed, paper and PDF metadata returned 200 for guests; summary,
  full-text discovery and library returned 401.
- Browser: public PDF reading, summary sign-in redirect, default landing redirect,
  enabled landing and sessions flags all verified. Summary streaming and six-card
  rendering were verified with the in-memory fixture, without a live LLM call.
