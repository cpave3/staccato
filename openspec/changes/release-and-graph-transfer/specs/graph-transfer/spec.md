## ADDED Requirements

### Requirement: Export Document

A stack SHALL be exported as a JSON document:

```json
{
  "format": 1,
  "kind": "staccato.graph",
  "st_version": "v0.5.0",
  "root": "main",
  "branches": [
    {"name": "legato/abc-1", "parent": "main", "base_sha": "…", "head_sha": "…"},
    {"name": "legato/abc-2", "parent": "legato/abc-1", "base_sha": "…", "head_sha": "…"}
  ]
}
```

`format` is the document's version, raised only for a change an older `st` cannot read correctly. `st_version` is informational. `branches` are listed depth first from the root, parents before their children and siblings by name, so each stack is contiguous. Each branch's `head_sha` is the commit its git branch points to when exported, not the graph's recorded head.

#### Scenario: Branch order
- **WHEN** a stack has `a` on `main`, and `c` and `b` both on `a`
- **THEN** the document MUST list `a`, `b`, `c` in that order

#### Scenario: Head taken from git
- **WHEN** the graph records `HeadSHA` X for a branch whose git branch points at Y
- **THEN** the exported `head_sha` MUST be Y

### Requirement: Export Command (`st graph export`)

`st graph export` SHALL write the document for the stack the current branch belongs to: the ancestor of the current branch whose parent is the root, and all of that ancestor's descendants. `--branch <name>` SHALL choose the stack containing another branch, and `--all` every branch in the graph. The document SHALL go to standard output, or to the file named by `--output`. Before writing, it SHALL check that each exported branch's tip equals its remote-tracking branch (`<remote>/<name>`, default remote `origin`, chosen with `--remote`), without contacting the remote, and SHALL fail naming every branch that is unpushed or behind, unless `--allow-unpushed` is given. A graph entry whose git branch does not exist SHALL fail the export naming it (`st detach <branch>` removes such an entry). It SHALL NOT change the graph or any branch.

#### Scenario: Export the current stack
- **WHEN** the current branch is `legato/abc-2` in a stack `legato/abc-1` → `legato/abc-2` on `main`, and both are pushed
- **THEN** the document MUST contain exactly `legato/abc-1` and `legato/abc-2` with root `main`

#### Scenario: Unpushed branch
- **WHEN** `legato/abc-2` has a commit that `origin/legato/abc-2` does not
- **THEN** `st graph export` MUST fail naming `legato/abc-2` and write no document

#### Scenario: Unpushed allowed
- **WHEN** `st graph export --allow-unpushed` runs with an unpushed branch
- **THEN** it MUST write the document with the branch's local tip

#### Scenario: Current branch not in a stack
- **WHEN** the current branch is the root or is not in the graph, and neither `--branch` nor `--all` is given
- **THEN** the command MUST fail saying which branch it looked for

### Requirement: Import Command (`st graph import`)

`st graph import <file>` (or `-` for standard input) SHALL bring a document's stack into this clone's graph. It SHALL refuse a document whose `kind` is not `staccato.graph` or whose `format` is newer than it reads (saying to update `st`), and a document whose `root` differs from this graph's root, unless this graph has no branches, in which case it adopts the document's root. Unless `--no-fetch` is given, it SHALL first fetch every document branch from the remote (default `origin`, chosen with `--remote`) into its remote-tracking branch. The document comes from another machine and SHALL be validated before any of it reaches git: every branch name MUST be a valid branch name (`git check-ref-format --branch`), appear once, and differ from the root; every parent MUST be non-empty; every `base_sha` and `head_sha` MUST be a full hexadecimal object name. A document that breaks this SHALL be refused naming the first offending branch. Every document `head_sha` MUST then be a commit in this repository; if one is missing, the import SHALL fail naming it and change nothing. The command SHALL decide an outcome for every branch, check the merged graph, and only then update branches and save the graph.

#### Scenario: Newer format
- **WHEN** the document's `format` is 2 and this `st` reads format 1
- **THEN** the import MUST fail saying to update `st` and change nothing

#### Scenario: Different root
- **WHEN** the document's root is `main` and this graph's root is `develop` and has branches
- **THEN** the import MUST fail naming both roots and change nothing

#### Scenario: Malformed branch name
- **WHEN** a document branch's name contains a space, a newline, or `..`
- **THEN** the import MUST fail naming it and change nothing, without running a fetch

#### Scenario: Missing commit
- **WHEN** after fetching, a document `head_sha` is not a commit in this repository
- **THEN** the import MUST fail naming the branch and commit and change nothing

### Requirement: Per-Branch Merge

For each document branch, the import SHALL compare the document's head (D) with the local branch's tip (L), and decide:

| Local branch | Outcome | Branch | Graph entry |
|---|---|---|---|
| does not exist | `added` | created at D, tracking `<remote>/<name>` when that exists | from the document |
| L = D | `updated` (or `unchanged` when the entry already matches) | unchanged | from the document |
| L is an ancestor of D | `fast-forwarded` | moved to D | from the document |
| D is an ancestor of L | `kept-local` | unchanged | the local entry; when the branch is not in the local graph, the document's parent and base with head L |
| neither | `conflict` | unchanged | the local entry, or none when the branch is not in the local graph |

A branch to be fast-forwarded that is checked out in another worktree, or checked out here with uncommitted changes to tracked files, SHALL get the outcome `skipped` with the reason, and keep its local entry. One checked out here with no such changes SHALL be fast-forwarded in the working tree with `git merge --ff-only`, before any other branch is changed; if git refuses (an untracked file in the way, say), the branch SHALL be `skipped` with git's reason and the remaining decisions and checks made again. Branches and graph entries the document does not name SHALL be left as they are; no branch SHALL be deleted.

#### Scenario: Stack new to this clone
- **WHEN** none of the document's branches exist locally
- **THEN** each MUST be created at its document head and added to the graph with the document's parent, base and head

#### Scenario: Worker built on a local stack
- **WHEN** `legato/abc-1` locally is an ancestor of the document's `legato/abc-1`, and `legato/abc-2` exists only in the document
- **THEN** `legato/abc-1` MUST be fast-forwarded and take the document's entry
- **AND** `legato/abc-2` MUST be added

#### Scenario: Diverged branch
- **WHEN** the local `legato/abc-1` and the document's have each got commits the other lacks
- **THEN** `legato/abc-1` MUST be reported as `conflict`, and its branch and graph entry MUST be left as they were
- **AND** the other branches MUST still be merged

#### Scenario: Local work ahead
- **WHEN** the document's head for a branch is an ancestor of the local tip
- **THEN** the branch MUST be `kept-local`, with its local entry

#### Scenario: Checked out with changes
- **WHEN** a branch to be fast-forwarded is checked out here with uncommitted changes
- **THEN** it MUST be `skipped` with that reason and not moved

#### Scenario: Local-only branches untouched
- **WHEN** the local graph has a branch the document does not name
- **THEN** its graph entry and git branch MUST be unchanged

### Requirement: Merged Graph Checks

Before changing anything, the import SHALL check the merged graph. A document branch whose parent is neither the root nor a branch in the merged graph (because the parent was a `conflict` or `skipped` with no local entry) SHALL itself be `skipped` with the reason `parent <name> was not merged`, and this SHALL repeat until every remaining branch's parent is present. The parents MUST then form no cycle; a cycle SHALL fail the import naming the branches involved and change nothing. Branch creations and moves SHALL be applied in one `git update-ref --stdin` transaction that names each moved branch's expected old value (the checked-out branch excepted, as above); an added branch then gets upstream tracking when `<remote>/<name>` exists; then the graph SHALL be saved once, in whichever storage mode is in use.

#### Scenario: Parent kept back by a conflict
- **WHEN** `b`'s document parent is `a`, `a` is a `conflict`, and `a` exists in the local graph
- **THEN** `b` MUST be merged with parent `a`

#### Scenario: Parent not merged
- **WHEN** `b`'s document parent is `a`, `a` is a `conflict`, and `a` is not in the local graph
- **THEN** `b` MUST be `skipped` with the reason `parent a was not merged`, and so MUST every branch above `b`

#### Scenario: Cycle
- **WHEN** the local graph has `x` on `y`, the document does not name `x`, and the document has `y` on `x`
- **THEN** the import MUST fail naming `x` and `y` and change nothing

#### Scenario: Shared storage
- **WHEN** the graph is in shared mode
- **THEN** the merged graph MUST be written to the shared ref

### Requirement: Import Report

The import SHALL print each branch with its outcome and, for `conflict` and `skipped`, the reason and what the person can do (for a conflict: compare `<branch>` with `<remote>/<branch>`). `--json` SHALL print `{"root", "branches": [{"name", "outcome", "reason"}]}`. `--dry-run` SHALL fetch, decide and print, and change no branch and no graph. The command SHALL exit 0 when no branch is `conflict` or `skipped`, and 1 otherwise, after applying the other branches. It SHALL NOT restack.

#### Scenario: Dry run
- **WHEN** `st graph import --dry-run stack.json` runs
- **THEN** it MUST print the outcome of every branch and leave the branches and the graph unchanged

#### Scenario: Partial import
- **WHEN** one branch is a `conflict` and the rest are `added`
- **THEN** the added branches MUST be created and saved in the graph
- **AND** the command MUST exit 1
