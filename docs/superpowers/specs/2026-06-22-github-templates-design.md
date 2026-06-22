# GitHub Issue & PR Templates Design

## Overview

Add GitHub issue templates and a PR template to improve contributor and self-hoster experience. Primary audience: external users self-hosting via Docker Compose or Kubernetes who encounter bugs or need help.

## Issue Templates

Located in `.github/ISSUE_TEMPLATE/`. Uses GitHub's YAML template format for auto-labeling and guided form fields.

### `bug_report.yml`

Auto-labels: `bug`

Fields:
- **Deployment type** — dropdown: Docker Compose / Kubernetes / Other
- **App version** — image tag or `latest`
- **config.yaml** — code block, with reminder to remove credentials
- **Steps to reproduce** — ordered list
- **Expected behavior** — text
- **Actual behavior** — text
- **Relevant log output** — code block (`docker compose logs syncer` or `kubectl logs`)
- **Additional context** — optional free text

### `feature_request.yml`

Auto-labels: `enhancement`

Fields:
- **Problem description** — prompted with "I'm frustrated when…"
- **Proposed solution** — what they'd like to see
- **Alternatives considered** — other approaches they thought of
- **Additional context** — optional free text

### `help_request.yml`

Auto-labels: `question`

Fields:
- **Deployment type** — dropdown: Docker Compose / Kubernetes / Other
- **What you're trying to do** — text
- **What you've tried** — text
- **Relevant config / log output** — code block, with reminder to remove credentials
- **Additional context** — optional free text

### `config.yml`

Enables the blank issue escape hatch. Provides a link to open a blank issue for anything that doesn't fit the above templates.

## PR Template

Single file: `.github/PULL_REQUEST_TEMPLATE.md`

Sections:
- **Description** — what this PR does, in plain terms
- **Motivation** — why this change is needed; link to related issue if applicable
- **Type of change** — checkbox list: bug fix / new feature / breaking change / documentation / refactor
- **Testing** — what was tested and how (manual steps taken, `go test ./...` run)
- **Pre-merge checklist** — all tests pass, docs updated if behavior changed, commit messages follow project convention (`type(scope)[story]: description`)

## File Structure

```
.github/
  ISSUE_TEMPLATE/
    bug_report.yml
    feature_request.yml
    help_request.yml
    config.yml
  PULL_REQUEST_TEMPLATE.md
```
