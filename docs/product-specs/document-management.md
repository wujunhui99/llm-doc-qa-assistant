# Document Management Spec

## Scope
- Upload document formats: TXT, Markdown, PDF.
- Document list, detail metadata, download, and delete operations.
- Persistent storage for raw file + parsed chunks + chunk vectors.

## Ownership Rules
- Every document has `owner_user_id`.
- Only owner can list/get/download/delete the document.
- Deleted document is removed from retrieval index and object storage.

## Functional Requirements (Implemented)
1. Upload:
- Validate file type and size (10MB max).
- Parse and chunk content (with Unicode normalization).
- Persist metadata and chunk index.
- Vectorize chunks and write vector index into Qdrant (when vector search enabled).
- Store raw file in MinIO bucket.
- Status lifecycle: `indexing -> ready` (or `failed` on parse error).

1. Retrieval-time repair:
- For historical PDF docs with unreadable stored chunks, system may re-parse source file and rebuild chunks/vectors on demand before answering.

2. List:
- Return only documents where `owner_user_id == current_user_id`.
- Pagination parameters supported (`page`, `page_size`).

3. Download:
- Owner can download uploaded file at any time.
- Download response uses attachment headers.

4. Delete:
- Requires explicit confirmation query `confirm=true`.
- Removes metadata, chunks, object storage file, and vector index entries.
- Writes audit event with actor and target doc id.

## Service Flow
- Frontend calls `api-go` `/api/documents/*`.
- `api-go` forwards request to `core-go-rpc`.
- `core-go-rpc` performs authz/ownership checks and interacts with MinIO + state store + Qdrant.

## API Contract
- `POST /api/documents/upload`
- `GET /api/documents`
- `GET /api/documents/{id}`
- `GET /api/documents/{id}/download`
- `DELETE /api/documents/{id}?confirm=true`

## Acceptance Criteria
- TXT/Markdown/PDF uploads succeed with clear status transitions.
- User never sees documents owned by other users.
- Uploaded files are downloadable by owner users.
- Deleted docs are not retrievable in subsequent QA turns.
