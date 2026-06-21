# Docker Compose — Design Spec

## Overview

Add a Docker Compose setup so anyone can clone the repo and run the syncer locally without Kubernetes. Targets open-source sharing on GitHub.

## Architecture

Two services: `syncer` (one-shot Go binary, exits after each run) + `ofelia` (scheduler, starts a fresh syncer container on schedule via `job-run`). Ofelia reads Docker labels on the syncer service definition. No changes to the Go binary or Dockerfile.

```
ofelia (always running)
  └─ reads labels on syncer service
  └─ runs a new syncer container on schedule
       ├─ reads credentials from .env
       ├─ reads config from ./config.yaml (bind-mount)
       └─ writes SQLite state to named volume
```

## Files

### `docker-compose.yml`
```yaml
services:
  ofelia:
    image: mcuadros/ofelia:latest
    restart: unless-stopped
    command: daemon --docker
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    depends_on:
      - syncer

  syncer:
    build: .
    restart: "no"
    env_file: .env
    volumes:
      - state:/data
      - ./config.yaml:/config/config.yaml:ro
    labels:
      ofelia.enabled: "true"
      ofelia.job-run.syncer.schedule: "${SYNC_SCHEDULE:-@every 5m}"

volumes:
  state:
```

### `.env.example`
```env
INSTAPAPER_USERNAME=your-instapaper-email@example.com
INSTAPAPER_PASSWORD=your-instapaper-password
INSTAPAPER_CONSUMER_KEY=your-consumer-key
INSTAPAPER_CONSUMER_SECRET=your-consumer-secret
SYNC_SCHEDULE=@every 5m
```

### `.gitignore`
Add `.env` line.

### `README.md`
Expand with local setup section:
1. Copy `config.example.yaml` → `config.yaml`, fill in feeds
2. Copy `.env.example` → `.env`, fill in credentials
3. `docker compose up -d`
4. Manual run: `docker compose run --rm syncer`
5. Reset state: `docker compose down -v && docker compose up -d`

Note Docker socket requirement for ofelia (`/var/run/docker.sock`).

## Credentials

Passed via `.env` file (gitignored). Matches the four vars already in `manifests/secret.yaml`. `.env.example` committed with placeholders.

## Schedule

Default `@every 5m` (matches Kubernetes CronJob `*/5 * * * *`). Override via `SYNC_SCHEDULE` in `.env`. Supports any ofelia/cron expression.

## State Reset

Delete the named volume: `docker compose down -v`. Equivalent to deleting the Kubernetes PVC. Document in README.

## Constraints

- No changes to Go binary or Dockerfile
- `.env` must never be committed (enforced by `.gitignore`)
- `config.yaml` must never be committed (already gitignored)
- Ofelia requires Docker socket access — document this in README as a known requirement
