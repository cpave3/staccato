## ADDED Requirements

### Requirement: Release Workflow

The repository SHALL have a GitHub Actions workflow, `release`, started by hand with a `bump` input of `patch`, `minor` or `major`. It SHALL run only on `main`, and only one release SHALL run at a time.

#### Scenario: Releasing from main
- **WHEN** the workflow is started on `main` with `bump=minor`
- **THEN** it MUST compute the next version, test, build, and publish a release named and tagged with that version

#### Scenario: Started on another branch
- **WHEN** the workflow is started on a branch other than `main`
- **THEN** it MUST NOT build or publish anything

### Requirement: Next Version

The next version SHALL be the newest tag of the form `vX.Y.Z` (numbers compared as numbers; prerelease and other tags ignored) with the chosen part bumped and the parts after it reset to zero. With no such tag, the base SHALL be `v0.0.0`.

#### Scenario: First release
- **WHEN** the repository has no `vX.Y.Z` tag and the bump is `minor`
- **THEN** the next version MUST be `v0.1.0`

#### Scenario: Numeric comparison
- **WHEN** the tags are `v0.9.0` and `v0.10.0` and the bump is `patch`
- **THEN** the next version MUST be `v0.10.1`

#### Scenario: Other tags ignored
- **WHEN** the tags are `v0.2.0` and `v0.3.0-rc1`
- **THEN** the base MUST be `v0.2.0`

### Requirement: Release Contents

A release SHALL be built only after `go vet ./...` and the test suite pass. It SHALL contain `st` built for `linux/amd64`, `linux/arm64` and `darwin/arm64` with `CGO_ENABLED=0`, `-trimpath`, stripped, and with the version stamped in, named `st-<os>-<arch>` with no version in the name, and a `SHA256SUMS` file listing each binary. The linux/amd64 binary MUST report the version it was built as before anything is published. Each binary SHALL have a build provenance attestation. The release notes SHALL be generated from the commits since the previous release, and nothing SHALL be committed back to `main`.

#### Scenario: Latest download address
- **WHEN** a release has been published
- **THEN** `https://github.com/cpave3/staccato/releases/latest/download/st-linux-amd64` MUST download that release's linux/amd64 binary

#### Scenario: Stamped version mismatch
- **WHEN** the built linux/amd64 binary's `st version` does not print the version being released
- **THEN** the workflow MUST fail without publishing

#### Scenario: Tests fail
- **WHEN** a test fails
- **THEN** no tag and no release MUST be created

### Requirement: Version Command (`st version`)

`st version` SHALL print `st <version>`, where the version is the one stamped in at build time. A build with no stamped version SHALL report the main module's version from the Go build information when it is a `vX.Y.Z` (as `go install …@vX.Y.Z` produces), and `dev` otherwise. `--json` SHALL print `{"version": "<version>", "release": <bool>}`, where `release` is true only for a stamped stable `vX.Y.Z`. The command SHALL work outside a git repository and SHALL NOT read the graph.

#### Scenario: Release build
- **WHEN** `st version` runs in a binary stamped `v0.4.2`
- **THEN** it MUST print `st v0.4.2`

#### Scenario: Built from a checkout
- **WHEN** `st version --json` runs in a binary built with `go build` from a checkout that is not exactly at a clean `vX.Y.Z` tag (Go then records a pseudo-version or `+dirty`, which is not a `vX.Y.Z`)
- **THEN** it MUST print `{"version": "dev", "release": false}`

#### Scenario: Outside a repository
- **WHEN** `st version` runs in a directory that is not a git repository
- **THEN** it MUST print the version and exit 0
