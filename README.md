# LLM Document QA Assistant

MVP implementation based on `AGENTS.md` + architecture/spec docs.

## Stack Direction (Go + Python Microservices)
- Frontend: React (Vite)
- `api-go`: frontend HTTP gateway (`/api/*`)
- `core-go-rpc`: Go domain/rule service (auth, docs, scope, turn orchestration)
- `llm-python-rpc`: Python LLM answer service
- Middleware:
  - MySQL (auth/session persistence)
  - MinIO (document binary storage)

## Directory Layout (backend)
- `backend/services/api-go`: HTTP API gateway
- `backend/services/core-go-rpc`: Go core RPC service
- `backend/services/llm-python-rpc`: Python LLM RPC service
- `backend/proto/qa/v1/qa.proto`: gRPC contract source
- `backend/proto/gen/go/qa/v1`: Go generated stubs

## Middleware startup (docker-compose)
```bash
docker compose up -d
```

This starts:
- MySQL (`127.0.0.1:3306`)
- MinIO API (`127.0.0.1:9000`)
- MinIO Console (`127.0.0.1:9001`)

## Run microservices

Terminal A (Python LLM RPC):
```bash
cd backend/services/llm-python-rpc
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
LLM_RPC_ADDR=:19091 python -m app.server
```

Terminal B (Go core RPC):
```bash
cd backend
LLM_RPC_ADDR=127.0.0.1:19091 go run ./services/core-go-rpc/cmd/server
```

Terminal C (Go API gateway):
```bash
cd backend
CORE_RPC_ADDR=127.0.0.1:19090 PORT=8080 go run ./services/api-go/cmd/api
```

Or run by one command (from repo root):
```bash
make start all
make stop all
make restart all
```

Single service control:
```bash
make start llm
make restart core
make stop api
make restart frontend
```

Service aliases:
- `llm` / `llm-python-rpc` / `python` (`:19091`)
- `core` / `core-go-rpc` / `go-rpc` (`:19090`)
- `api` / `api-go` (`:8080`)
- `frontend` / `fe` / `web` (`:5173`)

Optional shared env overrides:
```bash
export MYSQL_DSN='app:app123456@tcp(127.0.0.1:3306)/llm_doc_qa?parseTime=true&charset=utf8mb4&loc=Local'
export MINIO_ENDPOINT='127.0.0.1:9000'
export MINIO_ACCESS_KEY='minioadmin'
export MINIO_SECRET_KEY='minioadmin123'
export MINIO_BUCKET='qa-documents'
export MINIO_USE_SSL='false'
export INTERNAL_SERVICE_TOKEN='change-me-in-prod'
```

## Frontend run
```bash
cd frontend
npm install
npm run dev
```

## External API (via `api-go`)
- Auth: `/api/auth/register`, `/api/auth/login`, `/api/auth/logout`, `/api/auth/me`
- Documents: `/api/documents/upload`, `/api/documents`, `/api/documents/{id}`, `/api/documents/{id}/download`
- QA: `/api/threads`, `/api/threads/{thread_id}/turns`, `/api/threads/{thread_id}/turns/{turn_id}/stream`
- Config: `/api/config`, `/api/config/health`
