## Context

`st` is built from source today (`task build`, or `go install`), has no version, and keeps a stack's graph in one clone: `.git/stack/graph.json`, or a blob at `refs/staccato/graphs/<hash of email>` in shared mode. Shared mode's `ReconcileGraphs` merges two graphs of one person: the one with the higher `Version` (a counter raised on every save) is the base and the other's branches are added when they exist locally.

Legato, an orchestrator for coding agents, is adding worker instances: headless machines that take a task from a person's main machine, work on it in a fresh clone, and return it. Workers run Legato from its GitHub releases and update themselves. An agent on a worker should be able to split its work into a stack with `st` and push it; the person should then carry on with that stack on their machine. Two things stand in the way: getting `st` onto the worker and keeping it current, and getting the graph back. Pushes from the worker go through Legato's credential proxy, which can refuse a push or hold it for the person's approval, and `st sync` drops such failures silently today.

Legato's release workflow (`.github/workflows/release.yml`, `scripts/next-version.sh`) and updater (`internal/engine/selfupdate`, about 200 lines of standard library) are the model. Staccato cannot import the updater: it is under Legato's `internal/`.

## Goals / Non-Goals

**Goals:**
- A worker gets `st` with one `curl`, and keeps it current with `st update`.
- A stack built in one clone can be merged into another clone's graph, safely when both have touched the same branches.
- An agent sees when `st sync` failed to push a branch.

**Non-Goals:**
- Moving the graph through the remote (shared mode's ref). Legato carries the document with the task's return; other tools can carry it however they like.
- Changing shared mode's reconciliation. `st sync`'s `ReconcileGraphs` is left as it is.
- Restacking or pushing as part of an import.
- Automatic update checks, Homebrew taps, Windows builds, notarized macOS binaries.
- Bundling `st` into Legato.

## Decisions

### Releases copy Legato's workflow

Same shape: run by hand on `main` with a bump, next version from the newest `vX.Y.Z` tag, vet and test, build with `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X github.com/cpave3/staccato/pkg/version.stamped=<v>"`, check the binary reports its version, attest provenance, `gh release create` with generated notes. Taskfile gets `build:release` (one platform) and `release:dist` (all three, plus `SHA256SUMS`). The tag is the version; nothing is committed back, so `main` never carries a version string to conflict on.

- *Alternative: GoReleaser.* It does all of this from one file, but adds a tool to learn and a second way of doing what Legato already does. The person maintains both repositories; one pattern is cheaper.

Platforms are linux/amd64, linux/arm64 (workers and Raspberry Pi–class boxes) and darwin/arm64. `build.yml` keeps darwin/amd64 as a compile check, and adds linux/arm64.

### `pkg/version` and build information

`pkg/version` holds `stamped` (set by `-ldflags`), `String()` and `IsRelease()`. When nothing is stamped, `String()` falls back to `debug.ReadBuildInfo().Main.Version` when it is a clean `vX.Y.Z` (a `go install …@v0.5.0` build), else `dev`. Only a stamped version is a release: a `go install` binary lives where `go install` manages it, and `st update` writing over it would surprise.

### `pkg/selfupdate` is a copy, adapted

Copied from Legato's `internal/engine/selfupdate` with the repository and asset prefix as fields (`Client{Repo: "cpave3/staccato", Asset: "st"}`), tests included. Keeping the code the same shape means a fix in one is easy to carry to the other.

- *Alternative: a shared module both import.* Better if a third tool needs it; for two, a third repository to version and release is more work than the copy.

The API base address is a field on the client. E2E tests set it through `ST_RELEASES_API`, read only by `st update`; someone who can set the environment of `st` can already replace the binary, so this opens nothing new.

### The graph document is not `graph.json`

`graph.json` is storage: it has a save counter, no format number, and a map. The document is an exchange format with `format`, `kind`, `root`, and an ordered list, so it can be diffed, read by other tools (Legato will store it with a task's return), and changed later without breaking older readers silently. `pkg/transfer` owns the document type, `Export` and `Import`, using `pkg/graph`, `pkg/git` and `pkg/restack` (`GetStackBranches`, `GetAncestors`).

### The merge trusts git, branch by branch

For each branch the question is "which side's commits does the other already have?", which git answers exactly (`merge-base --is-ancestor`); the graph entry follows whichever head wins. A save counter cannot answer it: two machines' counters are unrelated, so "higher wins" would let a busy local graph overwrite a worker's restack, or the reverse. Diverged branches are left alone and reported, because only the person can say which commits they want; `st` has no business rewriting a branch they may have work on.

- *Alternative: reuse `ReconcileGraphs`.* Its rule is per graph, not per branch, and its spec describes ancestor checks the code does not make. Fixing it would change `st sync` for shared-mode users; this change leaves it alone.
- *Alternative: replace the local graph's entries wholesale.* Simple, but loses the person's local work on any branch they touched.

The import is decided in full before anything changes: fetch, check every commit exists, decide every outcome, cascade `skipped` to the children of branches that will not be present, check for cycles, then one `update-ref --stdin` transaction for created and moved branches, then one graph save. A failure before the transaction changes nothing; the transaction is atomic; the graph save comes last, so a crash between them leaves branches without graph entries, which `st attach` or a second import repairs.

The checked-out branch is the one exception to the transaction: moving it with `update-ref` would leave the working tree and index on the old commit. It is fast-forwarded with `git merge --ff-only` when clean, and skipped when not.

### Export checks the remote-tracking branches, not the remote

`git ls-remote` would be exact but needs the network and credentials, and on a Legato worker every remote read goes through the credential broker. Comparing with `origin/<name>` after `st sync` (which fetches and pushes) is right in the case that matters, and `--allow-unpushed` covers the rest. The document records git's tip, not the graph's `HeadSHA`, because the tip is what was pushed.

### `st sync` reports refused pushes

Push errors are collected in `SyncResult.FailedPushes` (`{Branch, Reason}`) instead of dropped. The reason is git's last non-progress line on standard error, which for a push Legato holds is `! [remote rejected] <ref> (pending ap-…)`, enough for an agent to know to wait. The exit status becomes non-zero, after the other pushes, so agents and scripts notice without parsing output.

## Risks / Trade-offs

- [Scripts that ignore `st sync`'s output and expect exit 0 now fail on a refused push] → Called out as breaking in the release notes; the old behaviour hid a failure.
- [A worker's `st` and the person's differ by a version] → The document has a `format`; an older `st` refuses a newer format with a clear message instead of misreading it. Format 1 is kept for anything an older reader handles correctly.
- [Unauthenticated GitHub API is limited to 60 requests an hour per address] → `st update` is run by hand or by Legato's updater, not on every command.
- [macOS binaries are not notarized] → Same as Legato: Go's linker signs darwin/arm64 ad hoc, which runs when downloaded with `curl` (no quarantine attribute). A browser download needs `xattr -d com.apple.quarantine`; the README says so.
- [A crash between the branch transaction and the graph save] → Branches exist without entries; re-running the import adds them (`L = D` is `updated`).
- [Fetching every document branch by name] → One `git fetch <remote> <refspecs…>` call; a branch deleted on the remote fails the fetch and the import says which.

## Migration Plan

1. Land `pkg/version`, `st version`, the Taskfile tasks and the release workflow; cut `v0.1.0`.
2. Land `st update`; cut a release to exercise it against the previous one.
3. Land export/import and the `st sync` change; cut a minor release.
4. README: install with `curl -fsSLo ~/.local/bin/st https://github.com/cpave3/staccato/releases/latest/download/st-linux-amd64 && chmod +x ~/.local/bin/st`.

Rollback: `st.prev` is the previous binary; renaming it back undoes an update.

## Open Questions

- Should `st graph import` restack the imported branches onto the local trunk when it has moved? Left out: the person can run `st restack`, and an import that rebases is harder to undo.
- Should the agent primer (`st help`) tell agents to export after their last `st sync`, or should Legato run the export itself at return time? This change makes both possible; Legato's design decides.
