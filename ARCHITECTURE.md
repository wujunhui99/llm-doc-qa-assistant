# ARCHITECTURE.md

## 1. Goal
Build a Smart Document QA Assistant that supports:
- user registration/login,
- document management with TXT/Markdown/PDF,
- multi-turn QA with source citations,
- per-user document isolation,
- query scope controls (`@doc` and `@all`),
- provider switching,
with a Go microservice architecture.

## 2. Microservice Topology

### Service 1: `api-go` (frontend HTTP gateway)
Responsibilities:
- Exposes stable external `/api/*` contract to frontend.
- Parses HTTP request/body/multipart/SSE.
- Performs lightweight auth header presence checks.
- Forwards business operations to `core-go-rpc` over gRPC.

Default runtime:
- Port: `:8080`
- Entry: `backend/apps/api-go/cmd/api`

### Service 2: `core-go-rpc` (Go domain/rule service)
Responsibilities:
- Auth and session lifecycle (DDD auth module + MySQL repositories).
- Document metadata/chunk persistence and ownership checks.
- Document binary storage operations via MinIO.
- Chunk vectorization + vector index write/read (Qdrant).
- Thread/turn lifecycle, scope resolution, retrieval chunk selection.
- Built-in LLM agent orchestration (SiliconFlow chat + fallback).

Default runtime:
- Port: `:19090`
- Entry: `backend/apps/core-go-rpc/cmd/server`

## 3. Inter-Service Contract

Proto source:
- `backend/proto/qa/v1/qa.proto`

Generated stubs:
- Go: `backend/proto/gen/go/qa/v1`

Service contracts:
- `api-go -> core-go-rpc`: `qa.v1.CoreService`

## 4. Persistence Architecture
- MySQL:
  - `users`, `user_sessions` (implemented)
  - document/thread/turn relational migration (planned)
- MinIO:
  - raw document files, object key `{owner_user_id}/{doc_id}.{ext}`
- Qdrant:
  - chunk vectors + payload metadata (`owner_user_id`, `doc_id`, `chunk_id`, `chunk_index`, `content`)
- JSON local state (transitional):
  - documents/chunks/threads/turns/provider config

## 5. Runtime Turn Model (Cross-Service)
1. Frontend sends `POST /api/threads/{thread_id}/turns` to `api-go`.
2. `api-go` forwards request to `core-go-rpc` `CreateTurn`.
3. `core-go-rpc` authenticates token, validates ownership + scope.
4. `core-go-rpc` embeds question, runs vector retrieval in Qdrant, and falls back to lexical retrieval when vector path fails.
5. `core-go-rpc` runs built-in agent answer generation (SiliconFlow chat + deterministic fallback).
6. `core-go-rpc` persists turn/items and returns citations + answer.
7. `api-go` returns turn JSON and can replay turn events via SSE stream endpoint.

## 6. External API Contract (Frontend-facing)
Base: `/api`

- Auth:
  - `POST /auth/register`
  - `POST /auth/login`
  - `POST /auth/logout`
  - `GET /auth/me`
- Documents:
  - `POST /documents/upload`
  - `GET /documents`
  - `GET /documents/{id}`
  - `GET /documents/{id}/download`
  - `DELETE /documents/{id}?confirm=true`
- QA:
  - `GET /threads`
  - `POST /threads`
  - `POST /threads/{thread_id}/turns`
  - `GET /threads/{thread_id}/turns/{turn_id}/stream`
- Config:
  - `GET /config`
  - `PUT /config`
  - `GET /config/health`

## 7. Security and Isolation Controls
- `core-go-rpc` is policy enforcement point for tenant/ownership checks.
- Upload allowlist is enforced server-side: `.txt`, `.md`, `.markdown`, `.pdf`.
- Session token validation performed in `core-go-rpc` for all protected operations.
- Vector retrieval enforces owner filter + scope doc filtering before generation.

## 8. Reliability Controls
- Timeouts on HTTP->gRPC, Core->SiliconFlow calls, and Core->Qdrant calls.
- Core fallback answer path when LLM call fails.
- Audit log for auth failures, unauthorized access, and destructive actions.
- Stable external API maintained while internal services evolve.

## 9. Migration Status
- Target runtime is two-service topology under `backend/apps/*`.
- Next migration step: move document/thread/turn state from JSON store to MySQL schema.
