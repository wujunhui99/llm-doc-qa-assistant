# CICD.md

## Purpose
This document is the CI/CD specification source of truth.
Agents must generate and update GitHub Actions workflows from this document.
Workflow YAML files are implementation artifacts, not the authority.

## Spec-First Rule
- If `CICD.md` conflicts with any existing workflow YAML, `CICD.md` wins.
- Agent must update workflow YAML to match this spec in the same change set.
- Agent must not require pre-existing YAML files to infer CI/CD behavior.

## Required Workflows

### 1) `ci.yml` (Build + Test Gate)
Trigger:
- `pull_request` to `main`
- `push` to `main`
- `workflow_dispatch`

Required jobs:
- `doc-sync`: fail when non-doc changes are not accompanied by required docs updates.
- `backend-go`: run Go build/checks/tests when Go module exists.
- `frontend-react`: run React build/checks/tests when package exists.

Required behavior:
- Concurrency enabled (`cancel-in-progress: true`).
- Fail-fast on required checks.
- Clear logs for skipped language stacks.
- This workflow does not deploy to server.

### 2) `docker-build.yml` (Docker Image Build)
Trigger:
- `push` to `main`
- `workflow_dispatch`

Required behavior:
- Build backend and frontend container images.
- Produce deterministic tags (at least short SHA).
- Use build cache to reduce publish time.
- If deploy uses remote image pull (this project does), push built images to GHCR.

### 3) `deploy.yml` (CD Deploy)
Trigger:
- After successful `docker-build.yml` on `main` OR manual dispatch.

Required behavior:
- Deploy via SSH.
- Pull latest images and recreate services via Compose.
- Run post-deploy health checks.
- Fail deployment if health checks fail.
- This workflow is deployment-only and does not run code test suites.

## CD Server Profile (Canonical)
The generated `deploy.yml` must use this deployment target profile unless this document is updated.

- SSH host: `${{ vars.DEPLOY_HOST }}`
- SSH user: `${{ vars.DEPLOY_USER }}`
- SSH port: `${{ vars.DEPLOY_PORT }}` or `22`
- App directory: `/home/ubuntu/code/go_code/c2c_monitor`
- Compose directory: `$APP_DIR/deploy/compose`
- Health check endpoints:
  - `http://localhost:8080`
  - `http://localhost:8080/api/config`

## Required GitHub Variables and Secrets
Variables:
- `DEPLOY_HOST`
- `DEPLOY_USER`
- `DEPLOY_PORT` (optional, default `22`)
- `GHCR_USERNAME`

Secrets:
- `DEPLOY_SSH_KEY`
- `GHCR_TOKEN`
- `GITHUB_TOKEN` (GitHub-provided token for Actions where applicable)

## Security and Reliability Constraints
- Do not hardcode secrets in workflow code.
- Keep deploy concurrency control enabled.
- Keep health checks mandatory in deploy workflow.
- Keep CI as a merge gate for `main`.
- Recommended chain: `ci.yml` -> `docker-build.yml` -> `deploy.yml`.

## Agent Output Contract
When asked to create or update CI/CD:
1. Read `CICD.md`.
2. Generate/update `.github/workflows/*.yml` to satisfy this spec.
3. Update docs impacted by CI/CD changes (`docs/SECURITY.md`, `docs/RELIABILITY.md`, and this file if rules changed).
4. Report any unresolved ambiguity explicitly.
