# PLANS.md

## Current Focus
Go microservice split has been scaffolded and implemented with gRPC contracts:
- external API remains stable on `api-go` (`/api/*`),
- domain logic runs in `core-go-rpc`,
- answer generation is built into `core-go-rpc` (SiliconFlow + local fallback).
- vector retrieval path is enabled in `core-go-rpc` via SiliconFlow embeddings + Qdrant.
- unit-test-first workflow is required for backend changes (tests must be added/updated in same PR).

## Next Focus
- Move document/thread/turn persistence from JSON store to MySQL schema.
- Add resilient retry/circuit-breaker policy for Core->provider HTTP calls.
- Introduce dedicated reranker/plan-executor modules in Go LLM package.
- Add outbound provider security hardening (allowlist/signature policy) for production.
