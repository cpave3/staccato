# Sync Command Specification

## Overview

The `st sync` command performs a full synchronization workflow: fetch from remote, detect branches merged on the remote, remove merged branches from the stack graph, restack remaining branches, and push updated branches.

---

### Requirement: Remote Prerequisite

The sync command SHALL require a configured git remote. If no remote is configured, the command MUST return an error.

#### Scenario: No remote configured
- **WHEN** the repository has no git remote
- **THEN** the command MUST return an error with the message "no remote configured"

---

### Requirement: Fetch with Prune

The sync command SHALL fetch from origin with pruning as its first operation. This ensures that deleted remote tracking branches are cleaned up locally before any merge detection occurs.

#### Scenario: Successful fetch
- **WHEN** `st sync` is invoked
- **THEN** the command MUST run `git fetch origin --prune`
- **AND** the result MUST indicate that fetching was performed

#### Scenario: Fetch failure
- **WHEN** `st sync` is invoked and the fetch operation fails
- **THEN** the command MUST return an error with the message "fetch failed: <cause>"
- **AND** no further sync steps SHALL be executed

---

### Requirement: Graph Reconciliation for Shared Storage

When using shared graph storage (`refs/staccato/graph`), the sync command SHALL reconcile the local graph with the remote graph after fetching.

#### Scenario: Remote graph exists after fetch
- **WHEN** `st sync` is invoked and the shared graph ref exists
- **THEN** the command MUST read the remote graph blob from the shared ref
- **AND** the command MUST perform a union merge of local and remote graphs

#### Scenario: Union merge keeps remote branches that exist locally
- **WHEN** reconciling local and remote graphs
- **THEN** branches present in the remote graph MUST be included in the result only if the branch exists locally
- **AND** branches present only in the local graph MUST be included if the branch exists locally

#### Scenario: Local-ahead branch wins during reconciliation
- **WHEN** a branch exists in both local and remote graphs with different HeadSHA values
- **AND** the remote HeadSHA is an ancestor of the local HeadSHA
- **THEN** the reconciled graph MUST use the local HeadSHA for that branch

#### Scenario: Remote HeadSHA preserved when local is not ahead
- **WHEN** a branch exists in both local and remote graphs with different HeadSHA values
- **AND** the remote HeadSHA is NOT an ancestor of the local HeadSHA
- **THEN** the reconciled graph MUST use the remote HeadSHA for that branch

#### Scenario: Remote graph is malformed
- **WHEN** the shared graph ref exists but contains invalid JSON
- **THEN** the reconciliation MUST silently skip without modifying the local graph

---

### Requirement: Trunk Fast-Forward

The sync command SHALL fast-forward the root (trunk) branch to match its remote counterpart.

#### Scenario: Currently on trunk branch
- **WHEN** the user is currently on the trunk branch
- **AND** the trunk has a remote counterpart
- **THEN** the command MUST perform a fast-forward-only merge (`git merge --ff-only origin/<trunk>`)

#### Scenario: Currently on a non-trunk branch
- **WHEN** the user is on a branch other than trunk
- **AND** the trunk has a remote counterpart
- **THEN** the command MUST fast-forward the trunk branch ref directly using `git update-ref` without checking it out

#### Scenario: Remote trunk does not exist
- **WHEN** the trunk branch has no remote counterpart on origin
- **THEN** the command MUST skip the fast-forward step
- **AND** the result MUST NOT indicate that the trunk was updated

---

### Requirement: Merged Branch Detection

The sync command SHALL detect stack branches that have been merged into the remote trunk.

#### Scenario: Branch is ancestor of remote trunk
- **WHEN** a stack branch's HEAD commit is an ancestor of `origin/<trunk>`
- **THEN** the branch MUST be reported as merged

#### Scenario: Branch has empty diff against remote trunk and no remote tracking branch
- **WHEN** a stack branch is NOT an ancestor of `origin/<trunk>`
- **AND** the branch does not exist on the remote
- **AND** the diff between `origin/<trunk>` and the branch is empty
- **THEN** the branch MUST be reported as merged (squash-merge detection)

#### Scenario: Branch with no changes (HEAD equals BaseSHA) is skipped
- **WHEN** a stack branch's actual HEAD SHA equals its recorded BaseSHA in the graph
- **THEN** the branch MUST NOT be considered for merge detection
- **AND** the branch MUST NOT be reported as merged

#### Scenario: Branch still has differences from trunk
- **WHEN** a stack branch is not an ancestor of `origin/<trunk>`
- **AND** the diff between the branch and `origin/<trunk>` is non-empty
- **THEN** the branch MUST NOT be reported as merged

#### Scenario: Detection order follows topological sort
- **WHEN** detecting merged branches
- **THEN** branches MUST be evaluated in topological order starting from trunk

---

### Requirement: Merged Branch Removal

After detecting merged branches, the sync command SHALL remove them from the graph and reparent their children.

#### Scenario: Merged branch with children
- **WHEN** a merged branch has child branches in the graph
- **THEN** the children MUST be reparented to the merged branch's parent
- **AND** the merged branch MUST be removed from the graph
- **AND** the local git branch MUST be force-deleted

#### Scenario: Currently checked out branch is merged
- **WHEN** the user's current branch is detected as merged
- **THEN** the command MUST check out the trunk branch before deleting the merged branch
- **AND** the original branch tracking MUST be updated to trunk for subsequent operations

#### Scenario: Graph persistence after removal
- **WHEN** one or more branches are removed as merged
- **THEN** the graph MUST be saved to storage (local file or shared ref)

#### Scenario: No branches merged
- **WHEN** no branches are detected as merged
- **THEN** the graph MUST NOT be saved at this step

---

### Requirement: Stash Uncommitted Changes on Merged Branch

The sync command SHALL preserve uncommitted work when the current branch is detected as merged.

#### Scenario: Uncommitted changes on merged branch
- **WHEN** the user's current branch is detected as merged
- **AND** the working tree has uncommitted changes
- **THEN** the command MUST stash the changes with the message "st-sync: changes from merged branch <branch>"
- **AND** the result MUST record the branch name in StashedFromBranch
- **AND** the command MUST then check out trunk

#### Scenario: No uncommitted changes on merged branch
- **WHEN** the user's current branch is detected as merged
- **AND** the working tree has no uncommitted changes
- **THEN** the command MUST NOT create a stash
- **AND** StashedFromBranch MUST remain empty

---

### Requirement: Restack Remaining Branches

After removing merged branches, the sync command SHALL restack all remaining branches in the graph.

#### Scenario: Branches remain after merge removal
- **WHEN** the graph still contains branches after merge removal
- **THEN** the command MUST create automatic backups and restack using topological order
- **AND** the graph MUST be saved after restacking

#### Scenario: Restack conflict
- **WHEN** a rebase conflict occurs during restacking
- **THEN** the command MUST save the graph
- **AND** the command MUST return an error instructing the user to resolve and run `st continue`
- **AND** the result MUST indicate the conflicting branch in ConflictsAt

#### Scenario: No branches remain
- **WHEN** all branches were removed as merged (empty graph)
- **THEN** the restack step MUST be skipped

---

### Requirement: Push with Upstream Tracking

The sync command SHALL push remaining stack branches to the remote with upstream tracking and force-with-lease.

#### Scenario: Successful push
- **WHEN** restacking completes without conflicts
- **AND** the `--down` flag is not set
- **THEN** the command MUST push each non-trunk branch in the current lineage using `git push -u origin <branch> --force-with-lease`
- **AND** each successfully pushed branch MUST be recorded in PushedBranches

#### Scenario: Push failure for a branch
- **WHEN** pushing a specific branch fails
- **THEN** that branch MUST NOT appear in PushedBranches
- **AND** the sync command MUST continue pushing remaining branches (non-fatal)

#### Scenario: Trunk branch excluded from push
- **WHEN** pushing branches in the lineage
- **THEN** the trunk/root branch MUST be excluded from the push list

#### Scenario: Shared graph ref push
- **WHEN** the `--down` flag is not set
- **AND** the shared graph ref exists
- **THEN** the command MUST push the shared graph ref to the remote

---

### Requirement: Down-Only Mode

The `--down` flag SHALL restrict sync to only pulling changes (fetch, fast-forward, detect merged, restack) without pushing.

#### Scenario: Down-only skips push
- **WHEN** `st sync --down` is invoked
- **THEN** the command MUST fetch, fast-forward trunk, detect and remove merged branches, and restack
- **AND** the command MUST NOT push any branches
- **AND** the command MUST NOT push the shared graph ref

#### Scenario: Down-only dry run
- **WHEN** `st sync --down --dry-run` is invoked
- **THEN** the dry-run output MUST NOT list any branches that would be pushed

---

### Requirement: Dry-Run Mode

The `--dry-run` flag SHALL perform fetch and detection but make no destructive changes, reporting what would happen.

#### Scenario: Dry-run reports trunk fast-forward
- **WHEN** `st sync --dry-run` is invoked
- **AND** the trunk is behind its remote counterpart
- **THEN** the output MUST include "Would fast-forward '<trunk>'"

#### Scenario: Dry-run reports merged branches
- **WHEN** `st sync --dry-run` is invoked
- **AND** merged branches are detected
- **THEN** the output MUST include "Would remove merged branch: <name>" for each merged branch
- **AND** the output MUST include "Would restack remaining branches"

#### Scenario: Dry-run reports branches that would be pushed
- **WHEN** `st sync --dry-run` is invoked
- **AND** there are non-trunk branches in the current lineage
- **THEN** the output MUST include "Would push: <name>" for each branch

#### Scenario: Dry-run with nothing to do
- **WHEN** `st sync --dry-run` is invoked
- **AND** no branches would be pushed and no branches are merged
- **THEN** the output MUST include "Nothing to do."

#### Scenario: Dry-run does not modify branches or graph
- **WHEN** `st sync --dry-run` is invoked
- **THEN** no branches SHALL be deleted, rebased, or pushed
- **AND** the graph SHALL NOT be modified or saved
- **AND** the trunk SHALL NOT be fast-forwarded

---

### Requirement: Original Branch Restoration

After sync completes, the command SHALL attempt to restore the user to their original branch.

#### Scenario: Original branch still exists
- **WHEN** the sync operation completes
- **AND** the original branch still exists (was not merged and deleted)
- **THEN** the command MUST check out the original branch

#### Scenario: Original branch was merged and deleted
- **WHEN** the original branch was detected as merged and deleted during sync
- **THEN** the command MUST remain on the trunk branch (the checkout target set during merged branch removal)

---

### Requirement: Sync Summary Output

The sync command SHALL display a summary of operations performed.

#### Scenario: Full sync with activity
- **WHEN** sync completes with merged branches, restacked branches, and pushed branches
- **THEN** the command MUST display a summary including the count of merged, restacked, and pushed branches

#### Scenario: Merged branch stash warning
- **WHEN** uncommitted changes were stashed from a merged branch
- **THEN** the command MUST display a warning: "Stashing uncommitted changes from merged branch '<branch>'"
