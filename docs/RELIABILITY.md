# RELIABILITY.md

## SLO Draft
- External API availability (`api-go`): 99.5%
- Turn success rate: >= 98%
- Auth API availability: 99.5%
- `core-go-rpc -> llm-python-rpc` call success rate: >= 99%

## Reliability Controls (Implemented)
- Stable external contract is served by `api-go` and decoupled from LLM runtime changes.
- `core-go-rpc` enforces auth/ownership/scope before any LLM call.
- Timeouts are applied on:
  - HTTP -> Core gRPC calls,
  - Core -> LLM gRPC calls.
- When Python LLM call fails, Core falls back to deterministic local answer template.
- Session expiration checks are enforced on every protected operation.
- Scope fallback:
  - no explicit scope => default `all`,
  - invalid `@doc` reference => deterministic error.

## Incident Readiness
- Audit log captures auth events, unauthorized access, and document deletion.
- Turn trace fields include `owner_user_id`, `thread_id`, `turn_id`.
- Cross-user leakage is treated as P0 incident.

## Remaining Reliability Work
- Add retry budgets and circuit-breaker policy for LLM RPC client.
- Add canary release strategy for Python LLM versions.
- Add chaos/failure injection tests for inter-service dependencies.
