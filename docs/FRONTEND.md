# FRONTEND.md

## Scope (Implemented)
- Auth page (register/login toggle form).
- Document Management page:
  - upload TXT/Markdown/PDF,
  - list owned documents,
  - download owned documents,
  - delete with confirmation.
- Agent QA page:
  - thread creation/switching,
  - query composer (`@` trigger does not require leading whitespace),
  - inline `@` doc mention picker (inserts `@doc(...)` tokens),
  - automatic retrieval mode (`auto`) when no explicit `@doc` mention is provided,
  - SSE streaming render (`message` / `retrieval_decision` / `retrieval` / `delta` / `final` / `done`),
  - citation list rendering.
- System Configuration page:
  - provider list,
  - active provider switching.

## Architecture
- Entry: `frontend/src/App.jsx`.
- API client: `frontend/src/api.js` (contract-driven, bearer token support, default same-origin `/api` base).
- Pages:
  - `pages/AuthPage.jsx`
  - `pages/DocumentsPage.jsx`
  - `pages/QAPage.jsx`
  - `pages/SettingsPage.jsx`
- State model:
  - token in `localStorage` (`qa_token`),
  - user/profile loaded via `/api/auth/me`.

## Interaction Guarantees
- Access-aware rendering:
  - unauthenticated users only see auth page,
  - authenticated users get tabbed app shell.
- Document status and citation visibility shown by default.
- Users can click `Download` for owned documents; request uses bearer token and browser blob download.
- Scope UX:
  - no mention: send turn with `scope_type=auto` and let backend decide retrieval,
  - with `@doc(...)` mention: send `scope_type=doc` + selected `scope_doc_ids` (forced scoped retrieval).
- Streaming UX:
  - show retrieval-decision note per turn from `retrieval_decision` event payload (`use_retrieval`, `reason`, `retrieval_query`).
- Mobile + desktop supported by responsive CSS rules.
- Deployed frontend defaults to same-origin API calls (`/api/*`) instead of hardcoded `localhost`, so cross-device access works.

## Quality Notes
- Frontend build/test commands exist via `vite` scripts.
- E2E automation is not yet added in this MVP commit.
