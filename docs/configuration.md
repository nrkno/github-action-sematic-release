---
type: Configuration
title: semrel configuration reference
description: All environment variables consumed by semrel, subcommand flags, and GITHUB_OUTPUT fields written on a successful release.
tags: [configuration, environment-variables, flags, github-output]
timestamp: 2026-06-29
---

# Configuration

semrel is configured entirely through environment variables. There is no configuration
file. All variables are provided automatically by GitHub Actions; no manual setup is
required for standard workflows.

---

## Environment variables

### Authentication

| Variable | Required | Description |
| -------- | -------- | ----------- |
| `GITHUB_TOKEN` | Yes | GitHub token used for API calls and git push. Set automatically by GitHub Actions via `secrets.GITHUB_TOKEN`. |

### GitHub Actions context (set automatically)

| Variable | Description |
| -------- | ----------- |
| `GITHUB_REPOSITORY` | Owner and repo name in `owner/repo` format (e.g., `nrkno/myservice`). |
| `GITHUB_REF` | Full ref of the event (e.g., `refs/heads/main`, `refs/pull/42/merge`). Used to extract the PR number on `pull_request` events. |
| `GITHUB_REF_NAME` | Short ref name (e.g., `main`, `42/merge`). |
| `GITHUB_SHA` | SHA of the commit that triggered the workflow. |
| `GITHUB_EVENT_NAME` | Event type that triggered the workflow (`push`, `pull_request`, `release`, etc.). Used by `lint` to determine the commit range. |
| `GITHUB_BASE_REF` | Target branch of a pull request. Only set on `pull_request` events. Used by `lint` to define the range start. |
| `GITHUB_OUTPUT` | Path to the step output file. semrel appends `key=value` lines here when outputs are available. |
| `GITHUB_API_URL` | GitHub API base URL. Defaults to `https://api.github.com`. Override for GitHub Enterprise Server. |
| `GITHUB_SERVER_URL` | GitHub server base URL. Defaults to `https://github.com`. Override for GitHub Enterprise Server. |
| `GITHUB_ACTIONS` | Set to `true` inside a GitHub Actions runner. semrel uses this to detect whether it is running in CI. |
| `GITHUB_RUN_ID` | Unique ID of the current workflow run (informational). |
| `GITHUB_ACTOR` | GitHub username of the person or app that triggered the run (informational). |

### semrel-specific (set by the release job, consumed by notify)

These are passed explicitly in the workflow from the `release` job outputs to the
`notify` job:

| Variable | Set by | Consumed by | Description |
| -------- | ------ | ----------- | ----------- |
| `SEMREL_RELEASED` | release job output | `notify` | `"true"` if a release was created, `"false"` for a no-op. |
| `SEMREL_VERSION` | release job output | `notify` | Version string without `v` prefix (e.g., `1.2.3`). |
| `SEMREL_TAG` | release job output | `notify` | Full tag name (e.g., `v1.2.3`). |

---

## Subcommand flags

### `lint`

| Flag | Type | Default | Description |
| ---- | ---- | ------- | ----------- |
| `--from-ref` | string | auto-detected | Start of the commit range. Defaults to `GITHUB_BASE_REF` on `pull_request` events, or the latest annotated tag on `push`/`release` events. |
| `--to-ref` | string | `HEAD` | End of the commit range. |

### `release`

| Flag | Type | Default | Description |
| ---- | ---- | ------- | ----------- |
| `--dry-run` | bool | `false` | Compute the next version and write output fields, but do not create the tag, push, or create the GitHub Release. Useful for testing version calculation. |

### `notify`

No flags. All configuration comes from environment variables.

### `notes`

No flags. All configuration comes from environment variables.

---

## GITHUB_OUTPUT fields

The `release` subcommand writes the following fields to `GITHUB_OUTPUT` on both
success (release created) and no-op (release already existed):

| Field | Example value | Description |
| ----- | ------------- | ----------- |
| `released` | `true` / `false` | Whether a new release was created in this run. |
| `version` | `1.2.3` | Version string without the `v` prefix. |
| `tag` | `v1.2.3` | Git tag name (always `v`-prefixed). |
| `major` | `1` | Major version component. |
| `minor` | `2` | Minor version component. |
| `patch` | `3` | Patch version component. |

When `released=false` (no-op run), all version fields still reflect the **current**
version (not empty), so downstream jobs can always read `steps.semrel.outputs.version`
without a null-check.

The `notes` subcommand writes one additional field:

| Field | Description |
| ----- | ----------- |
| `notes` | Full Markdown release notes body. |

---

## Version bump rules

The next version is calculated from the highest-impact commit type in the range:

| Commit signal | Bump |
| ------------- | ---- |
| Any commit with `BREAKING CHANGE:` footer or `!` suffix | major |
| `feat` | minor |
| `fix`, `perf`, `refactor` | patch |
| `docs`, `style`, `test`, `build`, `ci`, `chore` | none (no release) |

Bootstrap (no previous annotated tag): first release is always `v0.0.1`.
