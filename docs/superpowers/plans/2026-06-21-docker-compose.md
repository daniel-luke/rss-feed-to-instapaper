# Docker Compose — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Docker Compose setup so anyone can clone the repo and run the syncer locally without Kubernetes.

**Architecture:** Two services — `syncer` (Alpine container, runs `sleep infinity` so it stays alive) + `ofelia` (scheduler that `exec`s `/syncer` inside the syncer container on schedule via Docker socket). Ofelia reads `job-exec` labels on the syncer container. All env vars and volume mounts are declared once on the syncer service; ofelia reuses the running container. No changes to the Go binary.

**Tech Stack:** Docker Compose v2, `mcuadros/ofelia:latest`

## Global Constraints

- No changes to the Go binary (`cmd/syncer/main.go`) or Dockerfile
- `.env` must never be committed (enforce via `.gitignore`)
- `config.yaml` already gitignored — verify it stays that way
- Credential placeholders only in all committed files
- Default schedule: `@every 5m` (matches Kubernetes CronJob `*/5 * * * *`)
- `SYNC_SCHEDULE` env var in `.env` overrides the schedule

---

## File Map

| File | Change |
|------|--------|
| `docker-compose.yml` | Create: two services (syncer + ofelia), named volume, labels |
| `.env.example` | Create: credential + schedule template |
| `.gitignore` | Add `.env` line |
| `README.md` | Rewrite: add project description, local quickstart, k8s section, state reset docs |

---

### Task 1: Docker Compose setup + README

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Modify: `.gitignore`
- Modify: `README.md`

**Interfaces:**
- Consumes: existing `Dockerfile` (unchanged), existing env var names (`INSTAPAPER_*`), existing volume paths (`/data`, `/config/config.yaml`)
- Produces: runnable local setup

- [ ] **Step 1: Create `docker-compose.yml`**

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
    restart: unless-stopped
    entrypoint: ["sleep", "infinity"]
    env_file: .env
    volumes:
      - state:/data
      - ./config.yaml:/config/config.yaml:ro
    labels:
      ofelia.enabled: "true"
      ofelia.job-exec.run-sync.schedule: "${SYNC_SCHEDULE:-@every 5m}"
      ofelia.job-exec.run-sync.command: "/syncer"
      ofelia.job-exec.run-sync.no-overlap: "true"

volumes:
  state:
```

- [ ] **Step 2: Create `.env.example`**

```env
INSTAPAPER_USERNAME=your-instapaper-email@example.com
INSTAPAPER_PASSWORD=your-instapaper-password
INSTAPAPER_CONSUMER_KEY=your-consumer-key
INSTAPAPER_CONSUMER_SECRET=your-consumer-secret
SYNC_SCHEDULE=@every 5m
```

- [ ] **Step 3: Update `.gitignore`**

Current `.gitignore`:
```
/syncer
*.db
*.db-shm
*.db-wal
config.yaml
```

Add `.env` so it becomes:
```
/syncer
*.db
*.db-shm
*.db-wal
config.yaml
.env
```

- [ ] **Step 4: Validate the compose file**

```bash
docker compose config
```

Expected: prints the resolved compose config with no errors. The `SYNC_SCHEDULE` variable will show as `@every 5m` (the default).

- [ ] **Step 5: Rewrite `README.md`**

Replace the full `README.md` with:

````markdown
# RSS Feed to Instapaper

Syncs RSS/Atom feeds to your [Instapaper](https://www.instapaper.com/) reading list. Articles are added in publication-date order (newest on top). Articles older than `max_age_days` are archived automatically.

## Quick Start (Docker Compose)

**Requirements:** Docker with Compose v2, Instapaper full API credentials ([request here](https://www.instapaper.com/main/request_oauth_consumer_token))

```bash
# 1. Copy and fill in your config
cp config.example.yaml config.yaml
# Edit config.yaml: add your feeds

# 2. Copy and fill in your credentials
cp .env.example .env
# Edit .env: add your Instapaper credentials

# 3. Start
docker compose up -d
```

The syncer runs on schedule (default: every 5 minutes). Check logs:

```bash
docker compose logs -f syncer
```

**Change the schedule:** edit `SYNC_SCHEDULE` in `.env` (uses [ofelia schedule syntax](https://pkg.go.dev/github.com/robfig/cron), e.g. `@every 15m` or `0 */2 * * *`), then restart:

```bash
docker compose down && docker compose up -d
```

**Run manually:**

```bash
docker compose exec syncer /syncer
```

**Reset state** (re-add all articles on next run):

```bash
docker compose down -v && docker compose up -d
```

> **Note:** ofelia requires access to the Docker socket (`/var/run/docker.sock`) to schedule jobs.

## Configuration

Edit `config.yaml`:

```yaml
max_age_days: 1          # archive articles older than this many days (default: 1)
# sort_by_date: false    # uncomment to add in feed order instead of newest-first

feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
  - url: "https://another-blog.com/rss"
    label: "Another Blog"
```

## Kubernetes

Apply manifests in order:

```bash
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/pvc.yaml
kubectl apply -f manifests/secret.yaml   # update with real credentials first
kubectl create configmap rss-syncer-config \
  --from-file=config.yaml=config.yaml \
  -n rss-feed-to-instapaper
kubectl apply -f manifests/cronjob.yaml
```

**Update feed config:**

```bash
kubectl delete configmap rss-syncer-config -n rss-feed-to-instapaper
kubectl create configmap rss-syncer-config \
  --from-file=config.yaml=config.yaml \
  -n rss-feed-to-instapaper
```

**Reset state** (re-add all articles on next run):

```bash
kubectl delete pvc rss-syncer-state -n rss-feed-to-instapaper
kubectl apply -f manifests/pvc.yaml
```
````

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml .env.example .gitignore README.md
git commit -m "feat(compose): add Docker Compose setup with ofelia scheduler"
```
