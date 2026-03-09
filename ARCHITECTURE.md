# ARCHITECTURE.md

## 1. Goal
Smart Document QA Assistant with:
- register/login/logout/me,
- TXT/Markdown/PDF document lifecycle,
- multi-turn QA with citations,
- user-level isolation,
- `auto` / `@doc` scope control,
- provider-configurable LLM runtime (default SiliconFlow).

## 2. Microservice topology

### `api-go` (HTTP gateway)
- External boundary for frontend.
- Routes `/api/*` to core gRPC.
- Supports synchronous turn creation and SSE streaming turn creation.
- Default: `:8080`.

### `core-go-rpc` (domain service)
- Auth/session domain logic + ownership checks.
- Document metadata/chunk/turn/thread state management.
- MinIO file storage + Qdrant vector orchestration.
- Scope resolution and retrieval pipeline orchestration.
- Calls Python `LlmService` for:
  - embeddings (`EmbedTexts`)
  - answer generation (`GenerateAnswer` / `StreamGenerateAnswer`)
- Default: `:19090`.

### `agent-python-rpc` (model+rag service)
- Chat provider integration via `BaseChatClient` + LiteLLM adapters.
  - Current: SiliconFlow (OpenAI-compatible), OpenAI/ChatGPT, Claude (Anthropic), Ollama
- Ollama chat timeout is configured independently (`OLLAMA_TIMEOUT_SECONDS`, default `15s`).
- Embedding pipeline is fixed to SiliconFlow (not provider-routed).
- Context rerank and prompt construction for answer generation.
- Agent routing tools (`agent/tools/*`):
  - retrieval tool schema (`retrieval(query, reason)`),
  - route-mode parsing for function-calling outputs (tool calls or JSON fallback).
- Document text extraction/parsing (TXT/Markdown/PDF).
- Default: `127.0.0.1:51000`.

## 3. Inter-service contracts
- Proto source: `backend/proto/qa/v1/qa.proto`
- `api-go -> core-go-rpc`: `qa.v1.CoreService`
- `core-go-rpc -> agent-python-rpc`: `qa.v1.LlmService`

## 4. Persistence
- MySQL: users + sessions.
- MinIO: raw uploaded documents.
- Qdrant: chunk vectors and payload metadata.
- JSON state (transitional): documents/chunks/threads/turns/provider config.

## 5. Turn runtime flow
### 5.1 Non-streaming
1. Frontend loads thread history via `GET /api/threads/{thread_id}/turns` when entering/switching session.
2. Frontend creates new turn with `POST /api/threads/{thread_id}/turns`.
3. `api-go` forwards to `core-go-rpc.CreateTurn`.
4. Core authenticates and resolves scope.
5. Core decides retrieval path:
  - `@doc`: force retrieval,
  - `auto`: rule-based gate first, LLM route mode fallback (`scope_type=route`).
6. If retrieval enabled, core retrieves chunks using route query keywords:
  - vector retrieval preferred (Qdrant + `EmbedTexts`)
  - lexical fallback when vector path unavailable/fails.
7. Core calls Python `GenerateAnswer`.
8. Core persists turn/items/citations and returns response.

### 5.2 Streaming (SSE)
1. Frontend calls `POST /api/threads/{thread_id}/turns/stream` with `Accept: text/event-stream`.
2. `api-go` opens `core-go-rpc.CreateTurnStream` and forwards stream items as SSE events.
3. Core authenticates, resolves scope, emits retrieval decision, conditionally retrieves chunks, then calls Python `StreamGenerateAnswer`.
4. Python streams answer deltas from provider through LiteLLM streaming adapters.
5. Core emits item events in order:
  - `message`
  - `retrieval_decision`
  - `retrieval`
  - `delta` (0..N)
  - `final`
  - `done` (emitted by API gateway as stream terminator)
6. Core persists final turn + streamed items after generation completes.

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
- `core-go-rpc` and `agent-python-rpc` are internal-only.
- Authorization and tenant boundaries are enforced in Core only.
- Qdrant retrieval always includes owner filter and scope doc filtering.

## 8. Reliability boundaries
- Timeouts:
  - HTTP -> Core gRPC,
  - Core -> Python LLM gRPC,
  - Core -> Qdrant.
- Ollama path uses timeout+single-retry policy (base timeout default `15s`) to avoid long blocked requests.
- Streaming turn path reduces long blocked HTTP waits by forwarding provider deltas as they arrive.
- LLM provider failures return explicit service error (no mock fallback answer).
- Retrieval degrades to lexical mode when vector path fails.

## 9. Next migration
- Move document/thread/turn state from JSON state to relational schema.
