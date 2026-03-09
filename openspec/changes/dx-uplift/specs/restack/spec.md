## MODIFIED Requirements

### Requirement: Continue Restack After Conflict Resolution

The `st continue` command SHALL resume a paused restack operation. It MUST verify that a rebase is currently in progress; if not, it SHALL return an error. It MUST also verify that a restack state file (`.git/stack/restack-state.json`) exists; if a rebase is in progress but no state file exists, it SHALL return a specific error directing the user to `git rebase --continue`. It SHALL call `git rebase --continue`, update the current branch's metadata in the graph, and then continue restacking any remaining branches from the saved lineage.

#### Scenario: Continue resumes and completes restack

- **WHEN** the user resolves conflicts and runs `st continue` after a paused restack
- **THEN** the in-progress rebase SHALL be continued
- **AND** the current branch's BaseSHA and HeadSHA SHALL be updated in the graph
- **AND** any remaining branches in the saved lineage SHALL be restacked in topological order
- **AND** the restack state file SHALL be cleared on success

#### Scenario: Continue with no rebase in progress

- **WHEN** the user runs `st continue` but no rebase is in progress
- **THEN** the command SHALL return an error: "no rebase in progress — nothing to continue"

#### Scenario: Continue when conflicts still remain

- **WHEN** the user runs `st continue` but the conflicts have not been fully resolved
- **THEN** the result SHALL indicate conflicts still exist
- **AND** the command SHALL return an error: "still have conflicts to resolve"

#### Scenario: Continue with rebase but no restack state

- **WHEN** the user runs `st continue` and a git rebase is in progress
- **AND** no `.git/stack/restack-state.json` file exists
- **THEN** the command SHALL return an error: "no st restack in progress — did you mean 'git rebase --continue'?"

#### Scenario: Continue without saved lineage state

- **WHEN** the user runs `st continue` and the restack state file exists but contains no lineage
- **THEN** the engine SHALL fall back to restacking all branches from the root
- **AND** the restack SHALL proceed with the full stack branch set

### Requirement: Conflict Handling During Restack

When a rebase operation encounters a merge conflict, the restack engine SHALL stop immediately, mark the result as having conflicts, and record which branch the conflict occurred at. The `st restack` command SHALL persist restack state (the lineage being processed) to `.git/stack/restack-state.json` so that `st continue` can resume from where it left off. The graph state SHALL be saved even when conflicts occur. The error message SHALL clearly identify the conflicting branch.

#### Scenario: Conflict stops restack and saves state

- **WHEN** a rebase conflict occurs while rebasing branch B onto its parent A
- **THEN** the restack SHALL stop immediately without processing further branches
- **AND** the result's `Conflicts` field SHALL be true and `ConflictsAt` SHALL be "B"
- **AND** a restack state file SHALL be written containing the full lineage
- **AND** the graph SHALL be saved to preserve any already-updated branch metadata

#### Scenario: Conflict error message directs user to continue

- **WHEN** a conflict is detected during `st restack`
- **THEN** the command SHALL return an error: "conflict during restack at 'B' — resolve the conflicts and run 'st continue'"
