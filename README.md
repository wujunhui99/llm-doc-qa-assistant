# LLM Document QA Assistant

Frontend + Go/Python microservices for document QA (RAG + multi-turn agent QA).

## Online Access
- Project URL: [http://43.128.109.136:5173/](http://43.128.109.136:5173/)

## Stack
- Frontend: React (Vite)
- Backend:
  - `api-go` (`:8080`)
  - `core-go-rpc` (`:19090`)
  - `agent-python-rpc` (`127.0.0.1:51000`)
- Middleware:
  - MySQL
  - MinIO
  - Qdrant

## Backend layout
- `backend/apps/api-go`
- `backend/apps/core-go-rpc`
- `backend/apps/agent-python-rpc`
- `backend/proto/qa/v1/qa.proto`

`agent-python-rpc` internals:
- `app/agent/llm`: chat provider interface + LiteLLM-based adapters
- `app/agent/rag`: document extraction interface + implementations
- `app/proto`: generated python gRPC stubs

## Start middleware
```bash
docker compose up -d
```

## Run with Docker Compose (GHCR images)
Use `deploy/compose` when you want to run the project from published images.

```bash
cd deploy/compose
cp .env.example .env
```

Edit `.env` at least:
- `GHCR_USERNAME=<your-github-username-or-org>`
- `IMAGE_TAG=main` (or `sha-<short>` from CI)

Login and run:
```bash
echo "<ghcr_token>" | docker login ghcr.io -u "<github_username>" --password-stdin
docker compose pull
docker compose up -d --remove-orphans
```

Useful commands:
```bash
docker compose ps
docker compose logs -f api-go
docker compose down
```

## Start all services
```bash
make start all
make restart all
make stop all
```

Single service:
```bash
make restart llm
make restart agent
make restart core
make restart api
make restart frontend
```

## Local env defaults
- Core -> Agent RPC: `AGENT_RPC_ADDR=127.0.0.1:51000` (compatible with `LLM_RPC_ADDR`)
- Python Agent RPC port: `AGENT_RPC_PORT=51000` (compatible with `LLM_RPC_PORT`)
- Default chat model: `Pro/MiniMaxAI/MiniMax-M2.5`
- Default embedding model: `Qwen/Qwen3-Embedding-4B`
- Provider adapter defaults:
  - active: `siliconflow`
  - chat adapters: `siliconflow`, `ollama`, `openai`/`chatgpt`, `claude`
  - embedding path: fixed to `siliconflow` pipeline (not provider-routed)
  - ollama request timeout: `OLLAMA_TIMEOUT_SECONDS=15` (default, timeout + single retry)

## Tests
```bash
cd backend
go test ./...

cd backend/apps/agent-python-rpc
python3 -m unittest discover -s tests -p 'test_*.py'
```

## API docs artifact
- OpenAPI spec: `docs/generated/openapi.yaml`

## SSE turn streaming
- Endpoint: `POST /api/threads/{thread_id}/turns/stream`
- Headers:
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
  - `Accept: text/event-stream`
- Request body (same as non-streaming create turn):
```json
{
  "message": "hi",
  "scope_type": "all",
  "scope_doc_ids": []
}
```
- Event order:
  - `message`: normalized question/scope payload.
  - `retrieval`: citation candidates.
  - `delta`: incremental answer tokens/chunks (multiple events).
  - `final`: final answer + citations.
  - `done`: stream finished.
  - `error`: stream-side failure.

Example:
```bash
curl -N -X POST "http://localhost:8080/api/threads/<thread_id>/turns/stream" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  --data '{"message":"请用一句话介绍你自己","scope_type":"all","scope_doc_ids":[]}'
```
