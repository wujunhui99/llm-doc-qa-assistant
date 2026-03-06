# ARCHITECTURE.md

## 1. Goal
Build a Smart Document QA Assistant that supports:
- user registration/login,
- document management with TXT/Markdown/PDF,
- multi-turn QA with source citations,
- per-user document isolation,
- query scope controls (`@doc` and `@all`),
- provider switching,
with a harness-style turn model.

## 2. Implemented Top-Level Architecture
- Frontend (`frontend/`, React + Vite):
  - Auth page (register/login)
  - Document Management page
  - Agent QA page
  - System Configuration page
- Backend (`backend/`, Go stdlib HTTP):
  - API layer (`internal/api`)
  - Auth/KDF/session layer (`internal/auth`)
  - Ingestion parser/chunker (`internal/ingest`)
  - Scope resolver + retrieval/ranking (`internal/qa`)
  - State + audit persistence (`internal/store`)
- Storage:
  - Metadata: JSON state file (`backend/data/state.json`)
  - Raw files: filesystem (`backend/data/files/...`)
  - Retrieval index: chunk arrays per document in state
  - Audit log: JSONL (`backend/data/audit.log`)

## 3. Runtime Model
- Thread: conversation session (`threads` map).
- Turn: one QA request lifecycle (`turns` map).
- Item: turn event records (`turn_items` map), emitted as stream events by `/stream` endpoint.

Turn completion contract:
- scope resolved (`all` or `doc`),
- retrieval executed inside user ownership boundary,
- final answer generated,
- citations returned,
- status reaches terminal (`completed`).

## 4. Core Backend Modules
- `internal/auth/password.go`:
  - PBKDF2-HMAC-SHA256 password hashing.
  - Session token generation.
- `internal/ingest/parser.go`:
  - TXT/Markdown plain text parse.
  - Lightweight PDF text extraction fallback.
- `internal/ingest/chunker.go`:
  - fixed-size chunking with overlap.
- `internal/qa/scope.go`:
  - resolves explicit payload scope or message prefix (`@all`, `@doc(...)`).
- `internal/qa/retrieval.go`:
  - token-overlap ranking with CJK bigram support.
- `internal/store/store.go`:
  - user/session/document/thread/turn persistence.
  - audit event append.

## 5. API Contract (Implemented)
Base: `/api`

- Auth:
  - `POST /auth/register`
  - `POST /auth/login`
  - `POST /auth/logout`
  - `GET /auth/me`
- Documents:
  - `POST /documents/upload` (multipart `file`)
  - `GET /documents`
  - `GET /documents/{id}`
  - `DELETE /documents/{id}?confirm=true`
- QA:
  - `GET /threads`
  - `POST /threads`
  - `POST /threads/{thread_id}/turns`
  - `GET /threads/{thread_id}/turns/{turn_id}/stream` (SSE)
- Config:
  - `GET /config`
  - `PUT /config`
  - `GET /config/health`

## 6. Isolation and Safety Controls
- Protected APIs require bearer token.
- Ownership predicate enforced for document/thread reads and writes.
- Retrieval source docs always selected from `owner_user_id == current_user_id`.
- Upload validation:
  - type allowlist: TXT/Markdown/PDF,
  - size limit: 10MB.
- Delete API requires explicit confirmation query flag.
- Audit events recorded for:
  - login success/failure,
  - logout,
  - unauthorized access,
  - document deletion,
  - provider changes.

## 7. Testing Coverage (Current)
- Unit tests:
  - password hash/verify,
  - scope resolver,
  - retrieval ranking.
- API tests (httptest):
  - register/login flow,
  - cross-user document isolation.

## 8. Known Gaps vs Target Architecture
- Uses file-backed JSON state instead of SQL + vector DB (MVP simplification).
- Provider routing is config-level only (no real external provider invocation yet).
- PDF parser is lightweight and not full-fidelity for complex PDFs.
- No CI/CD workflow YAML generated yet.
