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
  - query composer,
  - scope controls (`@all` / `@doc`),
  - SSE streaming render (`message` / `retrieval` / `delta` / `final` / `done`),
  - citation list rendering.
- System Configuration page:
  - provider list,
  - active provider switching.

## Architecture
- Entry: `frontend/src/App.jsx`.
- API client: `frontend/src/api.js` (contract-driven, bearer token support).
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
  - `all`: global owned-doc retrieval,
  - `doc`: explicit checkbox selection for owned docs.
- Mobile + desktop supported by responsive CSS rules.

## Quality Notes
- Frontend build/test commands exist via `vite` scripts.
- E2E automation is not yet added in this MVP commit.
