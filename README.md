# semrel

[![Go version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![CI](https://github.com/nrkno/github-action-sematic-release/actions/workflows/ci.yml/badge.svg)](https://github.com/nrkno/github-action-sematic-release/actions/workflows/ci.yml)

**semrel** is NRK's in-house semantic-release CLI — a single Go binary that enforces
[Conventional Commits](https://www.conventionalcommits.org/), computes the next semver,
creates GitHub Releases, and notifies merged PRs. It replaces the supply-chain-compromised
`codfish/semantic-release` Action.

The container image is published to GHCR and signed with
[cosign keyless](https://docs.sigstore.dev/cosign/signing/overview/) on every release.

---

## Security

> ⚠️ **Always pin by commit SHA, not tag.**

Git tags are mutable. An attacker with push access to this repository could
force-push a tag to a malicious commit. Pinning by commit SHA gives you an
immutable reference that cannot be silently changed.

```yaml
# ✅ Secure — commit SHA is immutable
- uses: nrkno/github-action-sematic-release@<COMMIT_SHA>
  with:
    subcommand: release
    token: ${{ secrets.GITHUB_TOKEN }}

# ⚠️ Less secure — git tags can be force-pushed
- uses: nrkno/github-action-sematic-release@v1.2.3
  with:
    subcommand: release
    token: ${{ secrets.GITHUB_TOKEN }}
```

The action image is tagged by release version in `action.yml`. Supply-chain
integrity is provided by cosign keyless signatures — verify with:

```bash
cosign verify ghcr.io/nrkno/github-action-sematic-release:vX.Y.Z \
  --certificate-identity-regexp="https://github.com/nrkno/github-action-sematic-release" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

Always pin consuming workflows to a specific release tag or commit SHA
rather than `@main`.

Each release prints its commit SHA in the workflow summary. You can also find
it on the [releases page](https://github.com/nrkno/github-action-sematic-release/releases).

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
          go-version: '1.25'
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
          go-version: '1.25'
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
          go-version: '1.25'
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
| `lint`     | Validate conventional commits in the current event's relevant range. Lint rules can be overridden via `.semrelrc.yml` at the repo root — see [docs/configuration.md](/docs/configuration.md#lint-configuration-file-semrelrcyml). |
| `release`  | Compute next semver, create annotated tag, push, create GitHub Release  |
| `notify`   | Post a deduplicated release comment on the merged PR                    |
| `notes`    | Generate Markdown release notes from commits and linked PRs              |

---

## Inputs & Outputs

<!-- autodoc start -->
<!-- autodoc end -->

---

## Configuration (`.semrelrc.yml`)

Place a `.semrelrc.yml` at your repo root to customise semrel behaviour.
All fields are optional — absent fields retain the defaults shown.

```yaml
# .semrelrc.yml
lint:
  rules:
    capital-first-letter: true  # fail UPPER descriptions
    require-scope: false

bump-rules:                     # which type triggers which bump
  breaking-change: major        # default — customise freely
  feat: minor
  fix: patch
  # chore: patch               # example: add patch bump for chore

release-branches: [main, master]  # branches that trigger a release

tag-prefix: "v"               # tag format: "v1.2.3" (set "" for bare)

commit-types:
  extra-types: []             # add custom types to the built-in 10
  allowed-types: []           # replace built-in set entirely (non-empty)

initial-version: "0.0.0"     # bootstrap base version (bump applied on top)
```

See [docs/configuration.md](/docs/configuration.md) for the full field reference.

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
