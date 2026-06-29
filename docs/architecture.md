---
type: Architecture
title: semrel architecture
description: Why semrel exists, internal package structure, and key design decisions including go-git, distroless, idempotency ladder, and shallow clone guard.
tags: [architecture, supply-chain, go-git, distroless, cosign, idempotency]
timestamp: 2026-06-29
---

# Architecture

## Why semrel exists

In 2024, the `codfish/semantic-release` GitHub Action was identified as a supply-chain
risk: the upstream maintainer's account was compromised and a malicious version was
briefly published. NRK teams consuming that action were exposed.

**semrel** replaces it with a first-party Go CLI that:

- Has no external JavaScript runtime dependency
- Uses only `GITHUB_TOKEN` — no external secrets or service accounts
- Is built from source in CI before use (no pinned third-party action required)
- Publishes a cosign-signed container image for teams that prefer the container path

---

## Package structure

```
github.com/nrkno/semrel
│
├── cmd/semrel/           Entry point — wires concrete clients, injects into CLI
│
└── internal/
    ├── cli/              Cobra subcommand definitions; accepts interface-injected clients
    ├── conventional/     Conventional commit parser (type, scope, description, breaking)
    ├── env/              Reads and validates GitHub Actions environment variables
    ├── git/              go-git wrapper: FindLatestAnnotatedTag, ListCommitsSinceTag,
    │                     CreateAnnotatedTag, PushTag (BasicAuth over HTTPS)
    ├── github/           google/go-github REST client wrapper: GetReleaseByTag,
    │                     CreateRelease (with 422→re-GET idempotency), ListPRsForCommit,
    │                     SearchPRsForCommit, PostPRComment, FindPRComment
    ├── notes/            Markdown release notes generator (groups commits by type)
    ├── output/           Writes key=value pairs to the GITHUB_OUTPUT file
    └── semver/           Version parsing, bump-type detection (major/minor/patch/none),
                          next-version arithmetic
```

### Dependency direction

```
cmd/semrel → internal/cli
internal/cli → conventional, env, git, github, notes, output, semver
internal/notes → conventional
internal/github → (go-github, oauth2)
internal/git → (go-git)
internal/semver → (Masterminds/semver)
```

No internal package imports `internal/cli`. There are no circular imports.

---

## Design decisions

### go-git vs git binary

go-git is used instead of shelling out to the `git` binary. This eliminates a runtime
dependency on a git installation and makes the distroless container viable — the
`gcr.io/distroless/static-debian12:nonroot` base image has no shell or git binary.

Trade-off: go-git's annotated-tag and push APIs are lower-level than the git CLI.
The `internal/git` package wraps this complexity behind a narrow interface
(`GitClient`) so the rest of the codebase is insulated from go-git specifics.

### Distroless container

The final stage uses `gcr.io/distroless/static-debian12:nonroot`. This image:

- Contains only the CA certificate bundle (required for HTTPS calls to the GitHub API)
- Runs as a non-root user (`nonroot`, uid 65532)
- Has no shell, package manager, or other attack surface

The Go binary is compiled with `CGO_ENABLED=0` to produce a fully static binary.

### GITHUB_TOKEN only

semrel intentionally accepts only `GITHUB_TOKEN`. There is no support for PATs,
GitHub App JWTs, or SSH keys. This keeps the permission model simple: the token
is scoped to the repository and expires after the workflow run.

Git push authentication uses HTTP Basic Auth with username `x-access-token` and
password `GITHUB_TOKEN` — the [documented GitHub pattern](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github#authenticating-with-the-api).

### Idempotency ladder

The `release` subcommand applies a three-rung idempotency check before creating
anything. This prevents double-releases when a workflow is re-run:

1. **Release exists** for the computed tag → write output fields, exit 0 (no-op)
2. **Tag exists, no release** → create GitHub Release from the existing tag SHA;
   if the existing tag SHA ≠ HEAD SHA, exit 1 (conflict — a different commit has
   already been tagged with this version)
3. **Neither exists** → create annotated tag, push, create GitHub Release

Rung 2 includes a `422 already_exists` retry: if `CreateRelease` returns 422,
semrel calls `GetReleaseByTag` to retrieve the release that was created concurrently
and proceeds as if rung 1 had matched.

### SHA comparison

The tag SHA check in rung 2 compares `tag.TargetSHA()` (the commit a annotated tag
points to) against the HEAD commit SHA from `ListCommitsSinceTag(nil)`. A mismatch
means a different commit was tagged with the same calculated version, which is a
genuine conflict (exit 1). This guards against two parallel release runs computing
the same next version for different commits.

### Shallow clone requirement

semrel requires a full clone (`fetch-depth: 0`). The `release`, `lint`, and `notes`
subcommands all walk commit history back to the previous annotated tag. A shallow
clone truncates that history, causing `FindLatestAnnotatedTag` to return `nil` and
producing incorrect version calculations.

The startup code in `internal/git` detects a shallow repository and exits with
code 2 (system error) to surface the misconfiguration clearly rather than silently
producing a wrong result.

---

## Build and container pipeline

```
push to main
  └─▶ release job
        ├─ go build -o semrel ./cmd/semrel   (builds from source — no bootstrap dep)
        ├─ ./semrel release                  (creates tag + GitHub Release)
        └─ if released: publish-image job
              ├─ docker build (multi-stage, distroless)
              ├─ docker push  (ghcr.io/nrkno/github-action-sematic-release:<tag>)
              └─ cosign sign  (keyless, OIDC from GitHub Actions)
```

The release job builds semrel from source on every push to `main`. This means the
self-release pipeline never depends on a previously published container — avoiding
the bootstrap problem that affected the upstream action.
