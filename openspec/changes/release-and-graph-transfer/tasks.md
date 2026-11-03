## 1. Version

- [x] 1.1 Test `pkg/version`: stamped `vX.Y.Z` is a release; unstamped falls back to a clean build-info `vX.Y.Z` (not a release), else `dev`; a pseudo-version or `(devel)` gives `dev`
- [x] 1.2 Implement `pkg/version` (`stamped`, `String`, `IsRelease`, `Newer`, with the build-info reader injectable for tests)
- [x] 1.3 E2E test `st version` and `st version --json` (dev build, a build with `-ldflags -X`, outside a git repository)
- [x] 1.4 Add `cmd/st/version.go` with help that passes `TestEveryCommandIsDocumented`

## 2. Release workflow

- [x] 2.1 Add `scripts/next-version.sh` (from Legato's) and a test for it: no tags, numeric comparison, prerelease and other tags ignored, each bump
- [x] 2.2 Taskfile: `build:release` (`GOOS`, `GOARCH`, `VERSION`; `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X …/pkg/version.stamped=<v>"`) and `release:dist` (three platforms into `dist/st-<os>-<arch>`, then `dist/SHA256SUMS`)
- [x] 2.3 Add `.github/workflows/release.yml`: dispatch with `bump`, `main` only, one at a time; vet and test; `task release:dist`; check `dist/st-linux-amd64 version`; attest provenance; `gh release create` with generated notes
- [x] 2.4 Add linux/arm64 to `build.yml`
- [x] 2.5 Run `task release:dist VERSION=v0.0.0-test` locally and check each binary's `version` and `SHA256SUMS`

## 3. Self-update

- [x] 3.1 Copy Legato's `internal/engine/selfupdate` and its tests into `pkg/selfupdate`; make the repository, asset prefix and API base fields of `Client`; tests pass against `httptest` releases
- [x] 3.2 Tests for install: through a link, checksum mismatch, missing asset, wrong reported version, unwritable directory; each leaves the binary unchanged and no download behind
- [x] 3.3 E2E tests for `st update` against an `httptest` server named by `ST_RELEASES_API`: newer release installed and `.prev` kept; already newest; `dev` build refused without a request; `--check` and `--check --json`
- [x] 3.4 Implement `cmd/st/update.go` (`--check`, `--json`) with help and the refusal message for `dev` and `go install` builds

## 4. Graph document and export

- [x] 4.1 Tests for `pkg/transfer` document: encode/decode round trip, parents-first then name order, unknown `kind` and newer `format` refused, unknown fields ignored
- [x] 4.2 Implement the document type and ordering in `pkg/transfer`
- [x] 4.3 Tests for `Export`: current stack (bottom ancestor and descendants), `--branch`, `--all`, head from git not the graph, unpushed and behind branches named, `--allow-unpushed`, current branch not in a stack
- [x] 4.4 Implement `transfer.Export` and `cmd/st/graph_export.go` (`--branch`, `--all`, `--output`, `--remote`, `--allow-unpushed`) under the `graph` group

## 5. Import and merge

- [x] 5.1 Tests for the per-branch decision, using real repositories from `internal/testutil`: `added`, `updated`, `unchanged`, `fast-forwarded`, `kept-local` (with and without a local entry), `conflict` (with and without a local entry), `skipped` for a dirty checkout and for another worktree
- [x] 5.2 Tests for the merged-graph checks: parent kept by a local entry, `parent … was not merged` cascading up a stack, cycle fails with nothing changed
- [x] 5.3 Tests for the preconditions: different root refused, empty graph adopts the root, missing commit refused, fetch of a deleted remote branch fails naming it; each changes nothing
- [x] 5.3a Tests for document validation: an invalid or duplicate branch name, a branch named like the root, an empty parent, a short or non-hex SHA; each refused before any fetch
- [x] 5.4 Implement `transfer.Plan` (validate the document, fetch, check commits, decide, cascade, cycle check) returning outcomes without changing anything
- [x] 5.5 Implement `transfer.Apply`: one `update-ref --stdin` transaction (creates with upstream tracking where `<remote>/<name>` exists), `merge --ff-only` for the clean checked-out branch, then one graph save through the existing save path (local and shared modes)
- [x] 5.6 E2E tests for `st graph import`: a stack exported from one clone imported into a second clone of the same origin; worker built on the local stack; diverged branch; `--dry-run` changes nothing; `--json`; `--no-fetch`; `-` reads standard input; exit 1 on a conflict with the rest applied
- [x] 5.7 Implement `cmd/st/graph_import.go` (`--remote`, `--no-fetch`, `--dry-run`, `--json`) with the report and advice for conflicts

## 6. Sync push failures

- [x] 6.1 Tests in `pkg/sync`: a push refused by the remote (a `pre-receive` hook in the test origin that rejects one branch) is in `FailedPushes` with git's reason, the other branches are pushed, and a failed shared-ref push is recorded under the ref's name
- [x] 6.2 Collect push errors in `SyncResult.FailedPushes` (`{Branch, Reason}`, reason = the refused ref's status line, else the last `fatal:`/`error:`/`remote:` line that is not git's generic "failed to push some refs") instead of dropping them
- [x] 6.3 E2E tests for `st sync`: warning naming each failed branch after the summary, `failed_pushes` in `--json` (empty list when none), non-zero exit
- [x] 6.4 Add `st sync --json` (the object in the sync spec; no other standard output), print the warnings, pass `failed_pushes` to the post-sync hook, and set the exit status in `cmd/st/sync.go`

## 7. Documentation

- [x] 7.1 `st help` agent primer: moving a stack between clones (`st sync`, `st graph export`, then `st graph import` on the other clone), and that a refused push makes `st sync` fail
- [x] 7.2 README: install from a release with `curl`, `st update`, the macOS quarantine note
- [x] 7.3 AGENTS.md: `pkg/version`, `pkg/selfupdate`, `pkg/transfer`, the new commands, and the release steps
- [x] 7.4 Update `openspec/specs/graph-storage` to point to graph transfer, and the skill in `skill/` with the new commands
- [x] 7.5 `task check` passes
