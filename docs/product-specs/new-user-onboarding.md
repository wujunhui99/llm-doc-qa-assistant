# New User Onboarding Spec

## User Goal
A first-time user can create an account, upload personal documents (TXT/Markdown/PDF), and get grounded answers quickly.

## Happy Path (Implemented)
1. User opens app and registers (or logs in).
2. User uploads TXT/Markdown/PDF in Document page.
3. System parses/chunks/indexes and marks document `ready`.
4. User creates QA thread and asks with `@all` (or default all scope).
5. System returns answer with citations from owned documents.
6. User asks follow-up turn with `@doc` scope and gets scoped continuity.

## API dependency map
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/documents/upload`
- `POST /api/threads`
- `POST /api/threads/{thread_id}/turns`

## Acceptance Criteria
- Register/login flow succeeds for valid input.
- Upload feedback is visible and non-blocking for TXT/Markdown/PDF.
- At least one citation is shown for evidence-backed responses.
- No data from other users appears in retrieval context or citations.
