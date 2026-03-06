# SECURITY.md

## Baseline Requirements (Implemented)
- Passwords are never stored plaintext.
  - Current implementation: PBKDF2-HMAC-SHA256 + random salt + high iteration count.
- Session-based authentication with bearer token.
- Protected API routes enforce authenticated user context.
- Upload validation:
  - allowlisted file extensions (`.txt`, `.md`, `.markdown`, `.pdf`),
  - max size 10MB.
- Ownership checks for document and thread resources.
- Retrieval isolation guard:
  - scope resolution only targets current user's documents.
- Confirmation gate for destructive delete action (`?confirm=true`).

## Audit Events (Implemented)
- Login success/failure.
- Logout.
- Unauthorized access attempts.
- Document deletion.
- Provider config changes.

Audit sink: `backend/data/audit.log` (JSONL).

## Data Handling
- Raw documents stored in local filesystem path under backend data directory.
- Metadata/chunks/sessions stored in JSON state file.
- No secret values are written to logs by the API layer.

## Remaining security work
- Replace file-backed state with encrypted-at-rest production database.
- Add rate limiting and lockout policy for auth endpoints.
- Add CSRF and stricter CORS origin policy for production deployment.
- Add secret scanning + SAST in CI once workflows are created.
