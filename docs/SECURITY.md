# SECURITY.md

## Baseline Requirements
- Passwords are never stored plaintext (PBKDF2 hash).
- Auth persistence uses MySQL (`users`, `user_sessions`).
- Document binaries persist in MinIO bucket.
- Upload allowlist: `.txt`, `.md`, `.markdown`, `.pdf`.
- Ownership checks for document get/download/delete and thread/turn operations.
- Scope guardrails enforce current-user-only retrieval context.

## Microservice Security Controls (Implemented)
- Public exposure boundary:
  - Only `api-go` is frontend-facing.
  - `core-go-rpc` and `llm-python-rpc` are internal services.
- Policy enforcement point:
  - `core-go-rpc` authenticates session token and enforces ownership/scope.
- Internal service auth:
  - `core-go-rpc` can send `x-service-token` metadata to Python.
  - `llm-python-rpc` validates token when `INTERNAL_SERVICE_TOKEN` is configured.
- Python receives trusted identity context fields only from core service:
  - `owner_user_id`, `scope_type`, `scope_doc_ids`, `thread_id`, `turn_id`.

## Audit Events
- Login success/failure.
- Logout.
- Unauthorized access attempts.
- Document deletion.
- Provider/config changes.
- Internal service auth failures.

## Remaining Security Work
- Enforce mTLS or signed service tokens for Go->Python traffic in production.
- Add request signing and replay protection for internal calls.
- Encrypt sensitive backups at rest.
- Add SAST/secret scanning and dependency checks in CI.
