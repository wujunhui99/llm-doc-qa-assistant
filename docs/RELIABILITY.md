# RELIABILITY.md

## SLO Draft
- External API availability (`api-go`): 99.5%
- Turn success rate: >= 98%
- Auth API availability: 99.5%
- `core-go-rpc -> agent-python-rpc` RPC success rate: >= 99%

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

## CI/CD Reliability Controls (Implemented)
- `ci.yml` is configured as merge gate candidate for `main` with three required jobs: `doc-sync`, `backend-go`, `frontend-react`.
- `ci.yml` has workflow-level concurrency with `cancel-in-progress: true` to reduce stale parallel runs.
- `docker-build.yml` builds and publishes four images (api-go/core-go-rpc/agent-python-rpc/frontend) with deterministic tags (`sha-<short>`, plus `main` on default branch).
- Docker build cache (`type=gha`) is enabled to reduce rebuild latency and timeout risk.
- `deploy.yml` runs only on successful Docker build for `main` (or manual dispatch), with deploy concurrency control enabled.
- Deploy workflow validates repository-side `deploy/compose/docker-compose.yml` before SSH deploy, preventing image-name/tag drift.
- Deploy uses mandatory post-deploy health checks for `http://localhost:8080` and `http://localhost:8080/api/config`; failed checks fail the deployment.
- Deploy workflow app root is standardized to `/home/ubuntu/code/project/llm-doc-qa-assistant` to keep rollout target aligned with this repository.

## Remaining Work
- Add retry/circuit-breaker around Python LLM RPC and provider HTTP.
- Add dependency chaos tests for MySQL/MinIO/Qdrant/LLM RPC.
- Add automated e2e regression for document upload -> retrieval -> cited answer.
