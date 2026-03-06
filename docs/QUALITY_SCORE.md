# QUALITY_SCORE.md

## Core Metrics
- Grounded Answer Rate (with valid citations)
- Multi-turn Continuity Score
- Retrieval Relevance@K
- Scope Resolution Accuracy (`@doc` / `@all`)
- Tenant Isolation Leak Rate
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
- Worktree Boot Success Rate >= 95%
- CDP UI Test Pass Rate >= 90%
- UI Bug Reproduction-and-Validation Success Rate >= 85%
- P95 Turn Latency <= 8s (non-streaming equivalent)
- Critical regression count = 0
