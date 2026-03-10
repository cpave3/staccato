## ADDED Requirements

### Requirement: PR Reviews Tool (st_reviews)
The MCP server SHALL expose an `st_reviews` tool that collects and returns PR review feedback for stack branches.

#### Scenario: Call st_reviews with default scope
- **WHEN** `st_reviews` is called with no parameters
- **THEN** the tool SHALL collect reviews from all stack branches with PRs
- **AND** SHALL return the unified markdown feedback document as the tool result text

#### Scenario: Call st_reviews with current scope
- **WHEN** `st_reviews` is called with `scope` parameter set to `"current"`
- **THEN** the tool SHALL collect reviews only from the current branch's PR

#### Scenario: Call st_reviews with to-current scope
- **WHEN** `st_reviews` is called with `scope` parameter set to `"to-current"`
- **THEN** the tool SHALL collect reviews from the current branch and all ancestor branches' PRs

#### Scenario: Call st_reviews with out parameter
- **WHEN** `st_reviews` is called with an `out` parameter specifying a file path
- **THEN** the tool SHALL write the feedback document to that file path
- **AND** SHALL return a confirmation message as the tool result

#### Scenario: No forge detected
- **WHEN** `st_reviews` is called and no forge can be detected
- **THEN** the tool SHALL return an error result indicating forge detection failed

#### Scenario: Tool annotation
- **WHEN** `st_reviews` is registered
- **THEN** the tool SHALL have a read-only, non-destructive, idempotent annotation
- **AND** SHALL have an open-world hint set to true (makes network calls)
