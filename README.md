# LLM Document QA Assistant

Frontend + Go/Python microservices for document QA (RAG + multi-turn agent QA).

## Stack
- Frontend: React (Vite)
- Backend:
  - `api-go` (`:8080`)
  - `core-go-rpc` (`:19090`)
  - `llm-python-rpc` (`127.0.0.1:51000`)
- Middleware:
  - MySQL
  - MinIO
  - Qdrant

## Backend layout
- `backend/apps/api-go`
- `backend/apps/core-go-rpc`
- `backend/apps/llm-python-rpc`
- `backend/proto/qa/v1/qa.proto`

## Start middleware
```bash
docker compose up -d
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
make restart core
make restart api
make restart frontend
```

## Local env defaults
- Core -> LLM RPC: `LLM_RPC_ADDR=127.0.0.1:51000`
- Python LLM RPC port: `LLM_RPC_PORT=51000`
- Default chat model: `Pro/MiniMaxAI/MiniMax-M2.5`
- Default embedding model: `Qwen/Qwen3-Embedding-4B`

## Tests
```bash
cd backend
go test ./...

cd backend/apps/llm-python-rpc
python3 -m unittest discover -s tests -p 'test_*.py'
```
