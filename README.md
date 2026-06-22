# RSS Feed to Instapaper

Automatically sync RSS and Atom feeds to your [Instapaper](https://www.instapaper.com/) reading list. New articles are added on a schedule, old articles are archived, and optionally deleted after a set number of days — so your reading list stays clean without any manual effort.

Built with Kobo e-reader users in mind: if you use Instapaper's native Kobo integration, this gives you RSS feeds on your Kobo without installing KoReader or any other firmware modifications.

## Features

- **Automatic syncing** — polls your feeds on a configurable schedule (default: every 5 minutes)
- **Newest-first ordering** — articles are added in publication-date order so the latest appears at the top of your Instapaper list
- **Auto-archive** — articles older than a configurable number of days are automatically archived in Instapaper
- **Auto-delete archived articles** — optionally remove archived articles from Instapaper entirely after a set number of days
- **Duplicate prevention** — state is persisted between runs; articles are never added twice
- **Docker and Kubernetes support** — deploy anywhere with minimal configuration

## Prerequisites

- An [Instapaper](https://www.instapaper.com/) account
- Instapaper Full API credentials — [request them here](https://www.instapaper.com/main/request_oauth_consumer_token) (free, takes a day or two to receive)
- Docker with Compose v2 **or** a Kubernetes cluster

---

## Docker Compose Setup

### 1. Get the files

Clone this repository or download `docker-compose.yml`, `config.example.yaml`, and `.env.example`:

```bash
git clone https://github.com/daniel-luke/rss-feed-to-instapaper.git
cd rss-feed-to-instapaper
```

### 2. Configure your credentials

Copy the example environment file and fill in your Instapaper credentials:

```bash
cp .env.example .env
```

Edit `.env`:

```env
INSTAPAPER_USERNAME=your-instapaper-email@example.com
INSTAPAPER_PASSWORD=your-instapaper-password
INSTAPAPER_CONSUMER_KEY=your-consumer-key
INSTAPAPER_CONSUMER_SECRET=your-consumer-secret
SYNC_SCHEDULE=@every 5m
```

### 3. Configure your feeds

Copy the example config and add your feeds:

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` — see [Configuration](#configuration) for all options.

### 4. Start

```bash
docker compose up -d
```

Check that it's running:

```bash
docker compose logs -f syncer
```

### Common operations

**Run a sync manually:**

```bash
docker compose exec syncer /syncer
```

**Change the sync schedule** — edit `SYNC_SCHEDULE` in `.env`, then restart:

```bash
docker compose down && docker compose up -d
```

**Reset state** (re-adds all articles on next run):

```bash
docker compose down -v && docker compose up -d
```

> **Note:** The scheduler ([ofelia](https://github.com/mcuadros/ofelia)) requires read access to the Docker socket (`/var/run/docker.sock`) to schedule jobs.

---

## Kubernetes Setup

### 1. Create the namespace

```bash
kubectl create namespace rss-feed-to-instapaper
```

### 2. Create the secret

Edit `manifests/secret.yaml` and fill in your Instapaper credentials, then apply:

```bash
kubectl apply -f manifests/secret.yaml
```

### 3. Create the persistent volume claim

```bash
kubectl apply -f manifests/pvc.yaml
```

### 4. Create the feed config

Create a ConfigMap from your `config.yaml`:

```bash
kubectl create configmap rss-syncer-config \
  --from-file=config.yaml=config.yaml \
  -n rss-feed-to-instapaper
```

### 5. Deploy the CronJob

```bash
kubectl apply -f manifests/cronjob.yaml
```

The CronJob runs every 5 minutes by default. To change the schedule, edit the `schedule` field in `manifests/cronjob.yaml` (standard cron syntax).

### Common operations

**Update feed config after editing `config.yaml`:**

```bash
kubectl delete configmap rss-syncer-config -n rss-feed-to-instapaper
kubectl create configmap rss-syncer-config \
  --from-file=config.yaml=config.yaml \
  -n rss-feed-to-instapaper
```

**Reset state** (re-adds all articles on next run):

```bash
kubectl delete pvc rss-syncer-state -n rss-feed-to-instapaper
kubectl apply -f manifests/pvc.yaml
```

---

## Configuration

All feed and sync behavior is configured in `config.yaml`.

| Option | Type | Default | Description |
|---|---|---|---|
| `max_age_days` | integer | `1` | Articles older than this many days are automatically archived in Instapaper |
| `archive_retention_days` | integer | *(disabled)* | If set, archived articles are permanently deleted from Instapaper after this many days |
| `sort_by_date` | boolean | `true` | When `true`, articles are added newest-first. Set to `false` to add in feed order |
| `feeds` | list | *(required)* | List of RSS/Atom feeds to sync |
| `feeds[].url` | string | *(required)* | URL of the RSS or Atom feed |
| `feeds[].label` | string | *(optional)* | Label to apply to articles added from this feed in Instapaper |

### Example

```yaml
max_age_days: 7
archive_retention_days: 30
# sort_by_date: false  # uncomment to add in feed order instead of newest-first

feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
  - url: "https://another-blog.com/rss"
    label: "Another Blog"
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `INSTAPAPER_USERNAME` | Yes | Your Instapaper account email address |
| `INSTAPAPER_PASSWORD` | Yes | Your Instapaper account password |
| `INSTAPAPER_CONSUMER_KEY` | Yes | OAuth consumer key from the Instapaper Full API |
| `INSTAPAPER_CONSUMER_SECRET` | Yes | OAuth consumer secret from the Instapaper Full API |
| `SYNC_SCHEDULE` | No | Sync schedule in [ofelia/cron syntax](https://pkg.go.dev/github.com/robfig/cron). Default: `@every 5m`. Examples: `@every 15m`, `0 */2 * * *` |

> **Kubernetes:** `SYNC_SCHEDULE` is not used in the Kubernetes setup. Edit the `schedule` field in `manifests/cronjob.yaml` instead.
