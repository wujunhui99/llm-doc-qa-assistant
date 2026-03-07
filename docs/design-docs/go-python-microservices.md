# Go + Python Microservices

## Current service split
- `api-go`:
  - Frontend HTTP boundary (`/api/*`).
  - Request/response shaping and auth header pass-through.
  - Calls `core-go-rpc` only.
- `core-go-rpc`:
  - Domain rules: auth/session, ownership/scope, document/thread/turn state.
  - Document storage/index orchestration (MinIO + Qdrant).
  - Calls Python `LlmService` by gRPC for embeddings and answer generation.
- `llm-python-rpc`:
  - All model-facing logic (SiliconFlow chat + embeddings).
  - Context rerank for RAG answer generation.
  - gRPC service: `qa.v1.LlmService`.

## Contracts
- Source of truth: `backend/proto/qa/v1/qa.proto`.
- `api-go -> core-go-rpc`: `qa.v1.CoreService`.
- `core-go-rpc -> llm-python-rpc`: `qa.v1.LlmService`.
- Added RPC methods:
  - `EmbedTexts`
  - `GenerateAnswer`

## Runtime defaults
- `api-go`: `:8080`
- `core-go-rpc`: `:19090`
- `llm-python-rpc`: `127.0.0.1:51000`

## Notes
- If `51000` conflicts in local env, override:
  - `LLM_RPC_PORT` for python service
  - `LLM_RPC_ADDR` for core service
- Core remains policy enforcement point; Python service does not perform tenant authorization.
