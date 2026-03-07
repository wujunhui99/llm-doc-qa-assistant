# RELIABILITY.md

## SLO Draft
- External API availability (`api-go`): 99.5%
- Turn success rate: >= 98%
- Auth API availability: 99.5%
- `core-go-rpc -> llm-python-rpc` RPC success rate: >= 99%

## Reliability Controls (Implemented)
- Stable external HTTP contract is isolated in `api-go`.
- `core-go-rpc` validates auth/ownership/scope before retrieval and model calls.
- Timeout boundaries:
  - HTTP -> Core gRPC
  - Core -> Python LLM gRPC
  - Core -> Qdrant
- Retrieval degradation:
  - vector retrieval preferred
  - lexical fallback when vector path fails/unavailable
- LLM provider failure behavior:
  - return explicit error to caller
  - no mock/local answer fallback
- Session expiration and invalid scope return deterministic errors.
- Historical unreadable PDF chunks are repaired on-demand before retrieval.

## Incident Readiness
- Audit log includes auth failures, unauthorized access, and destructive actions.
- Turn tracing fields include `owner_user_id`, `thread_id`, `turn_id`.
- Cross-user leakage is P0.

## Remaining Work
- Add retry/circuit-breaker around Python LLM RPC and provider HTTP.
- Add dependency chaos tests for MySQL/MinIO/Qdrant/LLM RPC.
- Add automated e2e regression for document upload -> retrieval -> cited answer.
