# Branch Attach

The `st attach` command adopts a branch into the stack graph by assigning it a parent. It supports interactive TUI selection, automatic parent detection, and direct parent specification. If no branch name argument is provided, it operates on the current branch.

---

### Requirement: Branch Resolution

The command SHALL resolve the target branch to attach. If a branch name argument is provided, that branch SHALL be used. If no argument is provided, the current checked-out branch SHALL be used.

#### Scenario: Explicit branch argument
- **WHEN** the user runs `st attach feature-x`
- **THEN** the branch `feature-x` SHALL be the target for attachment

#### Scenario: No branch argument
- **WHEN** the user runs `st attach` without a branch argument
- **THEN** the current checked-out branch SHALL be the target for attachment

---

### Requirement: Interactive TUI Attachment (Default Mode)

When neither `--auto` nor `--parent` is specified, the command SHALL launch an interactive TUI that displays all local git branches (excluding the branch being attached) as parent candidates. The user SHALL be able to navigate, search, and select a parent branch.

#### Scenario: TUI displays all branches as candidates
- **WHEN** the TUI launches for attaching branch `feature-x`
- **THEN** all local git branches except `feature-x` SHALL be listed as candidates
- **AND** the current checked-out branch SHALL be marked with a filled circle indicator

#### Scenario: User selects a parent via Enter
- **WHEN** the user highlights a branch in the TUI and presses Enter
- **THEN** the highlighted branch SHALL be selected as the parent
- **AND** the target branch SHALL be attached as a child of the selected parent in the graph
- **AND** the graph SHALL be persisted

#### Scenario: User quits without selecting
- **WHEN** the user presses `q` or `Esc` in the TUI (outside search mode)
- **THEN** the TUI SHALL exit without attaching anything

#### Scenario: TUI search mode
- **WHEN** the user presses `/` in the TUI
- **THEN** the TUI SHALL enter search mode
- **AND** typed characters SHALL filter the displayed branches by substring match (case-insensitive)
- **AND** matching branches SHALL be visually highlighted
- **AND** pressing Enter SHALL exit search mode and move the cursor to the first match
- **AND** pressing Esc SHALL exit search mode and clear the search query
- **AND** pressing Backspace SHALL remove the last character from the search query

#### Scenario: No candidates available
- **WHEN** there are no other local branches besides the branch being attached
- **THEN** the command SHALL return an error: "no existing branches to use as parent for '<branch>'"

#### Scenario: TUI relaunches for tracked branches
- **WHEN** the user runs `st attach` on a branch that is already tracked in the graph
- **THEN** the TUI SHALL still launch (allowing the user to modify the stack interactively)

---

### Requirement: Auto-Attach Mode

When the `--auto` flag is specified, the command SHALL automatically select the best parent candidate without launching the TUI. Parent candidates SHALL be scored by merge-base recency among branches already in the graph.

#### Scenario: Auto-attach succeeds
- **WHEN** the user runs `st attach --auto` and there are branches in the graph with common history
- **THEN** the best-scoring candidate SHALL be selected as the parent
- **AND** the branch SHALL be attached to the graph and persisted

#### Scenario: Auto-attach with no suitable parent
- **WHEN** the user runs `st attach --auto` and no graph branches share common history with the target
- **THEN** the command SHALL return an error: "no suitable parent found for branch '<branch>'"

#### Scenario: Auto-attach on already-tracked branch
- **WHEN** the user runs `st attach --auto` on a branch that is already in the graph
- **THEN** the command SHALL return successfully without making changes (no-op)

---

### Requirement: Direct Parent Mode

When the `--parent` flag is specified, the command SHALL attach the branch under the given parent without launching the TUI or running auto-detection.

#### Scenario: Attach with valid parent
- **WHEN** the user runs `st attach --parent feature-a` and `feature-a` is in the graph
- **THEN** the target branch SHALL be attached as a child of `feature-a`
- **AND** the graph SHALL be persisted
- **AND** a success message SHALL be printed: "Attached '<branch>' as child of '<parent>'"

#### Scenario: Parent not in stack
- **WHEN** the user runs `st attach --parent unknown-branch` and `unknown-branch` is not in the graph and is not a trunk branch
- **THEN** the command SHALL return an error: "parent '<branch>' is not in the stack"

#### Scenario: Parent is the graph root
- **WHEN** the user runs `st attach --parent <root>` and the specified parent is the graph root
- **THEN** the target branch SHALL be attached as a child of the root

---

### Requirement: Recursive Attachment

When the user selects a parent in the TUI that is not yet tracked in the graph, the command SHALL recursively prompt the user to attach the untracked parent first, before completing the original attachment.

#### Scenario: Selected parent is untracked
- **WHEN** the user selects branch `parent-x` as the parent in the TUI
- **AND** `parent-x` is not in the graph and is not the root
- **THEN** the command SHALL print a message: "Parent '<parent-x>' is not yet in the stack. Attaching it first..."
- **AND** the command SHALL recursively launch attachment for `parent-x`
- **AND** once `parent-x` is attached, the original branch SHALL be attached under `parent-x`

#### Scenario: Recursive attachment stops when parent is already tracked
- **WHEN** recursive attachment is invoked for a branch that is already in the graph
- **THEN** the recursion SHALL stop immediately (no-op)

---

### Requirement: Root Detection

Trunk branches (main, master, develop, trunk) SHALL be automatically detected as roots. The user MAY also manually designate any branch as root by pressing `r` in the TUI.

#### Scenario: Trunk branch selected as parent in TUI
- **WHEN** the user selects a trunk branch (main, master, develop, or trunk) as the parent in the TUI
- **THEN** the selected branch SHALL be set as the graph root
- **AND** the target branch SHALL be attached as a child of the new root
- **AND** a message SHALL be printed: "Set '<trunk>' as stack root"

#### Scenario: User presses r to set root in TUI
- **WHEN** the user highlights a branch in the TUI and presses `r`
- **THEN** the highlighted branch SHALL be set as the graph root
- **AND** the target branch SHALL be attached as a child of the new root
- **AND** a message SHALL be printed: "Set '<branch>' as stack root"

#### Scenario: Trunk branch used with --parent flag
- **WHEN** the user runs `st attach --parent main` and `main` is not in the graph but exists in git
- **THEN** `main` SHALL be automatically set as the graph root
- **AND** a message SHALL be printed: "Set 'main' as stack root"
- **AND** the target branch SHALL be attached as a child of `main`

#### Scenario: Trunk branch used with --parent but does not exist in git
- **WHEN** the user runs `st attach --parent main` and `main` does not exist as a git branch
- **THEN** the command SHALL return an error: "parent 'main' is not in the stack"

---

### Requirement: Branch Relocation

When `--parent` is used on a branch that is already tracked in the graph, the command SHALL relocate the branch by reparenting it under the new parent, rebasing it, and restacking all downstream branches.

#### Scenario: Relocate to a new parent
- **WHEN** the user runs `st attach --parent new-parent` on a branch already in the graph with a different parent
- **THEN** the branch SHALL be reparented under `new-parent` in the graph
- **AND** the branch SHALL be rebased onto `new-parent`
- **AND** the branch metadata (BaseSHA, HeadSHA) SHALL be updated
- **AND** the graph SHALL be persisted
- **AND** a success message SHALL be printed: "Relocated '<branch>' under '<new-parent>'"

#### Scenario: Relocate with downstream branches
- **WHEN** the relocated branch has downstream (child) branches in the graph
- **THEN** all downstream branches SHALL be restacked after the relocation
- **AND** automatic backups SHALL be created for the relocated branch and all downstream branches before any destructive operations

#### Scenario: Relocate to same parent (no-op)
- **WHEN** the user runs `st attach --parent current-parent` and the branch already has that parent
- **THEN** the command SHALL print: "'<branch>' already has parent '<parent>'"
- **AND** no rebase or graph changes SHALL occur

#### Scenario: Rebase conflict during relocation
- **WHEN** a rebase conflict occurs during relocation of the branch itself
- **THEN** the backups SHALL be restored for all affected branches
- **AND** the command SHALL return an error

#### Scenario: Restack conflict during downstream restacking
- **WHEN** a rebase conflict occurs while restacking a downstream branch after relocation
- **THEN** the command SHALL return an error indicating which branch has the conflict: "conflict during restack at '<branch>'"

#### Scenario: Backup cleanup on success
- **WHEN** the relocation and downstream restacking complete successfully
- **THEN** all automatic backups created for the operation SHALL be cleaned up

---

### Requirement: Graph Persistence

The attach command SHALL persist the graph after any successful modification. The persistence target (local file or shared ref) SHALL match the current storage mode.

#### Scenario: Graph saved after attachment
- **WHEN** a branch is successfully attached, relocated, or a root is set
- **THEN** the graph SHALL be saved via `saveContext`
- **AND** if saving fails, the command SHALL return an error: "failed to save graph: <details>"

---

### Requirement: Branch Validation

The command SHALL validate that the target branch exists in git before attaching it.

#### Scenario: Branch does not exist in git
- **WHEN** `AttachBranch` is called for a branch that does not exist in git
- **THEN** the command SHALL return an error: "branch '<branch>' does not exist in git"

#### Scenario: Parent branch not found in graph
- **WHEN** `AttachBranch` is called with a parent that is neither the graph root nor a tracked branch
- **THEN** the command SHALL return an error: "parent branch '<parent>' not found in graph"
