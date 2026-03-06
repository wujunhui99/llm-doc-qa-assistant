# RELIABILITY.md

## SLO Draft
- QA API availability: 99.5%
- Turn success rate: >= 98%
- Auth API availability: 99.5%

## Current Reliability Controls
- Deterministic route-level validation and structured error codes.
- Session expiration checks on every authenticated request.
- Scope resolver fallback:
  - no explicit scope => default `all`,
  - invalid `@doc` reference => explicit scope error.
- Mandatory ownership filter in both API authorization and retrieval selection.
- Atomic state-file write pattern (`.tmp` then rename).
- SSE turn event stream endpoint for deterministic client consumption.

## Incident Readiness
- Audit log captures security-relevant actions and failures.
- QA turns persisted with status and scope metadata for traceability.
- Cross-user access attempts are explicitly logged as unauthorized events.

## Remaining reliability work
- Add retry/backoff policy for external provider adapters.
- Add health-aware provider failover logic.
- Add CI regression gate and uptime checks in workflow automation.
