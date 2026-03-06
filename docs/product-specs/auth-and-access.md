# Auth and Access Spec

## Scope
- User registration and login.
- Session token lifecycle.
- Authorization for document and QA access.

## Authentication (Implemented)
- Register with unique email + password.
- Password stored as PBKDF2 hash (salted), never plaintext.
- Login returns bearer session token.
- Logout invalidates active session token.
- Session expiration enforced on protected endpoints.

## Authorization (Implemented)
- Protected routes require `Authorization: Bearer <token>`.
- Ownership enforcement on documents and threads.
- QA retrieval only scans chunks owned by current user.
- Provider configuration endpoint currently user-scoped (same auth gate).

## Audit and Security Events
- Record login success/failure, logout, document deletion, and config changes.
- Record unauthorized access attempts with resource path context.

## API Contract
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

## Acceptance Criteria
- Unauthenticated calls to protected routes are denied.
- Cross-user access to documents and retrieval context is denied.
- Session expiration behavior is deterministic.
