# ARCHITECTURE.md

## 1. Goal
Smart Document QA Assistant with:
- register/login/logout/me,
- TXT/Markdown/PDF document lifecycle,
- multi-turn QA with citations,
- user-level isolation,
- `@all` / `@doc` scope control,
- provider-configurable LLM runtime (default SiliconFlow).

## 2. Microservice topology

### `api-go` (HTTP gateway)
- External boundary for frontend.
- Routes `/api/*` to core gRPC.
- Default: `:8080`.

### `core-go-rpc` (domain service)
- Auth/session domain logic + ownership checks.
- Document metadata/chunk/turn/thread state management.
- MinIO file storage + Qdrant vector orchestration.
- Scope resolution and retrieval pipeline orchestration.
- Calls Python `LlmService` for:
  - embeddings (`EmbedTexts`)
  - answer generation (`GenerateAnswer`)
- Default: `:19090`.

### `llm-python-rpc` (model service)
- Model provider integration (SiliconFlow chat/embedding).
- Context rerank and prompt construction for answer generation.
- Default: `127.0.0.1:51000`.

## 3. Inter-service contracts
- Proto source: `backend/proto/qa/v1/qa.proto`
- `api-go -> core-go-rpc`: `qa.v1.CoreService`
- `core-go-rpc -> llm-python-rpc`: `qa.v1.LlmService`

## 4. Persistence
- MySQL: users + sessions.
- MinIO: raw uploaded documents.
- Qdrant: chunk vectors and payload metadata.
- JSON state (transitional): documents/chunks/threads/turns/provider config.

## 5. Turn runtime flow
1. Frontend calls `POST /api/threads/{thread_id}/turns`.
2. `api-go` forwards to `core-go-rpc.CreateTurn`.
3. Core authenticates and resolves scope.
4. Core retrieves chunks:
  - vector retrieval preferred (Qdrant + `EmbedTexts`)
  - lexical fallback when vector path unavailable/fails.
5. Core calls Python `GenerateAnswer`.
6. Core persists turn/items/citations and returns response.

## 6. Document ingestion flow
1. Upload validation (`.txt/.md/.markdown/.pdf`, size <= 10MB).
2. Parse and chunk text.
3. Store raw file in MinIO.
4. Persist metadata/chunks.
5. If vector enabled:
  - call Python `EmbedTexts`,
  - upsert vectors to Qdrant.
6. For unreadable historical PDF chunks, Core attempts on-demand reparse/rechunk/reindex before retrieval.

## 7. Security boundaries
- `api-go` is public-facing.
- `core-go-rpc` and `llm-python-rpc` are internal-only.
- Authorization and tenant boundaries are enforced in Core only.
- Qdrant retrieval always includes owner filter and scope doc filtering.

## 8. Reliability boundaries
- Timeouts:
  - HTTP -> Core gRPC,
  - Core -> Python LLM gRPC,
  - Core -> Qdrant.
- LLM provider failures return explicit service error (no mock fallback answer).
- Retrieval degrades to lexical mode when vector path fails.

## 9. Next migration
- Move document/thread/turn state from JSON state to relational schema.
