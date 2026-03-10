## ADDED Requirements

### Requirement: Fetch inline review comments
The system SHALL fetch inline code review comments from `GET /repos/{owner}/{repo}/pulls/{number}/comments` via `gh api --paginate` for each PR associated with stack branches in scope.

#### Scenario: Fetch inline comments for a branch with a PR
- **WHEN** a stack branch has an open PR (number N)
- **THEN** the system SHALL call `gh api repos/{owner}/{repo}/pulls/{N}/comments --paginate`
- **AND** SHALL parse the JSON response extracting `user.login`, `body`, `path`, `line`, `original_line`, `diff_hunk`, `created_at`, and `in_reply_to_id` fields

#### Scenario: Branch has no PR
- **WHEN** a stack branch has no associated PR
- **THEN** the system SHALL skip fetching comments for that branch

### Requirement: Fetch review submissions
The system SHALL fetch formal review submissions from `GET /repos/{owner}/{repo}/pulls/{number}/reviews` via `gh api --paginate`.

#### Scenario: Fetch reviews for a branch with a PR
- **WHEN** a stack branch has an open PR (number N)
- **THEN** the system SHALL call `gh api repos/{owner}/{repo}/pulls/{N}/reviews --paginate`
- **AND** SHALL parse the JSON response extracting `user.login`, `body`, `state`, and `submitted_at` fields

#### Scenario: Skip empty review bodies
- **WHEN** a review has an empty `body` field
- **THEN** the system SHALL exclude that review from the output

#### Scenario: Skip pure approvals without comment
- **WHEN** a review has `state` of `APPROVED` and an empty `body`
- **THEN** the system SHALL exclude that review from the output

### Requirement: Fetch general PR comments
The system SHALL fetch general conversation comments from `GET /repos/{owner}/{repo}/issues/{number}/comments` via `gh api --paginate`.

#### Scenario: Fetch issue comments for a branch with a PR
- **WHEN** a stack branch has an open PR (number N)
- **THEN** the system SHALL call `gh api repos/{owner}/{repo}/issues/{N}/comments --paginate`
- **AND** SHALL parse the JSON response extracting `user.login`, `body`, and `created_at` fields

#### Scenario: Skip empty comment bodies
- **WHEN** an issue comment has an empty `body` field
- **THEN** the system SHALL exclude that comment from the output

### Requirement: Bot filtering
The system SHALL filter out comments from bot accounts (usernames ending in `[bot]`) unless the bot is in the allowlist of substantive review bots.

#### Scenario: Filter generic bot comments
- **WHEN** a comment author's login ends with `[bot]`
- **AND** the author is NOT in the review bot allowlist
- **THEN** the system SHALL exclude that comment from the output

#### Scenario: Keep substantive review bot comments
- **WHEN** a comment author is `coderabbitai[bot]` or `greptile-apps[bot]`
- **THEN** the system SHALL include that comment in the output
- **AND** SHALL mark it with author type `Bot`

#### Scenario: Human reviewer comments
- **WHEN** a comment author's login does NOT end with `[bot]`
- **THEN** the system SHALL include that comment
- **AND** SHALL mark it with author type `Human`

### Requirement: Reply threading
The system SHALL group inline comments into threads based on the `in_reply_to_id` field.

#### Scenario: Comment is a reply
- **WHEN** an inline comment has a non-null `in_reply_to_id`
- **THEN** the system SHALL attach it as a reply under the parent comment in the output

#### Scenario: Comment is a thread root
- **WHEN** an inline comment has a null `in_reply_to_id`
- **THEN** the system SHALL treat it as a standalone item (potentially with replies attached)

### Requirement: Scope selection
The system SHALL support three scopes for selecting which stack branches to collect reviews from.

#### Scenario: Whole stack (default)
- **WHEN** no scope flag is provided
- **THEN** the system SHALL collect reviews from all branches in the stack that have PRs

#### Scenario: Current branch only
- **WHEN** the `--current` flag is provided
- **THEN** the system SHALL collect reviews only from the currently checked-out branch's PR

#### Scenario: Ancestors to current
- **WHEN** the `--to-current` flag is provided
- **THEN** the system SHALL collect reviews from the current branch and all its ancestors in the stack (excluding the root/trunk branch)

### Requirement: Output format
The system SHALL produce a markdown document containing all collected feedback items, structured for both human reading and AI processing.

#### Scenario: Stdout output (default)
- **WHEN** no `--out` flag is provided
- **THEN** the system SHALL write the markdown to stdout

#### Scenario: File output
- **WHEN** the `--out <path>` flag is provided
- **THEN** the system SHALL write the markdown to the specified file path

#### Scenario: Output structure
- **WHEN** feedback items are collected
- **THEN** the output SHALL contain:
  - A title section with the repository and scope information
  - Items grouped by PR number
  - Each item showing: PR number, author (with Human/Bot label), type (inline/review/general), file path and line (for inline), the comment body, and diff context (for inline)
  - A classification prompt section with instructions for AI consumers to classify by severity (CRITICAL/HIGH/MEDIUM/LOW) and deduplicate

#### Scenario: No feedback found
- **WHEN** no review comments exist for any branches in scope
- **THEN** the system SHALL output a message indicating no reviews were found

### Requirement: Concurrent fetching
The system SHALL fetch API data concurrently across PRs with a maximum of 5 concurrent `gh api` calls.

#### Scenario: Multiple PRs in scope
- **WHEN** the scope includes N branches with PRs where N > 1
- **THEN** the system SHALL fetch comments for multiple PRs concurrently
- **AND** SHALL limit concurrent `gh api` invocations to 5

### Requirement: PR number resolution
The system SHALL resolve PR numbers for stack branches by using the forge's `StackStatus` method to find open PRs associated with each branch.

#### Scenario: Resolve PR for tracked branch
- **WHEN** collecting reviews for a stack branch
- **THEN** the system SHALL use `forge.StackStatus` to find the PR number for that branch
- **AND** SHALL use the `owner/repo` from the git remote URL
