# Notes from meeting (2026-08-19)

## 1) Price / Tariffs plan (to prepare)

### 1.1. Price list
- Price list for:
  - all our tools
  - the main service itself
  - `Web search`
  - `Deep research`

> Output: a single document/table that can be shown in the sales/pitch materials.

### 1.2. Tariffs development (backlog)
- Define Free / Pro (and any additional tiers if needed).
- Define limits for:
  - Web search calls
  - Deep research calls
  - other tools usage (if applicable)
- Decide billing units:
  - per request
  - per session
  - per token/credits (if the backend uses it)

## 2) Non-tech tasks

- “111 presentation”
  - Prepare slides deck (use the price list and tariff definitions as input).
  - Include: problem, solution, demo flow, differentiation, pricing & CTA.
- Search “Accel(s)” + submission
  - Identify relevant accelerators/incubators.
  - Prepare and submit applications.
  - Maintain a tracker: deadline / requirements / owner / status.

## 3) What to finish next (current follow-up)

### 3.1. Sync `web search` and `deep research` + UI visuals
- Make `web search` and `deep research` work coherently (same UX flow, same data model).
- Backend + frontend alignment (shared API contract and consistent states/loading/errors).
- Fix UI stretching:
  - remove layout stretching (stabilize heights/widths)
  - keep the Ask-box / results panel consistent across modes
- **Web search: tariff purchase button**
  - Implement a proper CTA in Web search for buying/upgrading a plan (not a stub).
  - Wire to tariff limits (Free vs Pro) and future payment flow.
  - Clear copy when quota is exhausted: what’s included, price, how to upgrade.

### 3.2. Remove mocks: Similar tab in chats
- In the reader/chat UI, the **Similar** tab still uses mock data (`readerSimilar` / hardcoded cards).
- Replace with real API or hide the tab until backend exists (EPIC-11 / discovery Stage 4).
- Same cleanup scope: any “similar papers” placeholders inside chat flows should not look live if they are fake.

### 3.3. Authorization / user sessions
- Ensure user sessions behave correctly across:
  - refresh token flow
  - login/logout
  - persistent state in UI (restore current mode/session context if needed)

### 3.4. Payment (backlog)
- Implement payments and connect them to:
  - тариф selection
  - plan limits
  - enforcing usage quotas
- Decide provider (and integration approach) before implementation.

## 4) How these tasks map to our iteration plan

### 4.1. Roadmap reference (what is “iteration 2”)
We don’t have an explicit “iteration-2.md” in docs. In `roadmap.md`, work is split by stages:
- Stage 2: Bibliography + search (BibTeX/RIS import/export, metadata editing, library search, filters, references extraction, dedup)
- Stage 3: Reading + AI (PDF reader, selections & notes, summary, explain, paper chat, links/refs)
- Stage 5 (Beta & scaling): mobile/adaptive UI, quotas, Free/Pro tariffs

### 4.2. Mapping your meeting items
- Price list + tariffs development + payment + adaptive devices
  - Fits mainly into **Stage 5 (Beta & scaling)**.
- Auth / user sessions
  - Fits earlier as a platform foundation (**Stage 1 / iteration 1 already contains auth**), but can be improved for production readiness (refresh/persistence/UX).
- Sync web search + deep research + visuals
  - Fits into **AI/discovery-related work after the initial reader+paper-chat foundation**, i.e. around **Stage 3** (and can spill into Stage 4 if it extends discovery/recommendations).
- Remove Similar mocks in chat
  - **EPIC-11** (front without mocks) + **Stage 4** (similar papers / discovery) when real recommendations exist.
- Web search tariff button + payment
  - **Stage 5** (Free/Pro, quotas); UI button can ship earlier as CTA once pricing table exists.

## 5) Suggested next actions (owners not assigned)

1. Produce the **pricing table** + confirm tiers and limits.
2. Decide the **API contract** for Web search / Deep research (backend/frontend sync).
3. Implement the UI fix to remove stretching (stabilize layout in the Ask-box/results panel).
4. **Similar tab:** remove mocks or hide until real similar-papers API (reader chat panel).
5. **Web search:** design and implement **Buy plan / Upgrade** button (quota-aware, links to tariffs).
6. Audit auth/session UX and confirm the session model is “production-ready”.
7. Choose payment provider and outline the quota enforcement approach.

