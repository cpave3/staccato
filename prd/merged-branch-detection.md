# Merged branch detection

How `st sync` decides a stack branch has been merged into trunk, and the known
gap in that detection.

## Current behavior

`DetectMergedBranches` (`pkg/sync/sync.go`) walks the graph in topological
order and applies three checks per branch:

1. **Unchanged branch** — if the branch head still equals its recorded
   `BaseSHA`, it has no commits of its own and is skipped.
2. **Regular / rebase merge** — `git merge-base --is-ancestor <branch>
   origin/<trunk>`. Catches merge commits and fast-forward or rebase merges,
   where the branch's commits are literally in trunk's history.
3. **Squash merge** — only when `origin/<branch>` no longer exists:
   `MergeAddsNoChanges("origin/"+trunk, branch)` (`pkg/git/git.go`) runs
   `git merge-tree --write-tree origin/<trunk> <branch>` and treats the branch
   as merged when merging it into trunk would change nothing. This works even
   when trunk has advanced past the squash commit (other PRs landed, or
   multiple branches from the same stack were squash-merged), which a plain
   `git diff origin/<trunk>..<branch>` emptiness check did not
   (see `TestSyncDetectsStackedSquashMerges`).

A branch detected as merged is removed from the graph, its children are
reparented to its parent, and the **local branch is deleted**.

## Known gap: squash merge with surviving remote branch

The squash check is gated on `origin/<branch>` being gone. GitHub deletes head
branches on merge when "Automatically delete head branches" is enabled — but
when it isn't (or the merge happened on a forge without that behavior), a
squash-merged branch keeps its remote ref and is never detected. Sync then
tries to restack it, replaying already-merged commits onto trunk and usually
conflicting.

The gate exists for safety: detection triggers local branch deletion, and the
merge-tree check alone could false-positive on a branch whose changes happen
to already exist in trunk (e.g. someone independently landed the same change).
Requiring the remote branch to be deleted keeps a strong, cheap signal that a
merge actually happened.

## Proposed solution: query PR state via `gh`

Add a forge-aware stage to detection, consulted when the pure-git checks are
inconclusive (remote branch still exists, but merge-tree says the branch adds
no changes):

1. If the `gh` CLI is available and the remote is GitHub, run
   `gh pr view <branch> --json state,mergedAt` (or
   `gh pr list --head <branch> --state merged`).
2. A PR in `MERGED` state confirms the branch is merged → treat it like any
   other merged branch, and additionally delete the stale remote ref
   (`git push origin --delete <branch>`) so subsequent syncs don't re-query.
3. If `gh` is missing, unauthenticated, or the remote isn't GitHub, keep
   current behavior (skip the branch) and surface a hint in verbose output,
   e.g. "branch <x> looks squash-merged but origin/<x> still exists; merge
   confirmation unavailable".

Design notes:

- Keep it opt-in-by-availability (no hard dependency on `gh`); all existing
  pure-git paths remain the defaults.
- Only invoke the API for candidate branches that already pass the merge-tree
  check, so sync stays fast and offline-friendly in the common case.
- `pkg/forge/github.go` already wraps the `gh` CLI (availability check, PR
  lookup by head branch); extend that package rather than adding a new GitHub
  client.
