# ARCHITECTURE.md

## 1. Goal
Build a Smart Document QA Assistant that supports:
- user registration/login,
- document management with TXT/Markdown/PDF,
- multi-turn QA with source citations,
- per-user document isolation,
- query scope controls (`@doc` and `@all`),
- provider switching,
with a Go + Python microservice architecture.

## 2. Microservice Topology

### Service 1: `api-go` (frontend HTTP gateway)
Responsibilities:
- Exposes stable external `/api/*` contract to frontend.
- Parses HTTP request/body/multipart/SSE.
- Performs lightweight auth header presence checks.
- Forwards business operations to `core-go-rpc` over gRPC.

Default runtime:
- Port: `:8080`
- Entry: `backend/services/api-go/cmd/api`

### Service 2: `core-go-rpc` (Go domain/rule service)
Responsibilities:
- Auth and session lifecycle (DDD auth module + MySQL repositories).
- Document metadata/chunk persistence and ownership checks.
- Document binary storage operations via MinIO.
- Thread/turn lifecycle, scope resolution, retrieval chunk selection.
- Calls Python LLM service for answer generation.

Default runtime:
- Port: `:19090`
- Entry: `backend/services/core-go-rpc/cmd/server`

### Service 3: `llm-python-rpc` (Python LLM service)
Responsibilities:
- Implements `LlmService` gRPC contract.
- Receives scope-constrained retrieval contexts from `core-go-rpc`.
- Generates grounded answer text from contexts + conversation history.

Default runtime:
- Port: `:19091`
- Entry: `backend/services/llm-python-rpc/app/server.py`

## 3. Inter-Service Contract

Proto source:
- `backend/proto/qa/v1/qa.proto`

Generated stubs:
- Go: `backend/proto/gen/go/qa/v1`
- Python: `backend/services/llm-python-rpc/app/generated/qa/v1`

Service contracts:
- `api-go -> core-go-rpc`: `qa.v1.CoreService`
- `core-go-rpc -> llm-python-rpc`: `qa.v1.LlmService`

Mandatory identity/scope fields carried into Python call:
- `owner_user_id`
- `thread_id`
- `turn_id`
- `scope_type`
- `scope_doc_ids`

## 4. Persistence Architecture
- MySQL:
  - `users`, `user_sessions` (implemented)
  - document/thread/turn relational migration (planned)
- MinIO:
  - raw document files, object key `{owner_user_id}/{doc_id}.{ext}`
- JSON local state (current transitional store):
  - documents/chunks/threads/turns/provider config

## 5. Runtime Turn Model (Cross-Service)
1. Frontend sends `POST /api/threads/{thread_id}/turns` to `api-go`.
2. `api-go` forwards request to `core-go-rpc` `CreateTurn`.
3. `core-go-rpc` authenticates token, validates ownership + scope.
4. `core-go-rpc` retrieves scoped chunks and calls Python `GenerateAnswer`.
5. `core-go-rpc` persists turn/items and returns citations + answer.
6. `api-go` returns turn JSON and can replay turn events via SSE stream endpoint.

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
- Inter-service auth header `x-service-token` is supported for `core-go-rpc -> llm-python-rpc`.
- Python service receives already-scoped contexts and must not broaden scope.

## 8. Reliability Controls
- Timeouts on HTTP->gRPC and Core->LLM gRPC calls.
- Core fallback answer path when LLM call fails.
- Audit log for auth failures, unauthorized access, and destructive actions.
- Stable external API maintained while internal services evolve.

## 9. Migration Status
- Target runtime is three-service topology under `backend/services/*`.
- Next migration step: move document/thread/turn state from JSON store to MySQL schema.
