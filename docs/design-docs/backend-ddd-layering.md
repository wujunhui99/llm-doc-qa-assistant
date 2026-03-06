# Backend DDD Layering

## Intent
Refactor backend to keep domain/business rules isolated from transport and infrastructure details, while supporting Go service boundaries.

## Layer responsibilities (Go core service)
- `domain`:
  - entities, value normalization rules, repository contracts, domain-level errors.
- `application`:
  - use-case orchestration and transaction flow.
  - no direct HTTP or SQL code.
- `infrastructure`:
  - concrete adapters (MySQL, MinIO, crypto/token providers, in-memory test doubles).
- `transport`:
  - `api-go`: HTTP mapping only.
  - `core-go-rpc`: gRPC mapping and application/domain orchestration.

## Current bounded context coverage
- Auth context is fully routed through DDD layers:
  - Domain: `internal/domain/auth`
  - Application: `internal/application/auth`
  - Infrastructure: `internal/infrastructure/mysql`, `internal/infrastructure/security`
- Document/QA context currently uses transitional store + infrastructure adapters and is invoked from `core-go-rpc`.

## Service placement
- `backend/apps/api-go`: thin HTTP gateway (no domain persistence logic).
- `backend/apps/core-go-rpc`: domain rule service and policy enforcement point.
- `backend/apps/core-go-rpc/internal/llm`: in-process answer-generation adapter layer.

## Next migration targets
- Move document/thread/turn aggregates and repositories from file-state store into relational schema.
- Introduce dedicated application services for document and conversation contexts.
