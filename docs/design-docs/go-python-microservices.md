# Go Runtime Architecture (Merged LLM)

## Background
This document originally described the Go + Python split.
As of 2026-03-06, answer generation has been merged into `core-go-rpc`.

## Service boundaries (current)
- `api-go` (Go):
  - HTTP `/api/*` transport and response shaping.
  - Multipart upload/SSE handling.
  - Forwards business calls to Core gRPC.
- `core-go-rpc` (Go):
  - Auth/session + ownership rules.
  - Document parse/chunk/index state updates.
  - Thread/turn orchestration and citations.
  - Built-in LLM agent generation (SiliconFlow chat) and fallback logic.
  - Vector retrieval with Qdrant + lexical fallback.

## Contract rules
- Source of truth proto: `backend/proto/qa/v1/qa.proto`.
- `api-go` only talks to `CoreService`; frontend never directly accesses core RPC port.
- `core-go-rpc` is authoritative for scope, tenant boundaries, and answer generation.

## Runtime defaults
- `api-go`: `:8080`
- `core-go-rpc`: `:19090`

## Security notes
- `core-go-rpc` enforces owner isolation on retrieval scope and Qdrant owner filter.
- `core-go-rpc` does not expose internal LLM calls over public endpoints.

## Rollout strategy
1. Keep external `/api/*` behavior stable.
2. Route all domain logic through `core-go-rpc`.
3. Keep model-provider adapter replaceable behind Go `llm.Generator` interface.
4. Migrate JSON state to relational schema without breaking frontend contract.
