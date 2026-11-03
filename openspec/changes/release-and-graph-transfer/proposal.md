## Why

Staccato is going to run where nobody builds it from source: on a Legato worker, a headless machine that takes tasks from a person's main machine, works on them in fresh clones, and hands the results back. An agent there should be able to break its work into a stack with `st` and push it, and the person should then be able to carry on with that stack on their own machine with `st`. Today `st` has no releases, so getting it onto a worker means a Go toolchain and `go install`; it cannot update itself; and a stack's graph lives in one clone's `.git/stack/graph.json` with no way to move it to another clone. Shared mode moves graphs, but only between one person's machines through a ref on the remote, and its reconciliation trusts a save counter that two machines do not share.

## What Changes

- **Releases.** A release workflow, run by hand on `main`, that tags the next `vX.Y.Z`, tests, builds `st` for linux/amd64, linux/arm64 and darwin/arm64 with the version stamped in, and publishes the binaries with `SHA256SUMS` as a GitHub release. Asset names carry no version, so `releases/latest/download/st-linux-amd64` is always the newest. Modelled on Legato's.
- **`st version`.** Prints the stamped version, or `dev` for a build from source; `--json` gives `{version, release}`.
- **`st update`.** Installs the newest stable release over the running binary: downloaded beside it, checked against `SHA256SUMS`, run to confirm its version, the old binary kept as `st.prev`. A `dev` build refuses. `--check` only reports whether a newer release exists.
- **`st graph export`.** Writes the current stack (or `--all`) as a self-describing JSON document with a format number, the trunk, and each branch's name, parent, base and head.
- **`st graph import <file>`.** Fetches the document's branches from the remote and merges the document into this clone's graph one branch at a time, trusting git over either graph: an incoming branch is taken when the local branch does not exist, matches it, or is behind it; a branch whose local copy has commits the incoming one lacks is a conflict and keeps its local entry. Local branches are never deleted. `--dry-run` and `--json` report what would change.
- **`st sync` reports failed pushes.** A branch whose push fails is named with git's reason, listed in `--json` as `failed_pushes`, and makes `st sync` exit non-zero after the other branches are pushed. **BREAKING** for scripts that relied on `st sync` exiting 0 when a push was refused.

## Capabilities

### New Capabilities
- `releases`: how a release is made and what it contains, and `st version`.
- `self-update`: `st update`, finding the newest release, verifying and installing it.
- `graph-transfer`: the export document, `st graph export`, `st graph import`, and the per-branch merge rules.

### Modified Capabilities
- `sync`: "Push with Upstream Tracking" — a failed push is reported by branch with its reason, appears in `--json`, and sets a non-zero exit status.

## Impact

- New: `cmd/st/version.go`, `cmd/st/update.go`, `cmd/st/graph_export.go`, `cmd/st/graph_import.go`; `pkg/version` (stamped version), `pkg/selfupdate` (adapted from Legato's `internal/engine/selfupdate`, which Staccato cannot import), `pkg/transfer` (document format and merge, beside `pkg/graph`).
- Changed: `pkg/sync` (collect push failures), `cmd/st/sync.go` (print them, exit status), `cmd/st/help.go` (agent primer: moving a stack between clones), `Taskfile.yml` (`build:release`, `release:dist`), `.github/workflows/build.yml` (linux/arm64), README (install from a release).
- New files: `.github/workflows/release.yml`, `scripts/next-version.sh`.
- No new dependencies; the binary stays free of C (`CGO_ENABLED=0`).
- Legato is the first consumer: a worker installs `st` from a release, and a return carries an exported graph that the main machine imports. Legato's side is designed in Legato; this change gives it stable commands and a versioned document to call.
