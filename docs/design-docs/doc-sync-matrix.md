# Documentation Sync Matrix

Use this matrix to decide mandatory documentation updates for each change set.

| Changed Area (examples) | Must Update | Minimum Required Update |
|---|---|---|
| Backend API contracts (`/api`, `/cmd`, `/internal/api`, handlers, DTOs) | `ARCHITECTURE.md`, `docs/product-specs/*` | Update request/response behavior and endpoint impact. |
| Internal RPC contracts (`/backend/proto/*`, gRPC methods/messages, service stubs) | `ARCHITECTURE.md`, `docs/design-docs/go-python-microservices.md`, `docs/product-specs/*` | Update inter-service request/response fields, ownership/scope fields, and compatibility notes. |
| Microservice topology/startup changes (`/backend/services/*`, service ports, service split/merge) | `ARCHITECTURE.md`, `README.md`, `docs/RELIABILITY.md`, `docs/SECURITY.md` | Update service responsibilities, startup/runbook, dependency graph, and failure boundaries. |
| Harness orchestration (`/internal/harness`, turn state machine, tool routing) | `ARCHITECTURE.md`, `docs/RELIABILITY.md`, `docs/QUALITY_SCORE.md` | Update turn lifecycle, failure handling, and quality gate impact. |
| Retrieval/ingestion (`parser`, `chunker`, `index`, `rerank`) | `ARCHITECTURE.md`, `docs/product-specs/*`, `docs/QUALITY_SCORE.md` | Update ingestion/retrieval flow and related accuracy metrics. |
| Frontend interaction changes (`/frontend/src`, pages/components) | `docs/FRONTEND.md`, `docs/DESIGN.md`, `docs/product-specs/*` | Update user flow, UI states, and acceptance behavior. |
| Provider/config changes (OpenAI/Claude/local, env vars, routing/fallback) | `ARCHITECTURE.md`, `docs/RELIABILITY.md`, `docs/SECURITY.md` | Update adapter policy, fallback rules, and secret/config handling. |
| CI/CD and deployment changes (`.github/workflows/*`, Dockerfiles, registry/tags, deploy scripts) | `docs/CICD.md`, `docs/SECURITY.md`, `docs/RELIABILITY.md` | Update triggers, images, deploy target/server settings, and health check behavior. |
| Database schema/migration changes | `docs/generated/db-schema.md`, `ARCHITECTURE.md` | Regenerate schema snapshot and update data model notes. |
| Security-sensitive logic (auth, permissions, upload validation, audit) | `docs/SECURITY.md`, `docs/RELIABILITY.md` | Update threat controls, validation rules, and incident impact. |
| Test strategy or eval changes (new benchmarks, threshold changes) | `docs/QUALITY_SCORE.md`, `docs/PLANS.md` | Update metrics definitions, thresholds, and rollout plan. |
| Scope/roadmap changes (feature added/removed/deferred) | `docs/PLANS.md`, `docs/exec-plans/*`, `docs/exec-plans/tech-debt-tracker.md` | Update scope decision, milestone, and debt record. |

## Rules
- If one change touches multiple areas, update the union of all mapped docs.
- If no existing product spec matches the change, create one under `docs/product-specs/` and add it to `docs/product-specs/index.md`.
- PR/commit is incomplete if code changed but required docs from this matrix did not change.
