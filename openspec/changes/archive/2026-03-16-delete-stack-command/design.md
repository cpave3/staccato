## Context

Staccato tracks branches in a parent-pointer tree (the graph). A "lineage" is the chain from root through ancestors, the current branch, and all its descendants. Today, removing branches from the graph is done one at a time via `delete` (removes branch + git branch) or `detach` (removes from graph only). There's no bulk operation for tearing down an entire stack.

The current lineage is computed on-the-fly via `restack.GetLineage(g, branch)`, which returns `[root, ...ancestors, branch, ...descendants]`. The non-root branches in this list are the ones to remove.

## Goals / Non-Goals

**Goals:**
- One command to remove all non-root branches in the current lineage from the graph
- Default to graph-only removal (like `detach`), with opt-in git branch deletion
- Check out the root branch after removal
- Protect against accidental data loss (unpushed commits warning)

**Non-Goals:**
- Deleting branches from other lineages (only current lineage)
- Interactive branch selection (delete the whole lineage or nothing)
- Restacking remaining branches (removal doesn't affect other lineages since we remove leaf-to-root)

## Decisions

**1. Command name: `delete-stack`**

Matches the existing `delete` command naming. `delete` removes one branch; `delete-stack` removes the whole lineage. Considered `drop`, `teardown`, `cleanup` but `delete-stack` is the most discoverable.

**2. Default to graph-only removal (detach semantics)**

Users asked for this explicitly. Git branches are left intact by default. A `--branches` flag opts in to also deleting the git branches. This matches `detach` behavior and is the safest default.

**3. Compute lineage, then remove non-root branches**

Use `restack.GetLineage(g, currentBranch)` to get all branches in the lineage. Filter out root. Remove each branch from the graph using `g.RemoveBranch()`. No reparenting needed because we remove all descendants too — there are no orphans. Remove in reverse order (leaves first) for clean graph state at each step.

**4. Unpushed commit protection with `--force`**

When `--branches` is set, check each branch for a remote counterpart. If any branch hasn't been pushed and `--force` isn't set, abort with a clear message listing the unpushed branches. When doing graph-only removal (default), skip this check since git branches are preserved.

**5. Check out root after deletion**

After removing the lineage, the current branch may no longer be in the graph. Check out `g.Root` (the trunk branch) unconditionally after removal.

## Risks / Trade-offs

**[Shared graph mode]** In shared graph mode, another user could be working on branches in the same lineage. Mitigation: the graph reconciliation in `saveContext` handles this; worst case, removed branches get re-added on next sync from the other user's graph.

**[Accidental deletion]** User might run this on a lineage they didn't intend. Mitigation: the command prints the list of branches being removed before acting. The default graph-only mode is reversible (branches can be re-attached).
