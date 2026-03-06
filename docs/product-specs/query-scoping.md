# Query Scoping Spec (`@doc` and `@all`)

## Scope
Support user-controlled retrieval scope in QA input:
- `@doc`: target one or multiple specific documents.
- `@all`: target all documents owned by current user.

## Input Semantics (Implemented)
- Priority order:
  1. Explicit `scope_type` in turn payload.
  2. Message prefix parsing (`@all`, `@doc(...)`).
  3. Fallback default `all`.
- `@all`:
  - Retrieve across all documents where `owner_user_id == current_user_id`.
- `@doc`:
  - Support document id or document name references.
  - Resolve only within current user's document set.
- Invalid `@doc` references return scope error.

## Examples
- `@all 请总结我所有产品需求文档的风险点`
- `@doc(PRD-v2.md) 这个文档的上线范围是什么？`

## System Behavior
- Scope is resolved before retrieval starts.
- Resolved scope is stored on turn metadata (`scope_type`, `scope_doc_ids`).
- Retrieval is executed only against selected owned docs.

## Acceptance Criteria
- `@doc` responses only cite selected documents.
- `@all` responses can cite any owned document but never other users' documents.
- Invalid or unauthorized scope references return deterministic errors.
