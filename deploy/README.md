# Deploy Compose

This directory is the deployment runtime artifact for Docker Compose.

## Structure
- `deploy/compose/docker-compose.yml`: production compose stack that pulls GHCR images.
- `deploy/compose/.env.example`: environment variable template for server-side runtime.

## Server usage
From `deploy/compose` on server:

```bash
docker compose pull
docker compose up -d --remove-orphans
```

The compose file expects:
- `GHCR_USERNAME`
- `IMAGE_TAG` (for example `main` or `sha-<short>`)

CI deploy workflow exports `IMAGE_TAG` automatically.
