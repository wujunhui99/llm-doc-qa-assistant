# Go + Python Microservices Design

## Why this split
- Go handles stable external API and strict policy/tenant enforcement.
- Python handles fast-changing LLM/RAG answer generation logic.
- gRPC contracts isolate internal evolution from frontend API compatibility.

## Service boundaries (implemented)
- `api-go` (Go):
  - HTTP `/api/*` transport and response shaping.
  - Multipart upload/SSE handling.
  - Forwards business calls to Core gRPC.
- `core-go-rpc` (Go):
  - Auth/session + ownership rules.
  - Document parse/chunk/index state updates.
  - Thread/turn orchestration and citations.
  - Calls Python `LlmService.GenerateAnswer`.
- `llm-python-rpc` (Python):
  - Health check and answer generation from scoped contexts.

## Contract rules
- Source of truth proto: `backend/proto/qa/v1/qa.proto`.
- `api-go` only talks to `CoreService`; frontend never directly accesses core/llm RPC ports.
- `core-go-rpc` is authoritative for scope and tenant boundaries.
- `llm-python-rpc` consumes only scope-filtered chunks and returns answer text.

## Runtime defaults
- `api-go`: `:8080`
- `core-go-rpc`: `:19090`
- `llm-python-rpc`: `:19091`

## Security notes
- Optional `INTERNAL_SERVICE_TOKEN` is forwarded as gRPC metadata `x-service-token`.
- When token is configured, Python rejects unmatched internal callers.

## Rollout strategy
1. Keep external `/api/*` behavior stable.
2. Route all domain logic through `core-go-rpc`.
3. Keep Python answer service replaceable behind `LlmService` contract.
4. Migrate JSON state to relational schema without breaking frontend contract.
