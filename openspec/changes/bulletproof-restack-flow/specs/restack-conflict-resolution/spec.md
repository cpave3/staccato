## ADDED Requirements

### Requirement: Multi-Branch Conflict Resolution Cycle

The restack system SHALL support resolving conflicts across multiple branches in a single restack operation. When a conflict occurs at branch N, the user resolves it and runs `st continue`, which SHALL complete rebasing branch N and proceed to rebase branch N+1. If branch N+1 also conflicts, the cycle repeats. The full lineage SHALL be preserved in restack state throughout the entire cycle.

#### Scenario: Sequential conflicts across two branches

- **WHEN** a stack has branches root → s1 → s2 → s3 where both s1 and s2 will conflict during rebase
- **AND** the user runs `st restack` from s3
- **THEN** the restack SHALL stop at s1 with a conflict
- **AND** after resolving and running `st continue`, s1 SHALL be rebased and s2 SHALL conflict
- **AND** after resolving and running `st continue` again, s2 and s3 SHALL be rebased successfully
- **AND** the final graph SHALL have correct BaseSHA and HeadSHA for all three branches

#### Scenario: Graph state is consistent after each continue

- **WHEN** a restack stops at branch s1 due to conflict
- **THEN** branches processed before s1 SHALL have updated SHAs in the graph
- **AND** branches after s1 (including s1) SHALL retain their pre-restack SHAs
- **AND** after `st continue` succeeds for s1, s1's SHAs SHALL be updated in the graph

#### Scenario: Restack state file persists across multiple continues

- **WHEN** a restack encounters conflicts at s1, user resolves and continues, then encounters conflict at s2
- **THEN** the restack state file SHALL exist after each conflict
- **AND** the lineage in the state file SHALL be the same full lineage from the original restack
- **AND** the state file SHALL be cleared only when the final continue completes without conflict

### Requirement: Rerere Prevents Duplicate Conflict Resolution

The restack engine SHALL enable git rerere before rebasing, which allows git to automatically resolve previously-seen conflicts. When a user resolves a conflict during restack and later restacks the same branches with the same conflict pattern, git SHALL auto-resolve without user intervention.

#### Scenario: Same conflict auto-resolves on second restack

- **WHEN** a restack encounters a conflict and the user manually resolves it
- **AND** the user later causes the same conflict pattern (e.g., by restoring and restacking)
- **THEN** git rerere SHALL automatically apply the previous resolution
- **AND** the restack SHALL complete without stopping for manual conflict resolution

### Requirement: Continue After Sync Conflict

When `st sync` triggers a restack that encounters a conflict, `st continue` SHALL resume the restack correctly. The continue command SHALL work identically whether the original restack was triggered by `st restack` or `st sync`.

#### Scenario: Sync triggers conflict then continue completes

- **WHEN** `st sync` is run and the restack phase encounters a conflict at branch s1
- **AND** the user resolves the conflict and runs `st continue`
- **THEN** s1 SHALL be rebased successfully
- **AND** remaining branches in the lineage SHALL be restacked
- **AND** the restack state file SHALL be cleared

### Requirement: Conflict Identifies Branch Clearly

When a restack encounters a conflict, the error message SHALL clearly identify which branch the conflict occurred on, so the user knows exactly what they are resolving.

#### Scenario: Error message includes conflicting branch name

- **WHEN** a restack encounters a conflict while rebasing branch s2
- **THEN** the error output SHALL include the branch name "s2"
- **AND** the message SHALL instruct the user to resolve and run `st continue`
