# Go + Python Microservices

## Current service split
- `api-go`:
  - Frontend HTTP boundary (`/api/*`).
  - Request/response shaping and auth header pass-through.
  - SSE bridge for streaming turns (`POST /api/threads/{thread_id}/turns/stream`).
  - Calls `core-go-rpc` only.
- `core-go-rpc`:
  - Domain rules: auth/session, ownership/scope, document/thread/turn state.
  - Document storage/index orchestration (MinIO + Qdrant).
  - Calls Python `LlmService` by gRPC for extraction, embeddings and answer generation (sync + stream).
- `agent-python-rpc`:
  - All model-facing logic and document text extraction.
  - `app/agent/llm`: chat-only provider interface + implementations (SiliconFlow + Ollama, reserved OpenAI/ChatGPT + Claude).
  - Ollama timeout uses dedicated env `OLLAMA_TIMEOUT_SECONDS` (default `15s`).
  - Embedding pipeline is fixed to SiliconFlow route (not provider-routed).
  - `app/agent/rag`: document extraction interface + implementations (PDF/Markdown/TXT).
  - `app/proto`: generated python gRPC stubs.
  - Context rerank for RAG answer generation.
  - gRPC service: `qa.v1.LlmService`.

## Contracts
- Source of truth: `backend/proto/qa/v1/qa.proto`.
- `api-go -> core-go-rpc`: `qa.v1.CoreService`.
- `core-go-rpc -> agent-python-rpc`: `qa.v1.LlmService`.
- Added RPC methods:
  - `ExtractDocumentText`
  - `EmbedTexts`
  - `GenerateAnswer`
  - `CreateTurnStream` (CoreService server-streaming, emits `TurnItem`)
  - `StreamGenerateAnswer` (LlmService server-streaming, emits `GenerateAnswerChunk`)
    - `GenerateAnswerChunk.delta`: answer token/chunk
    - `GenerateAnswerChunk.thinking_delta`: reserved field (current runtime disables reasoning mode)

## SSE mapping (`api-go`)
- `TurnItem.item_type=message` -> `event: message`
- `TurnItem.item_type=retrieval` -> `event: retrieval`
- `TurnItem.item_type=delta` -> `event: delta`
- `TurnItem.item_type=final` -> `event: final`

## Runtime defaults
- `api-go`: `:8080`
- `core-go-rpc`: `:19090`
- `agent-python-rpc`: `127.0.0.1:51000`

## Notes
- If `51000` conflicts in local env, override:
  - `AGENT_RPC_PORT` (or backward-compatible `LLM_RPC_PORT`) for python service
  - `AGENT_RPC_ADDR` (or backward-compatible `LLM_RPC_ADDR`) for core service
- Core remains policy enforcement point; Python service does not perform tenant authorization.
