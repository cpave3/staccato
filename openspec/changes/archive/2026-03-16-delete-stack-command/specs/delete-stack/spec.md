## ADDED Requirements

### Requirement: Delete current lineage from graph
The `delete-stack` command SHALL compute the current lineage using `GetLineage`, exclude the root branch, and remove all remaining branches from the graph.

#### Scenario: Delete lineage with multiple branches
- **WHEN** the user runs `st delete-stack` while on a branch in a lineage of `main -> A -> B -> C`
- **THEN** branches A, B, and C are removed from the graph and root (main) is preserved

#### Scenario: Delete lineage when on a mid-stack branch
- **WHEN** the user runs `st delete-stack` while on branch B in lineage `main -> A -> B -> C`
- **THEN** all non-root branches in the lineage (A, B, C) are removed from the graph

#### Scenario: Cannot delete-stack on root branch
- **WHEN** the user runs `st delete-stack` while on the root branch
- **THEN** the command SHALL return an error indicating the root branch cannot be deleted

### Requirement: Preserve git branches by default
The `delete-stack` command SHALL only modify the graph by default, leaving all git branches intact.

#### Scenario: Default removal preserves git branches
- **WHEN** the user runs `st delete-stack` without `--branches`
- **THEN** all lineage branches are removed from the graph but the git branches still exist

### Requirement: Optional git branch deletion
The `delete-stack` command SHALL accept a `--branches` flag that also deletes the git branches after removing them from the graph.

#### Scenario: Delete with --branches flag
- **WHEN** the user runs `st delete-stack --branches`
- **THEN** all lineage branches are removed from the graph AND the corresponding git branches are deleted

#### Scenario: Skip deletion of current checkout branch until last
- **WHEN** the user runs `st delete-stack --branches` while on branch B
- **THEN** the command checks out root before deleting git branches so the current branch can be deleted

### Requirement: Unpushed commit protection
When `--branches` is set, the command SHALL check each branch for a remote counterpart. If any branch has not been pushed, the command SHALL abort unless `--force` is also set.

#### Scenario: Abort on unpushed branches without force
- **WHEN** the user runs `st delete-stack --branches` and branch A has not been pushed
- **THEN** the command aborts with an error listing the unpushed branches

#### Scenario: Force delete unpushed branches
- **WHEN** the user runs `st delete-stack --branches --force` and branch A has not been pushed
- **THEN** all branches are removed from graph and git branches are deleted regardless

#### Scenario: No protection needed for graph-only removal
- **WHEN** the user runs `st delete-stack` (without `--branches`) and branches have not been pushed
- **THEN** the command succeeds without warning since git branches are preserved

### Requirement: Check out root after deletion
After removing the lineage, the command SHALL check out the graph root branch.

#### Scenario: Checkout root after delete-stack
- **WHEN** the user runs `st delete-stack` on any lineage
- **THEN** the working tree is on the root branch after the command completes

### Requirement: Print summary of removed branches
The command SHALL print the list of branches being removed before performing the operation.

#### Scenario: Summary output
- **WHEN** the user runs `st delete-stack` on lineage `main -> A -> B`
- **THEN** the command prints which branches will be removed (A, B) and confirms success after
