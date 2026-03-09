# Staleness Detection

Staleness detection provides fast, offline warnings when local repository state has drifted from the last-fetched remote state. It is advisory only and never blocks user operations.

## Data Model

### DetectedSignal

A single staleness indicator containing:
- `Branch` — the name of the affected branch.
- `Message` — a human-readable description of the staleness condition.

### Report

A collection of `DetectedSignal` values. The report exposes an `IsStale()` method that returns true when one or more signals are present.

---

### Requirement: Offline-Only Detection

Staleness detection SHALL NOT make any network calls. It MUST compare only local refs against whatever remote tracking refs were last fetched. This ensures the check is fast and safe for offline or air-gapped environments.

#### Scenario: Detection uses only local refs
- **WHEN** `Check()` is called on a repository with stale tracking refs
- **THEN** it SHALL compare local branch SHAs against `origin/<branch>` tracking refs using `rev-parse` and `merge-base --is-ancestor`
- **AND** it SHALL NOT invoke `git fetch` or any other network-reaching git command

---

### Requirement: Root Branch Behind Origin Detection

The staleness check MUST detect when the graph's root (trunk) branch is behind its remote tracking counterpart. A root branch is considered behind when the local commit is an ancestor of the remote tracking ref commit (i.e., the remote has commits the local branch does not).

#### Scenario: Trunk is behind origin
- **WHEN** the local root branch SHA differs from `origin/<root>` SHA
- **AND** the local root branch is an ancestor of `origin/<root>`
- **THEN** the report SHALL contain a signal for the root branch
- **AND** the signal message SHALL read `'<root>' is behind 'origin/<root>'`

#### Scenario: Trunk is up to date with origin
- **WHEN** the local root branch SHA equals the `origin/<root>` SHA
- **THEN** the report SHALL NOT contain a signal for the root branch

#### Scenario: Trunk has diverged from origin
- **WHEN** the local root branch SHA differs from `origin/<root>` SHA
- **AND** the local root branch is NOT an ancestor of `origin/<root>` (i.e., they have diverged)
- **THEN** the report SHALL NOT contain a signal for the root branch

---

### Requirement: Remote Tracking Ref Deletion Detection

The staleness check MUST detect when a branch tracked in the graph no longer has a corresponding remote tracking ref (`refs/remotes/origin/<branch>`). This typically indicates the branch was deleted on the remote (e.g., after a PR merge).

#### Scenario: Graph branch deleted on remote
- **WHEN** a branch exists in the graph's `Branches` map
- **AND** `refs/remotes/origin/<branch>` does not exist locally
- **THEN** the report SHALL contain a signal for that branch
- **AND** the signal message SHALL read `'<branch>' has been deleted on remote`

#### Scenario: Graph branch still exists on remote
- **WHEN** a branch exists in the graph's `Branches` map
- **AND** `refs/remotes/origin/<branch>` exists locally
- **THEN** the report SHALL NOT contain a deletion signal for that branch

---

### Requirement: Graceful Handling When No Remote Is Configured

Staleness detection MUST gracefully handle repositories with no remote configured. It SHALL produce an empty report without errors.

#### Scenario: Repository has no remote
- **WHEN** `Check()` is called on a graph whose root branch has no remote tracking ref
- **THEN** the report SHALL NOT be stale
- **AND** no error SHALL be returned

#### Scenario: checkStaleness skips when no remote exists
- **WHEN** `checkStaleness()` is called and `HasRemote()` returns false
- **THEN** the function SHALL return immediately without invoking `Check()`
- **AND** no output SHALL be printed

---

### Requirement: Warning-Only Behavior

Staleness detection MUST be advisory. It SHALL print warnings to inform the user but SHALL NOT block, abort, or alter the outcome of any command. The warning MUST suggest running `st sync` to update.

#### Scenario: Stale state produces a warning
- **WHEN** `checkStaleness()` detects one or more staleness signals
- **THEN** it SHALL print a warning message: `"Local state is behind remote -- run 'st sync' to update"`
- **AND** each signal message SHALL be printed as an indented detail line
- **AND** the calling command SHALL continue executing normally

#### Scenario: Clean state produces no output
- **WHEN** `checkStaleness()` detects no staleness signals
- **THEN** it SHALL NOT print any warning or output

---

### Requirement: Integration With Commands

`checkStaleness()` SHALL be called at the start of most user-facing commands so that users receive timely warnings about drift. It MUST be invoked after `getContext()` succeeds and before the command's core logic executes.

#### Scenario: Commands that check staleness
- **WHEN** any of the following commands are executed: `new`, `append`, `insert`, `restack`, `attach`, `log`, `status`
- **THEN** each command SHALL call `checkStaleness()` with the current graph, git runner, and printer
- **AND** the staleness check SHALL occur before the command performs its primary operation

#### Scenario: Commands that do not check staleness
- **WHEN** commands such as `sync`, `continue`, `restore`, `backup`, `switch`, or `graph` are executed
- **THEN** they SHALL NOT call `checkStaleness()`

---

### Requirement: Multiple Simultaneous Signals

The staleness check MUST be capable of reporting multiple signals in a single check. For example, the trunk may be behind origin while a graph branch has been deleted on the remote.

#### Scenario: Trunk behind and branch deleted
- **WHEN** the root branch is behind `origin/<root>`
- **AND** a graph branch has been deleted on the remote
- **THEN** the report SHALL contain signals for both conditions
- **AND** `IsStale()` SHALL return true
