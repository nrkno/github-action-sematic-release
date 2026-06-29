---
type: API Reference
title: semrel API reference
description: Complete reference for all semrel subcommands — synopsis, flags, exit codes, stdout/stderr behaviour, and GITHUB_OUTPUT fields emitted.
tags: [api-reference, subcommands, exit-codes, flags, github-output]
timestamp: 2026-06-29
---

# API reference

## Overview

```
semrel <subcommand> [flags]
```

All subcommands:

- Write structured log messages to **stderr** (`log/slog` text format)
- Reserve **stdout** for data output only (`notes` subcommand)
- Exit **0** for success or no-op
- Exit **1** for expected, user-actionable failures
- Exit **2** for system errors (misconfiguration, shallow clone)

---

## `semrel lint`

Validates that every commit in the relevant range conforms to
[Conventional Commits 1.0](https://www.conventionalcommits.org/).

### Synopsis

```
semrel lint [--from-ref <ref>] [--to-ref <ref>]
```

### Flags

| Flag | Type | Default | Description |
| ---- | ---- | ------- | ----------- |
| `--from-ref` | string | auto | Range start. On `pull_request`: `GITHUB_BASE_REF`. On `push`/`release`: latest annotated tag. On other events: beginning of history. |
| `--to-ref` | string | `HEAD` | Range end. |

### Automatic range detection

| `GITHUB_EVENT_NAME` | Default `--from-ref` | Default `--to-ref` |
| ------------------- | -------------------- | ------------------ |
| `pull_request` | `GITHUB_BASE_REF` (target branch) | `HEAD` |
| `push` | latest annotated tag | `HEAD` |
| `release` | latest annotated tag | `HEAD` |
| *(other / empty)* | beginning of history | `HEAD` |

### Exit codes

| Code | Meaning |
| ---- | ------- |
| `0` | All commits in range are valid conventional commits. |
| `1` | One or more commits violated the conventional commit format. Violations are printed to stderr, one per commit: `commit <short-sha>: <rule>\n  <raw message>\n  example: <example>`. |
| `2` | System error (e.g., shallow repository, git operation failed). |

### Stdout

Nothing. Lint output goes to stderr.

### Stderr

On violation (exit 1), one block per offending commit:

```
commit abc1234: missing type
  wip stuff
  example: fix: correct handling of empty input
```

---

## `semrel release`

Computes the next semantic version from conventional commits, creates an annotated
git tag, pushes it, and creates a GitHub Release.

### Synopsis

```
semrel release [--dry-run]
```

### Flags

| Flag | Type | Default | Description |
| ---- | ---- | ------- | ----------- |
| `--dry-run` | bool | `false` | Compute version and write `GITHUB_OUTPUT` fields without creating the tag, pushing, or calling the GitHub Releases API. |

### Exit codes

| Code | Meaning |
| ---- | ------- |
| `0` | Release created, or no release needed (no releasable commits), or release already existed (idempotent re-run). All `GITHUB_OUTPUT` fields are written in all `0` cases. |
| `1` | Tag conflict: the computed tag already exists but points to a different commit SHA. This indicates a concurrent release run computed the same version. Re-run the workflow or investigate the tag. |
| `2` | System error: shallow repository, missing `GITHUB_TOKEN`, invalid `GITHUB_REPOSITORY` format, or git operation failed. |

### GITHUB_OUTPUT fields

Written on exit 0 in all cases (released or not):

| Field | Example | Notes |
| ----- | ------- | ----- |
| `released` | `true` / `false` | `false` when no releasable commits were found. |
| `version` | `1.2.3` | Current version (without `v`). Always set. |
| `tag` | `v1.2.3` | Current tag. Always set. |
| `major` | `1` | Major component. Always set. |
| `minor` | `2` | Minor component. Always set. |
| `patch` | `3` | Patch component. Always set. |

### Stdout

Nothing. Log messages go to stderr.

### Idempotency

`release` is safe to re-run. See [Architecture — Idempotency ladder](/docs/architecture.md)
for the three-rung precedence logic.

---

## `semrel notify`

Posts a deduplicated comment on the merged PR announcing the new release version.

### Synopsis

```
semrel notify
```

### Flags

None.

### Skip conditions

`notify` exits 0 immediately (no-op) when any of the following are true:

- `SEMREL_RELEASED=false` — the release job produced no release
- `GITHUB_EVENT_NAME` ≠ `pull_request` — not running in a PR context
- `GITHUB_REF` does not contain a parseable PR number
- A `<!-- semrel-notify:<version> -->` marker comment already exists on the PR
  (deduplication guard)

### Marker format

Comments posted by `notify` contain an HTML marker on the first line:

```
<!-- semrel-notify:1.2.3 -->
🎉 Release 1.2.3 created!
```

The marker is used to detect and skip duplicate posts. It is not visible when the
comment is rendered.

### Exit codes

| Code | Meaning |
| ---- | ------- |
| `0` | Comment posted, or skipped (already exists / not a PR context / `SEMREL_RELEASED=false`). |
| `1` | GitHub API error (comment check or post failed). |

### Stdout

Nothing. Log messages go to stderr.

---

## `semrel notes`

Generates Markdown release notes from conventional commits since the last annotated
tag and linked PRs.

### Synopsis

```
semrel notes
```

### Flags

None.

### Output format

Release notes are written to **stdout** in Markdown:

```markdown
## What's Changed

### Features
- feat: add retry logic for API calls (abc1234) (#42)

### Bug Fixes
- fix: handle empty commit range correctly (def5678) (#43)

### Other Changes
- chore: update dependencies (ghi9012)
```

Commits are grouped by type. PR links are resolved via the GitHub API
(`GET /repos/{owner}/{repo}/commits/{sha}/pulls`) with a fallback to search.

### Exit codes

| Code | Meaning |
| ---- | ------- |
| `0` | Notes written to stdout (and to `GITHUB_OUTPUT` `notes` field if `GITHUB_OUTPUT` is set). |
| `1` | System error: git operation, GitHub API call, or output write failed. |

### GITHUB_OUTPUT fields

| Field | Description |
| ----- | ----------- |
| `notes` | Full Markdown notes body (only written if `GITHUB_OUTPUT` is set). |

---

## Global flags

| Flag | Description |
| ---- | ----------- |
| `--help` / `-h` | Print usage. |
| `--version` | Print the semrel version, commit SHA, and build date embedded at build time. |
