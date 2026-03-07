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
  - `core-go-rpc` and `agent-python-rpc` are internal-only.
- Policy enforcement point:
  - `core-go-rpc` authenticates session token and enforces ownership/scope.
- LLM boundary:
  - `agent-python-rpc` does not perform tenant authorization.
  - Core sends only scoped/authorized context to Python service.
- Vector isolation:
  - `core-go-rpc` applies owner filter for Qdrant search.
  - Returned vector hits are filtered by selected scope doc ids before answer generation.

## Audit Events
- Login success/failure.
- Logout.
- Unauthorized access attempts.
- Document deletion.
- Provider/config changes.

## CI/CD Security Controls (Implemented)
- GitHub Actions deploy path uses repository variables/secrets only (`DEPLOY_HOST`, `DEPLOY_USER`, optional `DEPLOY_PORT`, `GHCR_USERNAME`, `DEPLOY_SSH_KEY`, `GHCR_TOKEN`).
- Deploy workflow authenticates to the target host using SSH private key from secret (`DEPLOY_SSH_KEY`), with host key pinning via `ssh-keyscan`.
- Container registry authentication uses `GHCR_TOKEN` at runtime; no secret is hardcoded in workflow YAML.
- Workflow permissions are minimal-by-default (`contents: read`, plus `packages: write` only for image publish workflow).
- Manual deploy image tag input affects image selection only; authentication still relies on GHCR token and SSH key secrets.
- Post-deploy health checks are mandatory and deployment fails on health check timeout/failure.
- Deploy workflow validates repository-side compose artifacts before remote execution, reducing mis-deploy risk from stale or mismatched image references.
- Deploy target path baseline is `/home/ubuntu/code/project/llm-doc-qa-assistant` (`$APP_DIR`), avoiding cross-project path reuse.

## Remaining Security Work
- Add outbound egress allowlist for `agent-python-rpc` provider calls.
- Encrypt sensitive backups at rest.
- Add SAST/secret scanning and dependency checks in CI.
