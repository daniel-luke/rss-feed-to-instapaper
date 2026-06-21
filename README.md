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
