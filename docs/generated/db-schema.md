# DB Schema Snapshot (Go + Python Microservices)

## Current persistence
- MySQL: auth/session data (implemented)
- MinIO: raw document files (implemented)
- JSON state: document/chunk/thread/turn/provider (transitional)

## Runtime ownership
- `api-go`: no direct DB writes.
- `core-go-rpc`: writes MySQL auth/session and transitional state.
- `llm-python-rpc`: stateless answer service (no DB yet).

## Target persistence split
- Go domain DB (MySQL):
  - users
  - user_sessions
  - documents (target)
  - threads/turns metadata (target)
- Python retrieval stores (target):
  - vector index / embedding collections
  - retrieval job metadata

## MySQL tables currently implemented
### users
- `id VARCHAR(64) PRIMARY KEY`
- `email VARCHAR(255) UNIQUE NOT NULL`
- `password_hash VARCHAR(255) NOT NULL`
- `created_at DATETIME(6) NOT NULL`

### user_sessions
- `token VARCHAR(128) PRIMARY KEY`
- `user_id VARCHAR(64) NOT NULL` (FK -> `users.id`)
- `created_at DATETIME(6) NOT NULL`
- `expires_at DATETIME(6) NOT NULL`

## MinIO object storage
- default bucket: `qa-documents`
- object key: `{owner_user_id}/{doc_id}.{ext}`

## Cross-service identity fields (must be preserved)
- `owner_user_id`
- `thread_id`
- `turn_id`
- `scope_type`
- `scope_doc_ids`
