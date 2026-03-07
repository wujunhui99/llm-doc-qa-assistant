# QUALITY_SCORE.md

## Core Metrics
- Grounded Answer Rate (with valid citations)
- Multi-turn Continuity Score
- Retrieval Relevance@K
- Vector Retrieval Recall@K (when vector search enabled)
- Scope Resolution Accuracy (`@doc` / `@all`)
- Tenant Isolation Leak Rate
- Unit Test Pass Rate (backend/apps + core domain)
- Worktree Boot Success Rate (agent self-test runs)
- CDP UI Test Pass Rate
- UI Bug Reproduction-and-Validation Success Rate
- P95 Turn Latency
- Error Rate per 100 turns

## MVP Quality Gates
- Grounded Answer Rate >= 85%
- Multi-turn Continuity >= 80%
- Scope Resolution Accuracy >= 99%
- Tenant Isolation Leak Rate = 0
- Unit Test Pass Rate = 100% on changed modules
- Worktree Boot Success Rate >= 95%
- CDP UI Test Pass Rate >= 90%
- UI Bug Reproduction-and-Validation Success Rate >= 85%
- P95 Turn Latency <= 8s (non-streaming equivalent)
- Critical regression count = 0

## Engineering Process Requirement
- Any backend logic change must include corresponding unit tests in the same change set.
- A change is not complete unless unit tests are added/updated and pass locally.
- Minimum local verification before commit:
  - `cd backend && go test ./...`
  - `cd backend/apps/llm-python-rpc && python3 -m unittest discover -s tests -p 'test_*.py'`
