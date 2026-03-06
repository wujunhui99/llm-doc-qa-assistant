# LLM Document QA Assistant

MVP implementation based on `AGENTS.md` + architecture/spec docs.

## Stack
- Frontend: React (Vite)
- Backend: Go (stdlib HTTP server)

## Features implemented
- Register/Login/Logout/Me with session token.
- Password hashing with PBKDF2-HMAC-SHA256.
- Document upload/list/get/delete (TXT/Markdown/PDF).
- Parsing + chunking + retrieval index persistence.
- Per-user document isolation enforced in APIs and retrieval.
- QA threads/turns with `@all` + `@doc(...)` scope resolver.
- Grounded answer response with citations.
- Provider configuration API (`mock/openai/claude/local`) + health endpoint.
- Audit log for auth failures, logout, delete, provider changes.

## Backend run
```bash
cd backend
GOCACHE=/tmp/go-build go test ./...
go run ./cmd/server
```

The backend starts at `http://localhost:8080`.

## Frontend run
```bash
cd frontend
npm install
npm run dev
```

Frontend default API target is `http://localhost:8080`.
Override with `VITE_API_BASE_URL`.

## Core API routes
- Auth: `/api/auth/register`, `/api/auth/login`, `/api/auth/logout`, `/api/auth/me`
- Documents: `/api/documents/upload`, `/api/documents`, `/api/documents/{id}`
- QA: `/api/threads`, `/api/threads/{thread_id}/turns`, `/api/threads/{thread_id}/turns/{turn_id}/stream`
- Config: `/api/config`, `/api/config/health`
