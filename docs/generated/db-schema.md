# DB Schema Snapshot (Current MVP File-Backed State)

Current storage implementation uses a JSON state file (`backend/data/state.json`) plus filesystem artifacts.

## State root object
- `users: map[user_id]User`
- `sessions: map[token]Session`
- `documents: map[doc_id]Document`
- `chunks: map[doc_id][]Chunk`
- `threads: map[thread_id]Thread`
- `turns: map[turn_id]Turn`
- `turn_items: map[turn_id][]TurnItem`
- `provider: ProviderConfig`
- `email_to_user: map[email]user_id`

## Model fields
### User
- `id`
- `email`
- `password_hash`
- `created_at`

### Session
- `token`
- `user_id`
- `created_at`
- `expires_at`

### Document
- `id`
- `owner_user_id`
- `name`
- `size_bytes`
- `mime_type`
- `storage_path`
- `status` (`indexing|ready|failed`)
- `chunk_count`
- `created_at`
- `last_updated_at`

### Chunk
- `id`
- `doc_id`
- `index`
- `content`

### Thread
- `id`
- `owner_user_id`
- `title`
- `created_at`

### Turn
- `id`
- `thread_id`
- `owner_user_id`
- `question`
- `answer`
- `status`
- `scope_type` (`all|doc`)
- `scope_doc_ids` (array)
- `created_at`
- `updated_at`

### TurnItem
- `id`
- `turn_id`
- `item_type` (`message|retrieval|final`)
- `payload` (JSON object)
- `created_at`

## Ownership isolation keys
- `documents.owner_user_id`
- `threads.owner_user_id`
- `turns.owner_user_id`

## Raw artifact storage
- Uploaded files: `backend/data/files/{owner_user_id}/{doc_id}.{ext}`
- Audit log: `backend/data/audit.log` (JSONL)
