# PLANS.md

## Current Focus
Go + Python microservice split has been scaffolded and implemented with gRPC contracts:
- external API remains stable on `api-go` (`/api/*`),
- domain logic runs in `core-go-rpc`,
- answer generation is isolated in `llm-python-rpc`.

## Next Focus
- Move document/thread/turn persistence from JSON store to MySQL schema.
- Add resilient retry/circuit-breaker policy for Core->LLM RPC.
- Introduce vector store and reranker modules in Python service.
- Add inter-service auth hardening (mTLS or signed service tokens) for production.
