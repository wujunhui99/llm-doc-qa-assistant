# SECURITY.md

## Baseline Requirements
- Passwords are never stored plaintext (PBKDF2 hash).
- Auth persistence uses MySQL (`users`, `user_sessions`).
- Document binaries persist in MinIO bucket.
- Vector payloads persist in Qdrant with owner and doc identifiers.
- Upload allowlist: `.txt`, `.md`, `.markdown`, `.pdf`.
- Ownership checks for document get/download/delete and thread/turn operations.
- Scope guardrails enforce current-user-only retrieval context.

## Microservice Security Controls (Implemented)
- Public exposure boundary:
  - Only `api-go` is frontend-facing.
  - `core-go-rpc` is internal-only.
- Policy enforcement point:
  - `core-go-rpc` authenticates session token and enforces ownership/scope.
- Vector isolation:
  - `core-go-rpc` applies owner filter for Qdrant search.
  - Returned vector hits are filtered by selected scope doc ids before answer generation.

## Audit Events
- Login success/failure.
- Logout.
- Unauthorized access attempts.
- Document deletion.
- Provider/config changes.

## Remaining Security Work
- Add outbound egress allowlist and request signing for model provider calls.
- Encrypt sensitive backups at rest.
- Add SAST/secret scanning and dependency checks in CI.
