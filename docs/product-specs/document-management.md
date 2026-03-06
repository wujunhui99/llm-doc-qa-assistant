# Document Management Spec

## Scope
- Upload document formats: TXT, Markdown, PDF.
- Document list, detail metadata, and delete operations.
- Persistent storage for raw file + parsed chunks.

## Ownership Rules
- Every document has `owner_user_id`.
- Only owner can list/get/delete the document.
- Deleted document is removed from retrieval index and file storage.

## Functional Requirements (Implemented)
1. Upload:
- Validate file type and size (10MB max).
- Parse and chunk content.
- Persist metadata and chunk index.
- Status lifecycle: `indexing -> ready` (or `failed` on parse error).

2. List:
- Return only documents where `owner_user_id == current_user_id`.
- Pagination parameters supported (`page`, `page_size`).

3. Delete:
- Requires explicit confirmation query `confirm=true`.
- Removes metadata, chunks, and raw file.
- Writes audit event with actor and target doc id.

## API Contract
- `POST /api/documents/upload`
- `GET /api/documents`
- `GET /api/documents/{id}`
- `DELETE /api/documents/{id}?confirm=true`

## Acceptance Criteria
- TXT/Markdown/PDF uploads succeed with clear status transitions.
- User never sees documents owned by other users.
- Deleted docs are not retrievable in subsequent QA turns.
