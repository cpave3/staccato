# MCP Server Specification

This specification defines the behavior of the Staccato MCP (Model Context Protocol) server, which exposes stack management and git operations as MCP tools and prompts over a stdio transport.

---

## 1. Server Startup and Transport

### Requirement: Stdio Transport Initialization
The `st mcp` command SHALL start an MCP server that communicates over stdin/stdout using the stdio transport. The server SHALL load the StaccatoContext (graph, git runner, repo path) before accepting requests.

#### Scenario: Server starts successfully
- **WHEN** `st mcp` is invoked in a valid git repository with an initialized stack
- **THEN** the server SHALL listen on stdin/stdout for JSON-RPC MCP messages

#### Scenario: Server fails without valid repo
- **WHEN** `st mcp` is invoked outside a git repository or without a valid stack
- **THEN** the command SHALL return an error from context loading

### Requirement: Server Metadata
The server SHALL identify itself with the name `"staccato"` and version `"0.1.0"`. It SHALL advertise tool, prompt, and resource capabilities.

#### Scenario: Server capabilities
- **WHEN** a client sends an `initialize` request
- **THEN** the server SHALL respond with server info name `"staccato"`, version `"0.1.0"`
- **AND** the response SHALL include tool, prompt, and resource capabilities

---

## 2. Stack Tools

### Requirement: Stack Tree Display (st_log)
The `st_log` tool SHALL return the stack tree structure as JSON. The response SHALL be a nested tree of nodes, each containing a `branch` name, a `current` flag (true if the branch is the checked-out branch), and a `children` array.

#### Scenario: Display stack tree
- **WHEN** `st_log` is called with no parameters
- **THEN** the tool SHALL return a JSON tree rooted at the graph root branch
- **AND** the `current` field SHALL be `true` for the currently checked-out branch
- **AND** the tool annotation SHALL indicate read-only, non-destructive, idempotent behavior

### Requirement: Stack Status with PR Annotations (st_status)
The `st_status` tool SHALL return the stack tree structure as JSON, augmented with PR status information from the detected forge (e.g., GitHub). Each node SHALL include `pr_number`, `pr_state`, `pr_draft`, `review_status`, `check_status`, and `pr_url` fields when a PR exists.

#### Scenario: Display stack with PR info
- **WHEN** `st_status` is called and a forge is detected
- **THEN** the tool SHALL return a JSON tree with PR status fields populated for branches that have open PRs
- **AND** branches without PRs SHALL omit the PR fields (zero values)

#### Scenario: Display stack without forge
- **WHEN** `st_status` is called and no forge is detected
- **THEN** the tool SHALL return the tree structure without PR annotations

### Requirement: Current Branch Info (st_current)
The `st_current` tool SHALL return JSON containing the current branch name, the stack root, and whether the branch is in the stack. If the branch is in the stack, the response SHALL include its parent.

#### Scenario: Current branch is in the stack
- **WHEN** `st_current` is called while on a tracked branch
- **THEN** the response SHALL include `"in_stack": true` and the `parent` field

#### Scenario: Current branch is the root
- **WHEN** `st_current` is called while on the root branch
- **THEN** the response SHALL include `"in_stack": true` without a `parent` field

#### Scenario: Current branch is not in the stack
- **WHEN** `st_current` is called while on an untracked branch
- **THEN** the response SHALL include `"in_stack": false`

---

## 3. Branch Creation Tools

### Requirement: New Branch from Root (st_new)
The `st_new` tool SHALL create a new branch from the stack root and check it out. The `branch_name` parameter is required. The new branch SHALL be added to the graph with the root as its parent.

#### Scenario: Create a new branch
- **WHEN** `st_new` is called with `branch_name` set to `"feature-x"`
- **THEN** a new git branch `"feature-x"` SHALL be created from the root branch
- **AND** the branch SHALL be checked out
- **AND** the graph SHALL be saved with the new branch having the root as parent
- **AND** the response text SHALL indicate the branch was created from the root

#### Scenario: Missing branch name
- **WHEN** `st_new` is called without `branch_name`
- **THEN** the tool SHALL return an error result

### Requirement: Append Child Branch (st_append)
The `st_append` tool SHALL create a new branch as a child of the current branch and check it out. The `branch_name` parameter is required. The current branch MUST be in the stack or be the root; otherwise the tool SHALL return an error.

#### Scenario: Append from a tracked branch
- **WHEN** `st_append` is called with `branch_name` while on a tracked branch
- **THEN** a new branch SHALL be created from the current branch
- **AND** the graph SHALL record the new branch with the current branch as parent

#### Scenario: Append from an untracked branch
- **WHEN** `st_append` is called while the current branch is not in the stack and is not the root
- **THEN** the tool SHALL return an error indicating the current branch is not in the stack

### Requirement: Insert Branch Before Current (st_insert)
The `st_insert` tool SHALL insert a new branch before the current branch in the stack. The current branch's parent SHALL be reparented to point to the new branch, and the current branch SHALL become a child of the new branch. Downstream branches SHALL be restacked. The tool SHALL create automatic backups before restacking.

#### Scenario: Insert a branch
- **WHEN** `st_insert` is called with `branch_name` while on a tracked branch
- **THEN** the new branch SHALL be inserted between the current branch's old parent and the current branch
- **AND** downstream branches SHALL be restacked
- **AND** the new branch SHALL be checked out after completion
- **AND** automatic backups SHALL be cleaned up on success

#### Scenario: Insert with restack conflict
- **WHEN** `st_insert` triggers a rebase conflict during restacking
- **THEN** the tool SHALL return an error indicating the conflict location
- **AND** the error message SHALL instruct the user to resolve and run `st_continue`

#### Scenario: Insert on untracked branch
- **WHEN** `st_insert` is called while the current branch is not in the stack
- **THEN** the tool SHALL return an error

---

## 4. Management Tools

### Requirement: Attach Branch to Stack (st_attach)
The `st_attach` tool SHALL attach an existing branch to the stack under a specified parent. Both `branch_name` and `parent` parameters are required. The parent MUST be either the graph root, a branch already in the stack, or a recognized trunk branch (main, master, develop, trunk).

#### Scenario: Attach a branch to a tracked parent
- **WHEN** `st_attach` is called with a valid `branch_name` and `parent` that is in the stack
- **THEN** the branch SHALL be attached as a child of the parent in the graph
- **AND** the graph SHALL be saved

#### Scenario: Attach with trunk branch as parent
- **WHEN** `st_attach` is called with a `parent` that is a trunk branch name (e.g., `"main"`) and the branch exists in git
- **THEN** the graph root SHALL be updated to the trunk branch
- **AND** the branch SHALL be attached

#### Scenario: Attach with invalid parent
- **WHEN** `st_attach` is called with a `parent` that is not in the stack and is not a trunk branch
- **THEN** the tool SHALL return an error indicating the parent is not in the stack

### Requirement: Restack Lineage (st_restack)
The `st_restack` tool SHALL rebase branches in the current lineage onto their respective parents. The optional `to_current` boolean parameter controls whether to restack the full lineage or only up to the current branch.

#### Scenario: Restack full lineage from tip
- **WHEN** `st_restack` is called while on the tip branch of a lineage (no children)
- **THEN** the full lineage SHALL be restacked
- **AND** the response SHALL indicate how many branches were restacked

#### Scenario: Restack requires to_current flag
- **WHEN** `st_restack` is called while not at the tip of the lineage and `to_current` is false
- **THEN** the tool SHALL return an error instructing the user to set `to_current=true`

#### Scenario: Restack with to_current
- **WHEN** `st_restack` is called with `to_current=true` while not at the tip
- **THEN** only ancestors up to the current branch SHALL be restacked

#### Scenario: Restack conflict
- **WHEN** a rebase conflict occurs during restacking
- **THEN** the tool SHALL save the graph and return an error indicating the conflict location
- **AND** the error message SHALL instruct the user to resolve and run `st_continue`

### Requirement: Continue After Conflict (st_continue)
The `st_continue` tool SHALL resume a restack operation after the user has resolved a rebase conflict. It takes no parameters.

#### Scenario: Continue an in-progress rebase
- **WHEN** `st_continue` is called while a rebase is in progress
- **THEN** the rebase SHALL be continued
- **AND** the restack state SHALL be cleared
- **AND** the graph SHALL be saved
- **AND** the response SHALL indicate how many branches were restacked

#### Scenario: No rebase in progress
- **WHEN** `st_continue` is called with no rebase in progress
- **THEN** the tool SHALL return an error: `"no rebase in progress -- nothing to continue"`

#### Scenario: Continued restack hits another conflict
- **WHEN** `st_continue` is called but the continued restack encounters another conflict
- **THEN** the tool SHALL return an error indicating the new conflict location

---

## 5. Git Tools -- Read Operations

### Requirement: Git Log (st_git_log)
The `st_git_log` tool SHALL display the git log in oneline format. It accepts optional `range` (string, git range spec), `limit` (number, default 20), and `stat` (boolean) parameters.

#### Scenario: Default git log
- **WHEN** `st_git_log` is called with no parameters
- **THEN** the tool SHALL return up to 20 commits in oneline format

#### Scenario: Git log with range and stat
- **WHEN** `st_git_log` is called with `range: "main..HEAD"`, `limit: 5`, `stat: true`
- **THEN** the tool SHALL return up to 5 commits in the specified range with `--stat` output

### Requirement: Git Diff (st_git_diff)
The `st_git_diff` tool SHALL show diff output. It accepts optional `staged` (boolean, default false) and `paths` (string array) parameters. If the diff is empty, the response SHALL be `"No differences"`.

#### Scenario: Unstaged diff
- **WHEN** `st_git_diff` is called with no parameters
- **THEN** the tool SHALL return the unstaged diff of the working tree

#### Scenario: Staged diff filtered to paths
- **WHEN** `st_git_diff` is called with `staged: true` and `paths: ["src/main.go"]`
- **THEN** the tool SHALL return only the staged diff for the specified paths

### Requirement: Git Diff Stat (st_git_diff_stat)
The `st_git_diff_stat` tool SHALL show `diff --stat` output against a required `ref` parameter.

#### Scenario: Diff stat against a ref
- **WHEN** `st_git_diff_stat` is called with `ref: "main"`
- **THEN** the tool SHALL return the diff stat summary comparing HEAD to `main`

### Requirement: Git Status (st_git_status)
The `st_git_status` tool SHALL show working tree status in porcelain format. If the working tree is clean, the response SHALL be `"Working tree clean"`.

#### Scenario: Dirty working tree
- **WHEN** `st_git_status` is called with uncommitted changes present
- **THEN** the tool SHALL return porcelain-formatted status output

#### Scenario: Clean working tree
- **WHEN** `st_git_status` is called with no changes
- **THEN** the tool SHALL return `"Working tree clean"`

---

## 6. Git Tools -- Write Operations

### Requirement: Git Cherry-Pick (st_git_cherry_pick)
The `st_git_cherry_pick` tool SHALL cherry-pick one or more commits onto the current branch. The `commits` parameter (string array) is required and MUST be non-empty.

#### Scenario: Cherry-pick commits
- **WHEN** `st_git_cherry_pick` is called with `commits: ["abc123", "def456"]`
- **THEN** the specified commits SHALL be cherry-picked in order onto the current branch

#### Scenario: Empty commits array
- **WHEN** `st_git_cherry_pick` is called with an empty `commits` array
- **THEN** the tool SHALL return an error: `"commits is required and must be non-empty"`

### Requirement: Git Checkout (st_git_checkout)
The `st_git_checkout` tool SHALL check out an existing branch. The `branch` parameter is required.

#### Scenario: Checkout a branch
- **WHEN** `st_git_checkout` is called with `branch: "feature-x"`
- **THEN** the branch SHALL be checked out
- **AND** the response SHALL be `"Checked out feature-x"`

### Requirement: Git Reset (st_git_reset)
The `st_git_reset` tool SHALL reset HEAD using the specified mode. The `mode` parameter is required and MUST be one of `"soft"`, `"mixed"`, or `"hard"`. The optional `ref` parameter specifies the target ref. This tool SHALL be annotated as destructive.

#### Scenario: Hard reset to a ref
- **WHEN** `st_git_reset` is called with `mode: "hard"` and `ref: "HEAD~1"`
- **THEN** HEAD SHALL be reset to the specified ref with `--hard`

#### Scenario: Mixed reset without ref
- **WHEN** `st_git_reset` is called with `mode: "mixed"` and no `ref`
- **THEN** HEAD SHALL be reset in mixed mode against the current HEAD
- **AND** the response SHALL be `"Reset complete"` if git produces no output

### Requirement: Git Add (st_git_add)
The `st_git_add` tool SHALL stage files. The `paths` parameter (string array) is required and MUST be non-empty. If git produces no output, the response SHALL be `"Files staged"`.

#### Scenario: Stage files
- **WHEN** `st_git_add` is called with `paths: ["file1.go", "file2.go"]`
- **THEN** the specified files SHALL be staged

#### Scenario: Empty paths array
- **WHEN** `st_git_add` is called with an empty `paths` array
- **THEN** the tool SHALL return an error: `"paths is required and must be non-empty"`

### Requirement: Git Commit (st_git_commit)
The `st_git_commit` tool SHALL create a commit with the given message. The `message` parameter is required.

#### Scenario: Create a commit
- **WHEN** `st_git_commit` is called with `message: "fix: resolve issue"`
- **THEN** a commit SHALL be created with the specified message
- **AND** the response SHALL contain the git commit output

---

## 7. Sync Tools

### Requirement: Sync Operation (st_sync)
The `st_sync` tool SHALL perform a full sync: fetch from remote, detect merged branches, restack, and push. It accepts optional `dry_run` (boolean) and `down_only` (boolean) parameters. The response SHALL be a JSON object with fields: `fetched`, `trunk_updated`, `merged_branches`, `pushed_branches`, `restacked_count`, and `dry_run`.

#### Scenario: Full sync
- **WHEN** `st_sync` is called with no parameters
- **THEN** the tool SHALL fetch, detect merges, restack, and push
- **AND** the response SHALL be a JSON object with sync results

#### Scenario: Dry run sync
- **WHEN** `st_sync` is called with `dry_run: true`
- **THEN** the tool SHALL report what would happen without making changes
- **AND** the response SHALL include `"dry_run": true`

#### Scenario: Down-only sync
- **WHEN** `st_sync` is called with `down_only: true`
- **THEN** the tool SHALL fetch and restack but skip pushing

#### Scenario: Sync with conflicts
- **WHEN** a rebase conflict occurs during sync
- **THEN** the response SHALL include `"conflicts": true` and `"conflicts_at"` identifying the branch

### Requirement: PR Preparation (st_pr)
The `st_pr` tool SHALL push the current branch (if not already on the remote) and return JSON with `head`, `base`, `remote_url`, and `pushed` fields. The current branch MUST be in the stack.

#### Scenario: Push and return PR info
- **WHEN** `st_pr` is called while on a tracked branch that has not been pushed
- **THEN** the branch SHALL be pushed to the remote
- **AND** the response SHALL include `"pushed": true`, the head branch, the base (parent) branch, and the remote URL

#### Scenario: Branch already pushed
- **WHEN** `st_pr` is called while on a tracked branch that already exists on the remote
- **THEN** the tool SHALL NOT push again
- **AND** the response SHALL include `"pushed": false`

#### Scenario: Branch not in stack
- **WHEN** `st_pr` is called while on a branch not in the stack
- **THEN** the tool SHALL return an error indicating the branch is not in the stack

---

## 8. Prompts

### Requirement: Split Monolithic PR Prompt
The server SHALL register a prompt named `"split-monolithic-pr"` that provides guidance for splitting a large PR into focused, stacked commits or branches. The prompt accepts optional `base_branch` (default `"main"`) and `source_branch` (default `"current branch"`) arguments. Template placeholders `{{base_branch}}` and `{{source_branch}}` SHALL be replaced with the provided argument values.

#### Scenario: Prompt with default arguments
- **WHEN** the `split-monolithic-pr` prompt is requested without arguments
- **THEN** the rendered prompt SHALL use `"main"` as the base branch and `"current branch"` as the source branch

#### Scenario: Prompt with custom arguments
- **WHEN** the `split-monolithic-pr` prompt is requested with `base_branch: "develop"` and `source_branch: "feature/big-refactor"`
- **THEN** the rendered prompt SHALL substitute the provided values into the template

### Requirement: Split Monolithic PR Resource
The server SHALL also register the split-monolithic-pr prompt as a resource at URI `"staccato://prompts/split-monolithic-pr"` with MIME type `"text/markdown"`. The resource SHALL return the raw template (unrendered, with `{{base_branch}}` and `{{source_branch}}` placeholders intact).

#### Scenario: Read resource
- **WHEN** a client reads the resource at `"staccato://prompts/split-monolithic-pr"`
- **THEN** the server SHALL return the raw markdown template with placeholders

---

## 9. Tool Annotations

### Requirement: Read-Only Tool Annotations
Tools that only read data (st_log, st_status, st_current, st_git_log, st_git_diff, st_git_diff_stat, st_git_status) SHALL be annotated with `ReadOnlyHint: true`, `DestructiveHint: false`, `IdempotentHint: true`, and `OpenWorldHint: false`.

#### Scenario: Read-only tool metadata
- **WHEN** a client lists tools
- **THEN** read-only tools SHALL have annotations indicating they are safe, idempotent, and non-destructive

### Requirement: Mutating Tool Annotations
Tools that modify state but are not destructive (st_new, st_append, st_insert, st_attach, st_restack, st_continue, st_sync, st_pr, st_git_cherry_pick, st_git_checkout, st_git_add, st_git_commit) SHALL be annotated with `ReadOnlyHint: false`, `DestructiveHint: false`, `IdempotentHint: false`, and `OpenWorldHint: false`.

#### Scenario: Mutating tool metadata
- **WHEN** a client lists tools
- **THEN** mutating tools SHALL have annotations indicating they are non-read-only and non-destructive

### Requirement: Destructive Tool Annotations
Tools that can cause data loss (st_git_reset) SHALL be annotated with `ReadOnlyHint: false`, `DestructiveHint: true`, `IdempotentHint: false`, and `OpenWorldHint: false`.

#### Scenario: Destructive tool metadata
- **WHEN** a client lists tools
- **THEN** destructive tools SHALL have annotations indicating they are destructive

---

## 10. Error Handling

### Requirement: Tool Errors as Error Results
When a tool encounters an error, it SHALL return an MCP error result (via `NewToolResultError`) with a descriptive message rather than returning a Go-level error. This ensures the MCP protocol error response is properly formatted as a tool result with `isError` semantics.

#### Scenario: Git operation failure
- **WHEN** a git operation fails within any tool handler
- **THEN** the tool SHALL return an error result containing the error message
- **AND** the Go-level error return SHALL be `nil`

#### Scenario: Required parameter missing
- **WHEN** a required parameter is not provided to a tool
- **THEN** the tool SHALL return an error result describing the missing parameter

### Requirement: Graph Persistence After Write Operations
All tools that modify the stack graph SHALL call `sc.Save()` to persist state. If saving fails, the tool SHALL return an error result.

#### Scenario: Save failure
- **WHEN** a branch creation or management tool succeeds in its git operation but fails to save the graph
- **THEN** the tool SHALL return an error result indicating the save failure
