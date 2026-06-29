# semrel

[![Go version](https://img.shields.io/badge/go-1.23-00ADD8?logo=go)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![CI](https://github.com/nrkno/github-action-sematic-release/actions/workflows/ci.yml/badge.svg)](https://github.com/nrkno/github-action-sematic-release/actions/workflows/ci.yml)

**semrel** is NRK's in-house semantic-release CLI — a single Go binary that enforces
[Conventional Commits](https://www.conventionalcommits.org/), computes the next semver,
creates GitHub Releases, and notifies merged PRs. It replaces the supply-chain-compromised
`codfish/semantic-release` Action.

The container image is published to GHCR and signed with
[cosign keyless](https://docs.sigstore.dev/cosign/signing/overview/) on every release.

---

## Quick start

The minimal release workflow — drop this in `.github/workflows/release.yml`:

```yaml
on:
  push:
    branches: [main]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0        # ⚠️  required — see warning below
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: go build -o semrel github.com/nrkno/semrel/cmd/semrel
      - id: semrel
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: ./semrel release
```

> **⚠️ `fetch-depth: 0` is required.**
> semrel reads the full commit and tag history to determine the previous version and build
> the commit range. A shallow clone (`fetch-depth: 1`, the GitHub Actions default) causes
> semrel to exit with code `2` (system error). Always set `fetch-depth: 0`.

---

## Full workflow

The complete three-job workflow used by this repo itself:

```yaml
name: Release

on:
  push:
    branches: [main]

permissions:
  contents: write
  packages: write
  id-token: write

concurrency:
  group: release
  cancel-in-progress: false

jobs:
  release:
    runs-on: ubuntu-latest
    outputs:
      version:  ${{ steps.semrel.outputs.version }}
      tag:      ${{ steps.semrel.outputs.tag }}
      released: ${{ steps.semrel.outputs.released }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: go build -o semrel ./cmd/semrel
      - id: semrel
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: ./semrel release
      - run: ./semrel notes >> $GITHUB_STEP_SUMMARY
        if: steps.semrel.outputs.released == 'true'

  publish-image:
    needs: release
    if: needs.release.outputs.released == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v2
      - uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v4
        with:
          context: .
          push: true
          tags: ghcr.io/${{ github.repository }}:${{ needs.release.outputs.tag }}
          build-args: |
            VERSION=${{ needs.release.outputs.version }}
            COMMIT=${{ github.sha }}
            DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
      - uses: sigstore/cosign-installer@v3
      - run: cosign sign --yes ghcr.io/${{ github.repository }}:${{ needs.release.outputs.tag }}
        env:
          COSIGN_EXPERIMENTAL: 1

  notify:
    needs: [release, publish-image]
    if: needs.release.outputs.released == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: go build -o semrel ./cmd/semrel
      - env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SEMREL_RELEASED: ${{ needs.release.outputs.released }}
          SEMREL_VERSION: ${{ needs.release.outputs.version }}
          SEMREL_TAG:     ${{ needs.release.outputs.tag }}
        run: ./semrel notify
```

### Required permissions

| Permission      | Reason                                     |
| --------------- | ------------------------------------------ |
| `contents: write` | Create tags and GitHub Releases          |
| `packages: write` | Push container image to GHCR             |
| `id-token: write` | Cosign keyless signing (OIDC token)      |

---

## Subcommands

| Subcommand | Description                                                              |
| ---------- | ------------------------------------------------------------------------ |
| `lint`     | Validate conventional commits in the current event's relevant range     |
| `release`  | Compute next semver, create annotated tag, push, create GitHub Release  |
| `notify`   | Post a deduplicated release comment on the merged PR                    |
| `notes`    | Generate Markdown release notes from commits and linked PRs              |

---

## Documentation

Full documentation is in [`docs/`](/docs/index.md):

- [Architecture](/docs/architecture.md) — design decisions and package structure
- [Configuration](/docs/configuration.md) — environment variables and flags
- [API Reference](/docs/api-reference.md) — subcommand flags, exit codes, output fields
- [Playbook](/docs/playbook.md) — step-by-step runbooks

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
