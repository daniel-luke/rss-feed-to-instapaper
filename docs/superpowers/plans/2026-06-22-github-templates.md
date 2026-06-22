# GitHub Issue & PR Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub issue templates (bug, feature request, help) and a PR template to guide contributors and self-hosters.

**Architecture:** Static YAML issue templates in `.github/ISSUE_TEMPLATE/` using GitHub's native form schema for auto-labeling and guided fields. A single Markdown PR template in `.github/PULL_REQUEST_TEMPLATE.md`.

**Tech Stack:** GitHub issue forms (YAML), Markdown

## Global Constraints

- Issue templates use GitHub issue form schema (`name`, `description`, `labels`, `body` with typed inputs)
- All templates remind users to redact credentials before pasting config/logs
- Commit messages follow convention: `type(scope)[story]: description`

---

### Task 1: Bug report issue template

**Files:**
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`

- [ ] **Step 1: Create `.github/ISSUE_TEMPLATE/` directory and `bug_report.yml`**

```yaml
name: Bug report
description: Something isn't working as expected
labels: ["bug"]
body:
  - type: dropdown
    id: deployment-type
    attributes:
      label: Deployment type
      options:
        - Docker Compose
        - Kubernetes
        - Other
    validations:
      required: true

  - type: input
    id: version
    attributes:
      label: App version
      description: Image tag you are running (e.g. `latest`, `v1.2.0`)
      placeholder: latest
    validations:
      required: true

  - type: textarea
    id: config
    attributes:
      label: config.yaml
      description: >
        Paste your config.yaml here. **Remove any credentials before posting.**
        The `feeds[].url` and `feeds[].label` values are safe to include.
      render: yaml
    validations:
      required: true

  - type: textarea
    id: steps
    attributes:
      label: Steps to reproduce
      placeholder: |
        1. Start the syncer with `docker compose up -d`
        2. Wait for the first sync
        3. ...
    validations:
      required: true

  - type: textarea
    id: expected
    attributes:
      label: Expected behavior
    validations:
      required: true

  - type: textarea
    id: actual
    attributes:
      label: Actual behavior
    validations:
      required: true

  - type: textarea
    id: logs
    attributes:
      label: Relevant log output
      description: >
        Run `docker compose logs syncer` or `kubectl logs <pod>` and paste the relevant lines.
        **Remove any credentials.**
      render: shell

  - type: textarea
    id: context
    attributes:
      label: Additional context
```

- [ ] **Step 2: Verify file is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/ISSUE_TEMPLATE/bug_report.yml'))" && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/ISSUE_TEMPLATE/bug_report.yml
git commit -m "feat(github)[2]: add bug report issue template"
```

---

### Task 2: Feature request issue template

**Files:**
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`

- [ ] **Step 1: Create `feature_request.yml`**

```yaml
name: Feature request
description: Suggest a new feature or improvement
labels: ["enhancement"]
body:
  - type: textarea
    id: problem
    attributes:
      label: Problem description
      description: What problem does this solve? What are you trying to do?
      placeholder: I'm frustrated when...
    validations:
      required: true

  - type: textarea
    id: solution
    attributes:
      label: Proposed solution
      description: Describe what you'd like to see added or changed.
    validations:
      required: true

  - type: textarea
    id: alternatives
    attributes:
      label: Alternatives considered
      description: Other approaches you've thought of or tried.

  - type: textarea
    id: context
    attributes:
      label: Additional context
```

- [ ] **Step 2: Verify file is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/ISSUE_TEMPLATE/feature_request.yml'))" && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/ISSUE_TEMPLATE/feature_request.yml
git commit -m "feat(github)[2]: add feature request issue template"
```

---

### Task 3: Help request issue template

**Files:**
- Create: `.github/ISSUE_TEMPLATE/help_request.yml`

- [ ] **Step 1: Create `help_request.yml`**

```yaml
name: Help request
description: Stuck getting something working? Ask here.
labels: ["question"]
body:
  - type: dropdown
    id: deployment-type
    attributes:
      label: Deployment type
      options:
        - Docker Compose
        - Kubernetes
        - Other
    validations:
      required: true

  - type: textarea
    id: goal
    attributes:
      label: What you're trying to do
      placeholder: I'm trying to get my feeds to sync to Instapaper...
    validations:
      required: true

  - type: textarea
    id: tried
    attributes:
      label: What you've tried
      placeholder: I followed the README steps and...
    validations:
      required: true

  - type: textarea
    id: config-logs
    attributes:
      label: Relevant config / log output
      description: >
        Paste your `config.yaml` and/or log output here.
        **Remove any credentials before posting.**
      render: shell

  - type: textarea
    id: context
    attributes:
      label: Additional context
```

- [ ] **Step 2: Verify file is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/ISSUE_TEMPLATE/help_request.yml'))" && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/ISSUE_TEMPLATE/help_request.yml
git commit -m "feat(github)[2]: add help request issue template"
```

---

### Task 4: Issue template config (blank issue escape hatch)

**Files:**
- Create: `.github/ISSUE_TEMPLATE/config.yml`

- [ ] **Step 1: Create `config.yml`**

```yaml
blank_issues_enabled: true
contact_links:
  - name: Open a blank issue
    url: https://github.com/daniel-luke/rss-feed-to-instapaper/issues/new
    about: Use this if none of the templates above fit your situation.
```

- [ ] **Step 2: Verify file is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/ISSUE_TEMPLATE/config.yml'))" && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/ISSUE_TEMPLATE/config.yml
git commit -m "feat(github)[2]: add issue template config with blank issue escape hatch"
```

---

### Task 5: PR template

**Files:**
- Create: `.github/PULL_REQUEST_TEMPLATE.md`

- [ ] **Step 1: Create `PULL_REQUEST_TEMPLATE.md`**

```markdown
## Description

<!-- What does this PR do? -->

## Motivation

<!-- Why is this change needed? What problem does it solve? -->
<!-- Link to related issue: Closes #N -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation
- [ ] Refactor

## Testing

<!-- How did you test this? -->
<!-- Include manual steps and/or paste output of `go test ./...` -->

## Pre-merge checklist

- [ ] `go test ./...` passes
- [ ] Docs updated if behavior changed (README, config.example.yaml)
- [ ] Commit messages follow convention: `type(scope)[story]: description`
```

- [ ] **Step 2: Commit**

```bash
git add .github/PULL_REQUEST_TEMPLATE.md
git commit -m "feat(github)[2]: add PR template"
```
