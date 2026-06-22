# Security Policy

## Supported Versions

Only the latest release receives security patches. Older versions are not maintained.

## Reporting a Vulnerability

Report security vulnerabilities via [GitHub Security Advisories](https://github.com/daniel-luke/rss-feed-to-instapaper/security/advisories/new). Do **not** open a public issue.

Include as much detail as possible: steps to reproduce, affected versions, and potential impact.

Response time is best-effort. There is no guaranteed SLA for acknowledgement or resolution.

## Security Considerations

This tool handles sensitive credentials:

- **Instapaper credentials** — stored in `.env` (Docker) or a Kubernetes Secret. Never commit these to version control.
- **OAuth consumer key/secret** — treat these like passwords. Rotate them if compromised via [Instapaper's API request page](https://www.instapaper.com/main/request_oauth_consumer_token).
- **Docker socket access** — the Docker Compose setup mounts `/var/run/docker.sock`. Limit access to this socket to trusted users only.
- **Feed URLs** — only add feeds from sources you trust; malicious feed content is passed to Instapaper.
