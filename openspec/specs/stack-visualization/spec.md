# Stack Visualization

This specification covers the visual display of the stack graph via `st log`, `st status`, and `st switch`, including tree rendering, branch markers, PR status annotations, and interactive TUI navigation.

---

## Tree Rendering

### Requirement: Depth-Based Indentation

The stack tree SHALL be rendered with depth-based indentation, where each level of nesting adds two spaces of indentation relative to its parent.

#### Scenario: Root branch at zero depth
- **WHEN** the root branch is rendered
- **THEN** it SHALL appear with no leading indentation

#### Scenario: Child branch indentation
- **WHEN** a branch is a child of the root
- **THEN** it SHALL be indented by two spaces relative to its parent
- **AND** a grandchild SHALL be indented by four spaces relative to the root

#### Scenario: Multiple children at same depth
- **WHEN** a branch has multiple children
- **THEN** all children SHALL be rendered at the same indentation level
- **AND** each child's subtree SHALL follow immediately after the child

### Requirement: Recursive Tree Traversal

The tree SHALL be rendered using a recursive depth-first traversal starting from the root branch. Children of each branch SHALL be obtained via `Graph.GetChildren(branch)`.

#### Scenario: Full tree rendering order
- **WHEN** the tree is rendered
- **THEN** the root branch SHALL appear first
- **AND** each branch's children SHALL appear immediately after the branch, before any siblings

### Requirement: Current Branch Marker

The current (checked-out) branch SHALL be marked with the `●` icon. All other branches SHALL be marked with the `○` icon.

#### Scenario: Current branch display
- **WHEN** a branch is the currently checked-out branch
- **THEN** it SHALL be displayed with the `●` icon preceding its name

#### Scenario: Non-current branch display
- **WHEN** a branch is not the currently checked-out branch
- **THEN** it SHALL be displayed with the `○` icon preceding its name

---

## st log

### Requirement: Display Stack Hierarchy

The `st log` command SHALL display the full stack hierarchy as a tree, starting from the root branch. It SHALL print a "Stack:" header before the tree and include blank lines before and after the output.

#### Scenario: Basic stack log output
- **WHEN** the user runs `st log`
- **THEN** the output SHALL begin with a blank line followed by "Stack:"
- **AND** the tree SHALL be rendered with depth-based indentation and branch markers
- **AND** the output SHALL end with a blank line

#### Scenario: Single root with no children
- **WHEN** the stack contains only a root branch and no children
- **THEN** `st log` SHALL display only the root branch with its marker icon

### Requirement: Staleness Check on Log

The `st log` command SHALL perform a staleness check before displaying the tree, warning if local state is behind the remote.

#### Scenario: Stale local state
- **WHEN** the user runs `st log` and local state is behind the remote
- **THEN** a staleness warning SHALL be displayed before the stack tree

---

## st status

### Requirement: Display Stack with PR Status Annotations

The `st status` command SHALL display the stack tree with PR status annotations appended to each non-root branch. The root branch SHALL NOT have a PR status annotation.

#### Scenario: Root branch without annotation
- **WHEN** `st status` renders the root branch
- **THEN** the root branch SHALL be displayed with only its marker icon and name, with no PR suffix

#### Scenario: Branch with no PR
- **WHEN** a non-root branch has no associated pull request
- **THEN** it SHALL be displayed with the suffix ` — No PR`

### Requirement: Merged PR Status

When a branch has a merged PR, the annotation SHALL display the PR number, a success icon (`✔`), and the word "Merged".

#### Scenario: Merged PR display
- **WHEN** a branch has a PR in the MERGED state
- **THEN** the annotation SHALL read `#<number> ✔ Merged`

### Requirement: Closed PR Status

When a branch has a closed (not merged) PR, the annotation SHALL display the PR number, an error icon (`✘`), and the word "Closed".

#### Scenario: Closed PR display
- **WHEN** a branch has a PR in the CLOSED state
- **THEN** the annotation SHALL read `#<number> ✘ Closed`

### Requirement: Open PR Status with Review State

When a branch has an open PR, the annotation SHALL reflect the draft status and review state.

#### Scenario: Draft PR display
- **WHEN** a branch has an open PR that is a draft
- **THEN** the annotation SHALL read `#<number> Draft`

#### Scenario: Approved PR display
- **WHEN** a branch has an open, non-draft PR with APPROVED review status
- **THEN** the annotation SHALL read `#<number> ✔ Approved`

#### Scenario: Changes requested PR display
- **WHEN** a branch has an open, non-draft PR with CHANGES_REQUESTED review status
- **THEN** the annotation SHALL read `#<number> ⚠ Changes requested`

#### Scenario: Review pending PR display
- **WHEN** a branch has an open, non-draft PR with no specific review status (not APPROVED or CHANGES_REQUESTED)
- **THEN** the annotation SHALL read `#<number> Review pending`

### Requirement: CI Status Annotation

For open PRs, the CI check status SHALL be appended as a separate segment joined by ` | `.

#### Scenario: Failing CI display
- **WHEN** an open PR has a check status of "fail"
- **THEN** `CI ✘` SHALL be appended to the annotation, separated by ` | `

#### Scenario: Pending CI display
- **WHEN** an open PR has a check status of "pending"
- **THEN** `CI pending` SHALL be appended to the annotation, separated by ` | `

#### Scenario: Passing or absent CI display
- **WHEN** an open PR has a check status of "pass" or an empty check status
- **THEN** no CI segment SHALL be appended to the annotation

#### Scenario: Combined review and CI status
- **WHEN** a branch has an open, approved PR with failing CI
- **THEN** the annotation SHALL read `#<number> ✔ Approved | CI ✘`

### Requirement: Forge Detection

The `st status` command SHALL detect the forge (hosting provider) from the origin remote URL. Only GitHub is currently supported.

#### Scenario: GitHub remote detected
- **WHEN** the origin remote URL contains "github.com"
- **THEN** the GitHub forge SHALL be used to fetch PR statuses

#### Scenario: Unsupported forge
- **WHEN** the origin remote URL does not match any supported forge
- **THEN** `st status` SHALL return an error indicating the forge is not supported

### Requirement: Staleness Check on Status

The `st status` command SHALL perform a staleness check before displaying the tree.

#### Scenario: Stale local state on status
- **WHEN** the user runs `st status` and local state is behind the remote
- **THEN** a staleness warning SHALL be displayed before the stack status tree

---

## st switch

### Requirement: Interactive TUI Branch Switcher

The `st switch` command SHALL launch an interactive terminal UI displaying the stack as a tree. The user SHALL be able to navigate and select a branch to check out.

#### Scenario: TUI displays stack tree
- **WHEN** the user runs `st switch`
- **THEN** a TUI SHALL be displayed showing all branches in the stack as a tree with depth-based indentation and branch markers

#### Scenario: No branches in stack
- **WHEN** the stack contains no branches
- **THEN** `st switch` SHALL return an error "no branches in stack"

### Requirement: Initial Cursor Position

The TUI SHALL initialize the cursor (selected item) to the currently checked-out branch.

#### Scenario: Cursor starts on current branch
- **WHEN** the TUI is opened
- **THEN** the cursor SHALL be positioned on the current branch

### Requirement: Navigation with Arrow Keys

The TUI SHALL support up/down arrow key navigation to move the cursor between branches. Navigation messages SHALL be delegated to the underlying list model.

#### Scenario: Arrow key navigation
- **WHEN** the user presses up or down arrow keys
- **THEN** the cursor SHALL move to the previous or next branch in the list

### Requirement: Branch Selection via Enter

Pressing Enter SHALL select the branch under the cursor and check it out.

#### Scenario: Select a different branch
- **WHEN** the user presses Enter on a branch that is not the current branch
- **THEN** the TUI SHALL quit and git SHALL check out the selected branch
- **AND** the message "Switched to '<branch>'" SHALL be printed

#### Scenario: Select the current branch
- **WHEN** the user presses Enter on the branch that is already checked out
- **THEN** the TUI SHALL quit and print "Already on '<branch>'"
- **AND** no git checkout SHALL be performed

### Requirement: Quit Without Selection

Pressing `q` or `Esc` in normal mode SHALL quit the TUI without selecting or switching branches.

#### Scenario: Quit with q
- **WHEN** the user presses `q` in normal mode
- **THEN** the TUI SHALL quit without checking out any branch
- **AND** the `quitting` flag SHALL be true and `selected` SHALL be empty

#### Scenario: Quit with Esc
- **WHEN** the user presses `Esc` in normal mode
- **THEN** the TUI SHALL quit without checking out any branch

### Requirement: Search Mode Activation

Pressing `/` in normal mode SHALL activate search mode, clearing any previous search query and match list.

#### Scenario: Activate search mode
- **WHEN** the user presses `/`
- **THEN** the TUI SHALL enter search mode
- **AND** the search query SHALL be empty
- **AND** the search input SHALL be displayed at the bottom of the view

### Requirement: Search Query Input

In search mode, typed characters SHALL be appended to the search query. Backspace SHALL remove the last character. The match list SHALL update after each keystroke.

#### Scenario: Type search characters
- **WHEN** the user types characters in search mode
- **THEN** each character SHALL be appended to the search query
- **AND** the match list SHALL be recalculated

#### Scenario: Backspace in search mode
- **WHEN** the user presses backspace in search mode
- **THEN** the last character SHALL be removed from the search query
- **AND** the match list SHALL be recalculated

### Requirement: Case-Insensitive Search Matching

Search matching SHALL be case-insensitive. A branch matches if its name contains the search query as a substring.

#### Scenario: Case-insensitive matching
- **WHEN** the search query is "feat"
- **THEN** branches named "Feature-A", "my-FEAT", and "feat-1" SHALL all match

### Requirement: Search Mode Exit via Esc

Pressing `Esc` in search mode SHALL exit search mode, clear the search query, and clear all matches.

#### Scenario: Cancel search with Esc
- **WHEN** the user presses `Esc` in search mode
- **THEN** the TUI SHALL return to normal mode
- **AND** the search query SHALL be cleared
- **AND** the match list SHALL be cleared

### Requirement: Search Mode Exit via Enter

Pressing `Enter` in search mode SHALL exit search mode and jump the cursor to the first match, if any.

#### Scenario: Confirm search with Enter
- **WHEN** the user presses `Enter` in search mode and there are matches
- **THEN** the TUI SHALL return to normal mode
- **AND** the cursor SHALL move to the first matching branch

#### Scenario: Confirm search with no matches
- **WHEN** the user presses `Enter` in search mode and there are no matches
- **THEN** the TUI SHALL return to normal mode
- **AND** the cursor position SHALL remain unchanged

### Requirement: Search Match Navigation

In normal mode, pressing `n` SHALL move to the next search match and `N` SHALL move to the previous search match. These keys are documented in the help text.

#### Scenario: Navigate to next match
- **WHEN** the user presses `n` after a search with multiple matches
- **THEN** the cursor SHALL move to the next matching branch

#### Scenario: Navigate to previous match
- **WHEN** the user presses `N` after a search with multiple matches
- **THEN** the cursor SHALL move to the previous matching branch

### Requirement: Visual Styling in TUI

The TUI SHALL apply distinct visual styles to branches based on their state.

#### Scenario: Current branch styling
- **WHEN** a branch is the currently checked-out branch
- **THEN** it SHALL be rendered in green with bold text

#### Scenario: Selected line indicator
- **WHEN** the cursor is on a branch
- **THEN** the line SHALL be prefixed with `> `
- **AND** non-selected lines SHALL be prefixed with two spaces

#### Scenario: Search match highlighting
- **WHEN** a branch matches the search query
- **THEN** it SHALL be rendered in gold with bold text (unless it is the current branch and selected)

#### Scenario: Non-match dimming during search
- **WHEN** search mode is active with a non-empty query
- **THEN** branches that do not match SHALL be rendered in a dimmed style

### Requirement: Search Status Display

When search mode is active, the search query and match count SHALL be displayed below the branch list.

#### Scenario: Search status bar
- **WHEN** search mode is active
- **THEN** the search query SHALL be displayed as `/<query>`
- **AND** if there are matches, the match count SHALL be displayed as `[<current>/<total> matches]`

### Requirement: Help Text Display

The TUI SHALL display a help line showing available key bindings.

#### Scenario: Help text content
- **WHEN** the TUI is displayed
- **THEN** a help line SHALL appear showing: `↑↓ navigate  / search  n/N next/prev match  enter select  q quit`

### Requirement: Title Display

The TUI SHALL display a "Switch Branch" title at the top, styled in bold purple.

#### Scenario: Title rendering
- **WHEN** the TUI is displayed
- **THEN** the title "Switch Branch" SHALL appear at the top of the view

### Requirement: Window Resize Handling

The TUI SHALL respond to terminal window resize events by adjusting the list dimensions. The height SHALL reserve 3 lines for the search box and help text.

#### Scenario: Window resize
- **WHEN** the terminal window is resized
- **THEN** the list width SHALL match the new terminal width
- **AND** the list height SHALL be the new terminal height minus 3

### Requirement: Root Branch Resolution for Switch

The `st switch` command SHALL resolve the root branch by checking if the current branch has a root in the graph. If not, it SHALL fall back to `g.Root`.

#### Scenario: Current branch has a root in graph
- **WHEN** the current branch is part of a lineage with a root
- **THEN** the tree SHALL be displayed starting from that root

#### Scenario: Current branch not in graph
- **WHEN** the current branch is not tracked in the graph
- **THEN** the tree SHALL be displayed starting from `g.Root`

### Requirement: Quitting View Renders Empty

When the TUI is in the quitting state, the View function SHALL return an empty string to clean up the terminal.

#### Scenario: Quitting view
- **WHEN** the TUI is quitting
- **THEN** the View function SHALL return an empty string
