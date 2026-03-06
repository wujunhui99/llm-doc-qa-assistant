# LLM Document QA Assistant

MVP implementation based on `AGENTS.md` + architecture/spec docs.

## Stack Direction (Go Services)
- Frontend: React (Vite)
- `api-go`: frontend HTTP gateway (`/api/*`)
- `core-go-rpc`: Go domain/rule service (auth, docs, scope, retrieval, agent answer orchestration)
- Middleware:
  - MySQL (auth/session persistence)
  - MinIO (document binary storage)
  - Qdrant (vector index for RAG retrieval)

## Directory Layout (backend)
- `backend/apps/api-go`: HTTP API gateway
- `backend/apps/core-go-rpc`: Go core RPC + built-in LLM agent service
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
- Qdrant HTTP API (`127.0.0.1:6333`)
- Qdrant gRPC (`127.0.0.1:6334`)

## Run services

Terminal A (Go core RPC):
```bash
cd backend
go run ./apps/core-go-rpc/cmd/server
```

Enable vector retrieval + SiliconFlow chat:
```bash
cd backend
VECTOR_SEARCH_ENABLED=true \
QDRANT_ENDPOINT=http://127.0.0.1:6333 \
SILICONFLOW_API_KEY=your_key \
go run ./apps/core-go-rpc/cmd/server
```

Terminal B (Go API gateway):
```bash
cd backend
CORE_RPC_ADDR=127.0.0.1:19090 PORT=8080 go run ./apps/api-go/cmd/api
```

Or run by one command (from repo root):
```bash
make start all
make stop all
make restart all
```

Single service control:
```bash
make restart core
make stop api
make restart frontend
```

Service aliases:
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
export VECTOR_SEARCH_ENABLED='true'
export QDRANT_ENDPOINT='http://127.0.0.1:6333'
export QDRANT_COLLECTION='qa_chunks'
export SILICONFLOW_API_BASE='https://api.siliconflow.cn/v1'
export SILICONFLOW_API_KEY='replace-me'
export SILICONFLOW_CHAT_MODEL='Pro/MiniMaxAI/MiniMax-M2.5'
export SILICONFLOW_EMBEDDING_MODEL='Qwen/Qwen3-Embedding-4B'
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
