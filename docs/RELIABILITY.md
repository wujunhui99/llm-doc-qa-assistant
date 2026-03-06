# RELIABILITY.md

## SLO Draft
- External API availability (`api-go`): 99.5%
- Turn success rate: >= 98%
- Auth API availability: 99.5%
- `core-go-rpc -> siliconflow` call success rate: >= 99%

## Reliability Controls (Implemented)
- Stable external contract is served by `api-go` and decoupled from LLM runtime changes.
- `core-go-rpc` enforces auth/ownership/scope before any LLM call.
- Timeouts are applied on:
  - HTTP -> Core gRPC calls,
  - Core -> SiliconFlow calls,
  - Core -> Qdrant calls.
- When LLM call fails, Core falls back to deterministic local answer template.
- When vector retrieval fails, Core falls back to lexical retrieval to keep turn flow available.
- Session expiration checks are enforced on every protected operation.
- Scope fallback:
  - no explicit scope => default `all`,
  - invalid `@doc` reference => deterministic error.

## Incident Readiness
- Audit log captures auth events, unauthorized access, and document deletion.
- Turn trace fields include `owner_user_id`, `thread_id`, `turn_id`.
- Cross-user leakage is treated as P0 incident.

## Remaining Reliability Work
- Add retry budgets and circuit-breaker policy for LLM HTTP client.
- Add canary release strategy for model-version updates.
- Add chaos/failure injection tests for inter-service dependencies.
