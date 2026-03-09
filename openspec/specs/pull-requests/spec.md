# Pull Requests

## Overview

The `st pr` command group provides pull request operations that integrate with hosting providers (forges). PRs created through `st` are automatically targeted at the parent branch in the stack, ensuring correct chaining. The system delegates to the `gh` CLI for GitHub operations.

---

### Requirement: PR Command Structure

The `st pr` command SHALL be a parent command with `make` and `view` subcommands for creating and viewing pull requests respectively.

#### Scenario: Invoking st pr without subcommand

- **WHEN** the user runs `st pr` with no subcommand
- **THEN** the CLI SHALL display help text describing available subcommands

---

### Requirement: Create PR for Current Branch

The `st pr make` command SHALL create a pull request for the current branch, targeting the parent branch as defined in the stack graph. This ensures that PRs in a stack are chained so each PR shows only the diff for its own branch.

#### Scenario: Creating a PR for a tracked branch

- **WHEN** the current branch is tracked in the stack graph
- **AND** the branch has a parent branch recorded in the graph
- **THEN** `st pr make` SHALL invoke the forge to create a PR with the current branch as `head` and the parent branch as `base`

#### Scenario: Creating a PR with the web flag

- **WHEN** the user runs `st pr make --web` or `st pr make -w`
- **THEN** the forge SHALL be invoked with the `Web` option set to true
- **AND** the PR creation flow SHALL open in the user's browser

---

### Requirement: Auto-Push Before PR Creation

If the current branch has not been pushed to the remote, `st pr make` SHALL automatically push it before creating the pull request.

#### Scenario: Branch not yet pushed to remote

- **WHEN** the current branch does not exist on the remote
- **THEN** `st pr make` SHALL push the branch to the remote
- **AND** a message SHALL be printed indicating the branch is being pushed
- **AND** after the push succeeds, the PR creation SHALL proceed

#### Scenario: Branch already pushed to remote

- **WHEN** the current branch already exists on the remote
- **THEN** `st pr make` SHALL skip the push step and proceed directly to PR creation

#### Scenario: Push fails

- **WHEN** the current branch has not been pushed
- **AND** the push operation fails
- **THEN** `st pr make` SHALL return an error with the message "failed to push branch" wrapping the underlying error
- **AND** no PR creation SHALL be attempted

---

### Requirement: View PR for Current Branch

The `st pr view` command SHALL display the pull request associated with the current branch by delegating to the forge's view operation.

#### Scenario: Viewing a PR in the terminal

- **WHEN** the user runs `st pr view`
- **THEN** the forge SHALL display the PR details for the current branch in the terminal

#### Scenario: Viewing a PR in the browser

- **WHEN** the user runs `st pr view --web` or `st pr view -w`
- **THEN** the forge SHALL open the PR in the user's browser

#### Scenario: View does not require branch to be in stack

- **WHEN** the user runs `st pr view`
- **THEN** the command SHALL NOT require the current branch to be tracked in the stack graph
- **AND** it SHALL delegate directly to the forge's view operation

---

### Requirement: Forge Detection

The system SHALL detect the appropriate forge (hosting provider) by inspecting the `origin` remote URL. Only GitHub is currently supported.

#### Scenario: GitHub remote detected

- **WHEN** the origin remote URL contains "github.com"
- **THEN** the system SHALL return a GitHub forge implementation

#### Scenario: Unsupported forge

- **WHEN** the origin remote URL does not contain "github.com"
- **THEN** the system SHALL return an error with the message "forge not supported for remote" including the URL
- **AND** the message SHALL note that only GitHub is implemented

#### Scenario: No remote configured

- **WHEN** the repository has no `origin` remote
- **THEN** the system SHALL return an error with the message "failed to get remote URL" wrapping the underlying error

---

### Requirement: GitHub CLI Dependency

The GitHub forge implementation SHALL require the `gh` CLI tool to be installed. All GitHub operations (create, view, stack status) MUST check for the presence of `gh` before proceeding.

#### Scenario: gh CLI not installed

- **WHEN** any GitHub forge operation is invoked
- **AND** the `gh` CLI is not found on the system PATH
- **THEN** the operation SHALL return an error with the message "gh CLI not found" and a link to https://cli.github.com

#### Scenario: gh CLI is installed

- **WHEN** the `gh` CLI is found on the system PATH
- **THEN** the operation SHALL proceed normally

---

### Requirement: GitHub PR Creation

The GitHub forge SHALL create pull requests by invoking `gh pr create` with the correct head and base branch arguments. The command SHALL inherit stdin, stdout, and stderr from the parent process to allow interactive input (e.g., title and body prompts).

#### Scenario: Creating a PR via gh CLI

- **WHEN** `CreatePR` is called with head and base branches
- **THEN** the forge SHALL execute `gh pr create --head <head> --base <base>`
- **AND** the process stdin, stdout, and stderr SHALL be connected to the terminal

#### Scenario: Creating a PR with web flag via gh CLI

- **WHEN** `CreatePR` is called with `Web` set to true
- **THEN** the forge SHALL append the `-w` flag to the `gh pr create` command

---

### Requirement: GitHub PR Viewing

The GitHub forge SHALL view pull requests by invoking `gh pr view` for the current branch. The command SHALL inherit stdin, stdout, and stderr from the parent process.

#### Scenario: Viewing a PR via gh CLI

- **WHEN** `ViewPR` is called without the web flag
- **THEN** the forge SHALL execute `gh pr view`

#### Scenario: Viewing a PR in browser via gh CLI

- **WHEN** `ViewPR` is called with `Web` set to true
- **THEN** the forge SHALL execute `gh pr view -w`

---

### Requirement: Stack PR Status Querying

The `StackStatus` method SHALL fetch PR information for a list of branch names and return a map of branch names to their PR status. This enables the display of PR state alongside the stack tree.

#### Scenario: Querying status for stack branches

- **WHEN** `StackStatus` is called with a list of branch names
- **THEN** the forge SHALL invoke `gh pr list --state all --limit 100` with JSON fields: number, headRefName, title, state, isDraft, reviewDecision, url, statusCheckRollup
- **AND** the result SHALL contain an entry for every requested branch

#### Scenario: Branch has an associated PR

- **WHEN** a branch in the requested list matches a PR's `headRefName`
- **THEN** the corresponding `PRStatusInfo` SHALL have `HasPR` set to true
- **AND** it SHALL include the PR number, title, state, draft status, review decision, URL, and derived check status

#### Scenario: Branch has no associated PR

- **WHEN** a branch in the requested list does not match any PR
- **THEN** the corresponding `PRStatusInfo` SHALL have `HasPR` set to false
- **AND** only the `Branch` field SHALL be populated

#### Scenario: Multiple PRs exist for the same branch

- **WHEN** multiple PRs exist with the same `headRefName`
- **THEN** the system SHALL prefer the PR with the highest state priority: OPEN (3) > MERGED (2) > CLOSED (1)

---

### Requirement: PR Status Information

The `PRStatusInfo` struct SHALL capture the complete status of a PR for a given branch, including state, review status, and CI check status.

#### Scenario: PR state values

- **WHEN** a PR is fetched from GitHub
- **THEN** the `State` field SHALL be one of "OPEN", "MERGED", or "CLOSED"

#### Scenario: Review status values

- **WHEN** a PR has review information
- **THEN** the `ReviewStatus` field SHALL reflect the GitHub `reviewDecision` value (e.g., "APPROVED", "CHANGES_REQUESTED", "REVIEW_REQUIRED", or empty string)

#### Scenario: Draft PR

- **WHEN** a PR is marked as a draft on GitHub
- **THEN** the `IsDraft` field SHALL be set to true

---

### Requirement: CI Check Status Derivation

The system SHALL derive an aggregate check status from the individual status check rollup entries on a PR.

#### Scenario: No checks configured

- **WHEN** the PR has no status checks
- **THEN** the derived check status SHALL be an empty string

#### Scenario: All checks pass

- **WHEN** all status checks have a conclusion of "SUCCESS", "NEUTRAL", or "SKIPPED"
- **THEN** the derived check status SHALL be "pass"

#### Scenario: Any check fails

- **WHEN** any status check has a conclusion of "FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", or "ACTION_REQUIRED"
- **THEN** the derived check status SHALL be "fail"
- **AND** the result SHALL be returned immediately without evaluating remaining checks

#### Scenario: Checks still running

- **WHEN** no checks have failed
- **AND** at least one check has a conclusion that is not in the pass or fail sets (e.g., still pending)
- **THEN** the derived check status SHALL be "pending"

#### Scenario: Check conclusion fallback

- **WHEN** a status check has an empty `conclusion` field
- **THEN** the system SHALL use the `state` field as the conclusion value for that check

---

### Requirement: Branch Not in Stack Error

The `st pr make` command SHALL require the current branch to be tracked in the stack graph.

#### Scenario: Current branch not in stack

- **WHEN** the user runs `st pr make`
- **AND** the current branch is not tracked in the stack graph
- **THEN** the command SHALL return an error with the message "branch '<name>' is not in the stack -- run 'st attach' first"

---

### Requirement: Context Loading Failure

Both `st pr make` and `st pr view` SHALL fail gracefully if the stack context cannot be loaded.

#### Scenario: Failed to load context

- **WHEN** `getContext()` fails (e.g., not a git repository, corrupted graph)
- **THEN** the command SHALL return the error from `getContext`

#### Scenario: Failed to get current branch during make

- **WHEN** `st pr make` is run
- **AND** determining the current branch fails
- **THEN** the command SHALL return an error with the message "failed to get current branch" wrapping the underlying error
