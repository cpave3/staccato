# Graph Storage

This specification covers how Staccato persists and manages the stack graph, including the two storage modes (local file and shared git ref), the commands to switch between them, and the graph data model itself.

## Storage Modes

### Requirement: Local Storage Mode

The local storage mode SHALL persist the graph as a JSON file at `.git/stack/graph.json` relative to the repository root. This is the default storage mode used when no shared ref exists. The directory `.git/stack/` SHALL be created automatically if it does not exist. The file SHALL be written with `0644` permissions and the directory with `0755` permissions.

#### Scenario: Graph is saved in local mode
- **WHEN** the shared ref `refs/staccato/graph` does not exist
- **THEN** the graph SHALL be written as indented JSON to `.git/stack/graph.json`
- **AND** the parent directory SHALL be created if it does not exist

#### Scenario: Graph is loaded from local file
- **WHEN** the shared ref does not exist and `.git/stack/graph.json` exists
- **THEN** the graph SHALL be deserialized from that file
- **AND** the `Branches` map SHALL be initialized to an empty map if it is nil after deserialization

### Requirement: Shared Storage Mode

The shared storage mode SHALL persist the graph as a blob stored at the git ref `refs/staccato/graph`. This mode enables the graph to be pushed to and fetched from a remote, allowing teammates to share the stack graph. When the shared ref exists, it SHALL take precedence over the local file for both loading and saving.

#### Scenario: Graph is saved in shared mode
- **WHEN** the shared ref `refs/staccato/graph` already exists
- **THEN** the graph SHALL be serialized as indented JSON and written to the ref via `WriteBlobRef`
- **AND** no local file SHALL be written

#### Scenario: Graph is loaded from shared ref
- **WHEN** the shared ref `refs/staccato/graph` exists
- **THEN** the graph SHALL be deserialized from the blob data at that ref
- **AND** the `Branches` map SHALL be initialized to an empty map if it is nil after deserialization

## Graph Commands

### Requirement: Share Graph (`st graph share`)

The `st graph share` command SHALL convert the graph from local storage to shared storage. It MUST read the local file at `.git/stack/graph.json`, validate that it contains valid JSON conforming to the `Graph` structure, write the data to `refs/staccato/graph`, and remove the local file. If a remote is configured, it SHALL add the fetch refspec `+refs/staccato/*:refs/staccato/*` unless one matching `refs/staccato` is already present.

#### Scenario: Converting local graph to shared
- **WHEN** the user runs `st graph share` and the graph is currently stored locally
- **THEN** the graph data SHALL be written to `refs/staccato/graph`
- **AND** the local file at `.git/stack/graph.json` SHALL be removed
- **AND** the command SHALL print a message indicating the graph is shared

#### Scenario: Share when already shared
- **WHEN** the user runs `st graph share` and `refs/staccato/graph` already exists
- **THEN** the command SHALL return an error with the message "graph is already shared (stored at refs/staccato/graph)"

#### Scenario: Share when no local graph exists
- **WHEN** the user runs `st graph share` and no local file exists at `.git/stack/graph.json`
- **THEN** the command SHALL return an error indicating no local graph was found

#### Scenario: Share configures fetch refspec when remote exists
- **WHEN** the graph is shared and a remote is configured
- **AND** no fetch refspec matching `refs/staccato` is already present
- **THEN** the fetch refspec `+refs/staccato/*:refs/staccato/*` SHALL be added to the remote configuration

#### Scenario: Share skips fetch refspec when already configured
- **WHEN** the graph is shared and a remote is configured
- **AND** a fetch refspec matching `refs/staccato` is already present
- **THEN** no duplicate refspec SHALL be added

#### Scenario: Share with invalid local graph
- **WHEN** the user runs `st graph share` and the local file contains invalid JSON
- **THEN** the command SHALL return an error indicating the local graph is invalid

### Requirement: Localize Graph (`st graph local`)

The `st graph local` command SHALL convert the graph from shared storage back to local storage. It MUST read the blob data from `refs/staccato/graph`, write it to `.git/stack/graph.json`, create the directory if needed, and delete the shared ref. If a remote is configured, it SHALL remove the fetch refspec `+refs/staccato/*:refs/staccato/*` if one matching `refs/staccato` is present.

#### Scenario: Converting shared graph to local
- **WHEN** the user runs `st graph local` and the graph is currently shared
- **THEN** the graph data SHALL be written to `.git/stack/graph.json`
- **AND** the directory `.git/stack/` SHALL be created if it does not exist
- **AND** the shared ref `refs/staccato/graph` SHALL be deleted
- **AND** the command SHALL print a message indicating the graph was moved to local storage

#### Scenario: Local when already local
- **WHEN** the user runs `st graph local` and no shared ref exists
- **THEN** the command SHALL return an error with the message "graph is already local (no shared ref found)"

#### Scenario: Local removes fetch refspec when remote exists
- **WHEN** the graph is localized and a remote is configured
- **AND** a fetch refspec matching `refs/staccato` is present
- **THEN** the fetch refspec `+refs/staccato/*:refs/staccato/*` SHALL be removed from the remote configuration

#### Scenario: Local skips refspec removal when not configured
- **WHEN** the graph is localized and a remote is configured
- **AND** no fetch refspec matching `refs/staccato` is present
- **THEN** no refspec removal SHALL be attempted

### Requirement: Query Storage Mode (`st graph which`)

The `st graph which` command SHALL display the current storage mode of the graph. It MUST check whether the shared ref `refs/staccato/graph` exists and report accordingly.

#### Scenario: Graph is in shared mode
- **WHEN** the user runs `st graph which` and the shared ref exists
- **THEN** the command SHALL print "Shared (refs/staccato/graph)"

#### Scenario: Graph is in local mode
- **WHEN** the user runs `st graph which` and the shared ref does not exist
- **THEN** the command SHALL print "Local (.git/stack/graph.json)"

## Graph Data Model

### Requirement: Graph Structure

A `Graph` SHALL contain a `Version` integer field, a `Root` string field identifying the root branch name, and a `Branches` map from branch name to `Branch` struct. The `Branch` struct SHALL contain `Name`, `Parent`, `BaseSHA`, and `HeadSHA` string fields. All fields SHALL be serialized as JSON using snake_case keys (`version`, `root`, `branches`, `name`, `parent`, `base_sha`, `head_sha`).

#### Scenario: Creating a new graph
- **WHEN** a new graph is created with a root branch name
- **THEN** the `Version` field SHALL be set to the current version (1)
- **AND** the `Root` field SHALL be set to the provided branch name
- **AND** the `Branches` map SHALL be initialized as an empty map

### Requirement: AddBranch

The `AddBranch` method SHALL add a branch to the graph's `Branches` map, keyed by the branch name. The branch entry SHALL store the name, parent, base SHA, and head SHA.

#### Scenario: Adding a branch
- **WHEN** `AddBranch` is called with a name, parent, baseSHA, and headSHA
- **THEN** a `Branch` entry SHALL be stored in the `Branches` map under the given name
- **AND** the entry SHALL contain all four provided values

#### Scenario: Overwriting an existing branch
- **WHEN** `AddBranch` is called with a name that already exists in the map
- **THEN** the existing entry SHALL be replaced with the new values

### Requirement: RemoveBranch

The `RemoveBranch` method SHALL delete the branch entry from the `Branches` map by name. It SHALL not return an error if the branch does not exist.

#### Scenario: Removing an existing branch
- **WHEN** `RemoveBranch` is called with a branch name that exists in the map
- **THEN** that entry SHALL be removed from the `Branches` map

#### Scenario: Removing a non-existent branch
- **WHEN** `RemoveBranch` is called with a branch name that does not exist
- **THEN** the operation SHALL complete without error

### Requirement: GetChildren

The `GetChildren` method SHALL return all branches whose `Parent` field matches the given parent name. The returned slice MAY be in any order. If no children exist, the result SHALL be nil or an empty slice.

#### Scenario: Branch has children
- **WHEN** `GetChildren` is called with a parent name that has child branches
- **THEN** all branches with that parent SHALL be returned

#### Scenario: Branch has no children
- **WHEN** `GetChildren` is called with a parent name that has no child branches
- **THEN** a nil or empty slice SHALL be returned

### Requirement: ReparentChildren

The `ReparentChildren` method SHALL update the `Parent` field of every branch whose current parent matches the given branch name, setting it to the new parent.

#### Scenario: Reparenting children
- **WHEN** `ReparentChildren` is called with a branch name and a new parent
- **THEN** all branches whose `Parent` equals the branch name SHALL have their `Parent` updated to the new parent
- **AND** branches with other parents SHALL not be affected

### Requirement: ValidateNoCycle

The `ValidateNoCycle` method SHALL check whether setting `parentName` as the parent of `branchName` would create a cycle. It MUST walk up the parent chain from `parentName` and return an error if `branchName` is encountered. If the parent chain terminates (reaches a branch not in the map or an empty parent), it SHALL return nil indicating no cycle.

#### Scenario: No cycle exists
- **WHEN** `ValidateNoCycle` is called and the parent chain does not contain the branch name
- **THEN** the method SHALL return nil

#### Scenario: Cycle detected
- **WHEN** `ValidateNoCycle` is called and walking up from the proposed parent eventually reaches the branch name
- **THEN** the method SHALL return an error with a message indicating the cycle

### Requirement: UpdateBranch

The `UpdateBranch` method SHALL update the `BaseSHA` and `HeadSHA` of an existing branch. If the branch does not exist in the map, the operation SHALL be a no-op.

#### Scenario: Updating an existing branch
- **WHEN** `UpdateBranch` is called with a name that exists in the map
- **THEN** the branch's `BaseSHA` and `HeadSHA` SHALL be updated to the new values
- **AND** the `Name` and `Parent` fields SHALL remain unchanged

#### Scenario: Updating a non-existent branch
- **WHEN** `UpdateBranch` is called with a name that does not exist in the map
- **THEN** no entry SHALL be created and no error SHALL occur

### Requirement: GetBranch

The `GetBranch` method SHALL retrieve a branch by name from the `Branches` map. It SHALL return the branch pointer and a boolean indicating whether the branch was found.

#### Scenario: Branch exists
- **WHEN** `GetBranch` is called with a name that exists in the map
- **THEN** the method SHALL return the branch pointer and `true`

#### Scenario: Branch does not exist
- **WHEN** `GetBranch` is called with a name that does not exist in the map
- **THEN** the method SHALL return nil and `false`

## Context Loading

### Requirement: Context Load Priority

The `Load` function SHALL discover the git repository root and load the graph using a priority-based fallback strategy. It MUST first check whether the shared ref `refs/staccato/graph` exists; if so, load from the ref. Otherwise, it SHALL attempt to load from the local file at `.git/stack/graph.json`. If neither source is available, it SHALL create a new graph with the current branch as the root.

#### Scenario: Shared ref exists
- **WHEN** the context is loaded and the shared ref `refs/staccato/graph` exists
- **THEN** the graph SHALL be loaded from the shared ref
- **AND** the local file SHALL not be read

#### Scenario: No shared ref, local file exists
- **WHEN** the context is loaded and the shared ref does not exist but `.git/stack/graph.json` exists
- **THEN** the graph SHALL be loaded from the local file

#### Scenario: No shared ref, no local file
- **WHEN** the context is loaded and neither the shared ref nor the local file exists
- **THEN** a new graph SHALL be created with the current branch as the root
- **AND** the `Version` SHALL be set to the current version

#### Scenario: Not a git repository
- **WHEN** the context is loaded from a path that is not inside a git repository
- **THEN** the function SHALL return an error indicating it is not a git repository

### Requirement: Context Save Routing

The `Save` method on `StaccatoContext` SHALL persist the graph to the appropriate storage backend. It MUST check whether the shared ref exists at save time: if it does, write to the ref; otherwise, write to the local file.

#### Scenario: Saving when shared ref exists
- **WHEN** `Save` is called and `refs/staccato/graph` exists
- **THEN** the graph SHALL be serialized and written to the shared ref

#### Scenario: Saving when shared ref does not exist
- **WHEN** `Save` is called and `refs/staccato/graph` does not exist
- **THEN** the graph SHALL be saved to `.git/stack/graph.json`
