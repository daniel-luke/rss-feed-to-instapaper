# RSS → Instapaper Syncer — Design Spec

**Date:** 2026-06-21  
**Status:** Approved

---

## Overview

Containerized Go binary that runs on a Kubernetes CronJob schedule. Reads RSS feeds from a YAML config file, finds new items not yet sent, and adds them to Instapaper via the Simple API. State (sent GUIDs) is persisted in SQLite on a PersistentVolumeClaim.

---

## Architecture

### Execution flow

```
start
  → load /config/config.yaml
  → open SQLite at /data/state.db
  → for each feed:
      → fetch + parse RSS (gofeed)
      → for each item: skip if GUID already in SQLite
      → POST to Instapaper Simple API (/api/add)
      → on success: insert GUID into SQLite
  → exit
```

Sequential per-feed, sequential per-item. No concurrency needed.

### Instapaper Simple API

Endpoint: `https://www.instapaper.com/api/add`  
Method: POST  
Params: `username`, `password`, `url`, `title` (optional)  
Auth: HTTP Basic Auth  
Success: HTTP 201  
Error: HTTP 400 (bad request), 500 (server error)

No OAuth, no consumer key required.

---

## Project Structure

```
rss-feed-to-instapaper/
├── cmd/syncer/main.go          # entry point, orchestration
├── internal/
│   ├── config/config.go        # load + validate config.yaml
│   ├── feed/fetcher.go         # fetch + parse RSS via gofeed
│   ├── instapaper/client.go    # POST to Instapaper Simple API
│   └── state/db.go             # SQLite open/read/write (mattn/go-sqlite3)
├── manifests/
│   ├── cronjob.yaml            # k8s CronJob
│   ├── pvc.yaml                # PersistentVolumeClaim for SQLite
│   └── secret.yaml             # Secret template (no real values)
├── config.example.yaml         # feed config example
├── Dockerfile
└── go.mod
```

---

## Configuration

### Feed config (`config.yaml`, mounted into container)

```yaml
feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
  - url: "https://another.com/rss"
    label: "News"
```

Mounted as a volume from a k8s ConfigMap.

### Credentials (k8s Secret → env vars)

```
INSTAPAPER_USERNAME=<email>
INSTAPAPER_PASSWORD=<password>
```

---

## State

SQLite database at `/data/state.db` (on PVC).  
Table: `sent_items(guid TEXT PRIMARY KEY, sent_at DATETIME)`  

On each run: SELECT known GUIDs for this feed, skip any matching, INSERT new ones after successful POST.

---

## Error Handling

| Scenario | Behavior |
|---|---|
| Feed fetch fails | Log error, skip feed, continue others |
| Instapaper POST fails | Log error + URL, skip item (GUID not inserted — retry next run) |
| SQLite unavailable | Fatal exit (code 1) |
| Config missing or invalid | Fatal exit (code 1) |
| All feeds processed | Exit 0 (even if some items were skipped due to errors) |

---

## Docker

Multi-stage build. CGO required for `mattn/go-sqlite3`.

```dockerfile
# stage 1: build
FROM golang:1.23-alpine AS builder
RUN apk add gcc musl-dev
WORKDIR /app
COPY . .
RUN go build -o /syncer ./cmd/syncer

# stage 2: runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /syncer /syncer
ENTRYPOINT ["/syncer"]
```

Expected image size: ~15MB.

---

## Kubernetes Manifests

### CronJob (`manifests/cronjob.yaml`)

- Schedule: `"0 * * * *"` (hourly, configurable)
- `restartPolicy: OnFailure`
- Two volume mounts:
  - `/data` from PVC (SQLite state)
  - `/config/config.yaml` from ConfigMap (feed list)
- `envFrom` referencing the Instapaper credentials Secret

### PVC (`manifests/pvc.yaml`)

- `accessModes: [ReadWriteOnce]`
- `storage: 100Mi` (SQLite is tiny)

### Secret template (`manifests/secret.yaml`)

Placeholder values only — no real credentials committed to git.

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/mmcdole/gofeed` | RSS/Atom parsing |
| `github.com/mattn/go-sqlite3` | SQLite (CGO) |
| `gopkg.in/yaml.v3` | Config parsing |

---

## Out of Scope

- Folder support (requires Instapaper xAuth full API)
- Parallel feed fetching
- Metrics / alerting (rely on CronJob failure notifications from k8s)
- Web UI or admin interface
