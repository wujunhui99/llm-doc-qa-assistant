# Query Scoping Spec (`auto` + `@doc`)

## Scope
Support user-controlled retrieval scope in QA input:
- `auto` (default): agent decides whether retrieval is needed.
- `@doc`: target one or multiple specific documents and force retrieval.

## Input Semantics (Implemented)
- Priority order:
  1. Explicit `scope_type` in turn payload.
  2. Message parsing (`@doc(...)`) when explicit `scope_type` not provided.
  3. Fallback default `auto`.
- `auto`:
  - Backend applies rule-based decision first.
  - If rule cannot decide, backend asks LLM for binary retrieval routing.
- `@doc`:
  - Support document id or document name references.
  - Resolve only within current user's document set.
- Invalid `@doc` references return scope error.

## Examples
- `请总结本周进展` (no mention, `auto` routing)
- `@doc(PRD-v2.md) 这个文档的上线范围是什么？`

## System Behavior
- Scope is resolved before retrieval starts.
- Resolved scope is stored on turn metadata (`scope_type`, `scope_doc_ids`).
- `@doc` always triggers retrieval against selected owned docs only.
- `auto` may skip retrieval for small-talk/non-doc turns.
- Retrieval prefers vector search (Qdrant) when enabled; lexical retrieval is fallback.
- Vector hits are filtered by owner and selected docs before answer generation.
- Streaming path emits a `retrieval_decision` event before `retrieval`:
  - payload fields: `mode`, `use_retrieval`, `reason`, `scope_type`, `scope_doc_ids`, `selected_doc_count`.

## Acceptance Criteria
- `@doc` responses only cite selected documents.
- `auto` mode can return answer with empty citations when decision is `use_retrieval=false`.
- If `retrieval_decision.use_retrieval=true`, citations are restricted to owned docs in resolved scope.
- Invalid or unauthorized scope references return deterministic errors.
- If model provider is unavailable, API returns explicit provider/config error instead of mock fallback text.
