# Restack Specification

## Overview

The restack system rebases branches in a stack onto their parents in topological order, with automatic backup creation, conflict handling, and the ability to resume after conflict resolution.

---

### Requirement: Restack Lineage in Topological Order

The `st restack` command SHALL rebase all branches in the current branch's lineage onto their respective parents, processing branches in topological order (parents before children). The current branch MUST be part of the stack; if it is not, the command SHALL return an error. The graph state SHALL be saved after restack completes, regardless of success or failure.

#### Scenario: Restack a linear stack

- **WHEN** the user runs `st restack` from the tip of a three-branch linear stack (root -> A -> B -> C) where the root has new commits
- **THEN** branch A is rebased onto root, then B onto A, then C onto B
- **AND** each branch's BaseSHA and HeadSHA are updated in the graph after its rebase succeeds

#### Scenario: Restack when current branch is not in stack

- **WHEN** the user runs `st restack` while on a branch that is not tracked in the graph and is not the root
- **THEN** the command SHALL return an error: "current branch '<name>' is not in the stack"

#### Scenario: Restack filters to lineage branches only

- **WHEN** the user runs `st restack` from a tip branch in a stack that has multiple lineages (e.g., root -> A -> B and root -> C -> D)
- **THEN** only branches in the current branch's lineage SHALL be rebased
- **AND** branches in other lineages SHALL remain untouched

---

### Requirement: Restack To Current Branch

When the current branch is not at the tip of its lineage, `st restack` SHALL refuse to proceed unless the `--to-current` flag is provided. When `--to-current` is specified, the restack SHALL only process ancestor branches from root up to and including the current branch, excluding any descendants.

#### Scenario: Restack refuses when not at tip without flag

- **WHEN** the user runs `st restack` from a mid-stack branch that has children
- **THEN** the command SHALL print a warning: "You are not at the tip of your stack lineage"
- **AND** the command SHALL return an error instructing the user to use `--to-current` or switch to the tip

#### Scenario: Restack with --to-current from mid-stack

- **WHEN** the user runs `st restack --to-current` from branch B in a stack root -> A -> B -> C
- **THEN** only branches root, A, and B SHALL be included in the restack (ancestors only)
- **AND** branch C SHALL NOT be rebased

---

### Requirement: Automatic Backup Creation Before Restack

Before performing any destructive rebase operations, the restack engine SHALL create automatic backups of all branches in the restack set. Backups SHALL use the `backup/<branch>/<nanosecond-timestamp>` naming convention. If backup creation fails, the restack SHALL abort and return an error with the partial backup map.

#### Scenario: Backups created before rebase begins

- **WHEN** the user runs `st restack` on a stack with branches A and B
- **THEN** automatic backup refs SHALL be created for both A and B before any rebase operation starts
- **AND** the result SHALL contain a Backups map with entries for each backed-up branch

#### Scenario: Backups cleaned up on success

- **WHEN** a restack completes successfully with no conflicts
- **THEN** the automatic backups for all branches in the lineage SHALL be cleaned up
- **AND** the restack state file SHALL be cleared

#### Scenario: Backups preserved on failure

- **WHEN** a restack fails due to a non-conflict error
- **THEN** the backups SHALL be preserved
- **AND** the user SHALL be informed to run `st restore` to recover

---

### Requirement: Conflict Handling During Restack

When a rebase operation encounters a merge conflict, the restack engine SHALL stop immediately, mark the result as having conflicts, and record which branch the conflict occurred at. The `st restack` command SHALL persist restack state (the lineage being processed) to `.git/stack/restack-state.json` so that `st continue` can resume from where it left off. The graph state SHALL be saved even when conflicts occur.

#### Scenario: Conflict stops restack and saves state

- **WHEN** a rebase conflict occurs while rebasing branch B onto its parent A
- **THEN** the restack SHALL stop immediately without processing further branches
- **AND** the result's `Conflicts` field SHALL be true and `ConflictsAt` SHALL be "B"
- **AND** a restack state file SHALL be written containing the full lineage
- **AND** the graph SHALL be saved to preserve any already-updated branch metadata

#### Scenario: Conflict error message directs user to continue

- **WHEN** a conflict is detected during `st restack`
- **THEN** the command SHALL return an error: "conflict during restack - resolve and run 'st continue'"

---

### Requirement: Continue Restack After Conflict Resolution

The `st continue` command SHALL resume a paused restack operation. It MUST verify that a rebase is currently in progress; if not, it SHALL return an error. It SHALL call `git rebase --continue`, update the current branch's metadata in the graph, and then continue restacking any remaining branches from the saved lineage. If no lineage state is found, it SHALL fall back to restacking all branches from root.

#### Scenario: Continue resumes and completes restack

- **WHEN** the user resolves conflicts and runs `st continue` after a paused restack
- **THEN** the in-progress rebase SHALL be continued
- **AND** the current branch's BaseSHA and HeadSHA SHALL be updated in the graph
- **AND** any remaining branches in the saved lineage SHALL be restacked in topological order
- **AND** the restack state file SHALL be cleared on success

#### Scenario: Continue with no rebase in progress

- **WHEN** the user runs `st continue` but no rebase is in progress
- **THEN** the command SHALL return an error: "no rebase in progress - nothing to continue"

#### Scenario: Continue when conflicts still remain

- **WHEN** the user runs `st continue` but the conflicts have not been fully resolved
- **THEN** the result SHALL indicate conflicts still exist
- **AND** the command SHALL return an error: "still have conflicts to resolve"

#### Scenario: Continue without saved lineage state

- **WHEN** the user runs `st continue` and no restack state file exists
- **THEN** the engine SHALL fall back to restacking all branches from the root
- **AND** the restack SHALL proceed with the full stack branch set

---

### Requirement: Topological Sort with Cycle Detection

The `TopologicalSort` function SHALL return all tracked branches (excluding root) in an order where every parent appears before its children. If a cycle is detected in the branch graph, it SHALL return an error identifying the branch involved in the cycle. If a branch references a parent that does not exist in the graph, it SHALL return an error.

#### Scenario: Linear stack sorted correctly

- **WHEN** `TopologicalSort` is called on a graph with root -> A -> B -> C
- **THEN** the result SHALL be [A, B, C] with each parent preceding its child

#### Scenario: Branching stack sorted correctly

- **WHEN** `TopologicalSort` is called on a graph with root -> A -> B and root -> A -> C
- **THEN** A SHALL appear before both B and C in the result
- **AND** the relative order of B and C is unspecified (both are valid)

#### Scenario: Cycle detected

- **WHEN** `TopologicalSort` is called on a graph where branch A's parent is B and B's parent is A
- **THEN** it SHALL return an error: "cycle detected involving branch: <name>"

#### Scenario: Missing branch in graph

- **WHEN** `TopologicalSort` encounters a branch whose parent is not the root and not found in the graph
- **THEN** it SHALL return an error: "branch <name> not found in graph"

---

### Requirement: Lineage Computation

The lineage computation functions SHALL correctly traverse the branch graph to determine relationships between branches.

**GetStackBranches** SHALL return the given branch plus all of its descendants (via DFS). **GetDownstreamBranches** SHALL return the same set but excluding the start branch. **GetLineage** SHALL return the full chain from root through the given branch and all its descendants. **GetAncestors** SHALL return the chain from root to the given branch, excluding descendants. **IsBranchAtTip** SHALL return true if and only if the branch has no children.

#### Scenario: GetStackBranches includes start and all descendants

- **WHEN** `GetStackBranches` is called with branch A in a graph root -> A -> B -> C
- **THEN** the result SHALL be [A, B, C]

#### Scenario: GetDownstreamBranches excludes start branch

- **WHEN** `GetDownstreamBranches` is called with branch A in a graph root -> A -> B -> C
- **THEN** the result SHALL be [B, C] (A is excluded)

#### Scenario: GetLineage returns ancestors and descendants

- **WHEN** `GetLineage` is called with branch B in a graph root -> A -> B -> C
- **THEN** the result SHALL be [root, A, B, C]

#### Scenario: GetLineage for root branch

- **WHEN** `GetLineage` is called with the root branch
- **THEN** the result SHALL include the root and all its descendants

#### Scenario: GetAncestors returns root-to-branch chain only

- **WHEN** `GetAncestors` is called with branch B in a graph root -> A -> B -> C
- **THEN** the result SHALL be [root, A, B]
- **AND** C SHALL NOT be included

#### Scenario: GetAncestors for root branch

- **WHEN** `GetAncestors` is called with the root branch itself
- **THEN** the result SHALL be [root]

#### Scenario: IsBranchAtTip for tip branch

- **WHEN** `IsBranchAtTip` is called with branch C in a graph root -> A -> B -> C
- **THEN** it SHALL return true

#### Scenario: IsBranchAtTip for mid-stack branch

- **WHEN** `IsBranchAtTip` is called with branch A in a graph root -> A -> B
- **THEN** it SHALL return false

---

### Requirement: Rerere Auto-Enable During Restack

The restack engine SHALL enable git's rerere (reuse recorded resolution) feature before performing any rebase operations. This allows git to remember conflict resolutions for reuse in future restacks. If enabling rerere fails, the restack SHALL continue with a warning rather than aborting.

#### Scenario: Rerere enabled before rebase

- **WHEN** the restack engine begins processing branches
- **THEN** `git config rerere.enabled true` SHALL be set before any rebase operation

#### Scenario: Rerere failure is non-fatal

- **WHEN** enabling rerere fails (e.g., due to permissions)
- **THEN** the restack SHALL proceed normally
- **AND** a warning SHALL be printed to the user

---

### Requirement: Rebase Uses --onto With BaseSHA

When a branch has a recorded BaseSHA, the restack engine SHALL use `git rebase --onto <parent> <BaseSHA>` to replay only the branch's own commits onto its parent. This avoids replaying ancestor commits and reduces conflicts. If no BaseSHA is recorded, a plain `git rebase <parent>` SHALL be used instead.

#### Scenario: Rebase with BaseSHA uses --onto

- **WHEN** rebasing branch B which has a recorded BaseSHA
- **THEN** the rebase command SHALL be `git rebase --onto <parent> <BaseSHA>`
- **AND** only commits between BaseSHA and B's HEAD SHALL be replayed

#### Scenario: Rebase without BaseSHA uses plain rebase

- **WHEN** rebasing branch B which has an empty BaseSHA
- **THEN** the rebase command SHALL be `git rebase <parent>`

---

### Requirement: Restack State Persistence

Restack state SHALL be persisted to `.git/stack/restack-state.json` when a conflict occurs, and cleared when the restack completes successfully (either via initial `st restack` or via `st continue`). The state file SHALL contain the lineage (list of branch names) being restacked, encoded as JSON.

#### Scenario: State file created on conflict

- **WHEN** a restack encounters a conflict
- **THEN** a file at `.git/stack/restack-state.json` SHALL be created
- **AND** it SHALL contain a JSON object with a `lineage` array of branch names

#### Scenario: State file cleared on successful restack

- **WHEN** a restack completes without conflicts
- **THEN** the restack state file SHALL be removed if it exists

#### Scenario: State file cleared on successful continue

- **WHEN** `st continue` completes the remaining restack without further conflicts
- **THEN** the restack state file SHALL be removed
