## ADDED Requirements

### Requirement: IsTrunkBranch detects known trunk names

`IsTrunkBranch` SHALL return `true` for `main`, `master`, `develop`, and `trunk`, and `false` for all other branch names.

#### Scenario: Known trunk names return true

- **WHEN** `IsTrunkBranch` is called with `"main"`, `"master"`, `"develop"`, or `"trunk"`
- **THEN** it SHALL return `true`

#### Scenario: Non-trunk names return false

- **WHEN** `IsTrunkBranch` is called with `"feature-x"` or `"release/1.0"`
- **THEN** it SHALL return `false`

### Requirement: Load reads graph from local file

When no shared ref exists, `Load` SHALL read the graph from `.git/stack/graph.json`.

#### Scenario: Load from existing local graph

- **WHEN** a valid `graph.json` exists at `.git/stack/graph.json`
- **AND** no shared ref exists
- **THEN** `Load` SHALL return a `StaccatoContext` with the graph from that file

### Requirement: Load reads graph from shared ref

When a shared ref exists at `refs/staccato/graph`, `Load` SHALL read and unmarshal the graph from that ref.

#### Scenario: Load from shared ref

- **WHEN** a valid graph blob exists at `refs/staccato/graph`
- **THEN** `Load` SHALL return a `StaccatoContext` with the graph from the ref
- **AND** the graph's `Branches` map SHALL be initialized (not nil)

### Requirement: Load creates new graph when none exists

When neither local file nor shared ref contains a graph, `Load` SHALL create a new graph with the current branch as root.

#### Scenario: Fallback to new graph

- **WHEN** no `graph.json` file exists
- **AND** no shared ref exists
- **THEN** `Load` SHALL return a `StaccatoContext` with a new graph whose root is the current branch

### Requirement: Load fails outside a git repository

`Load` SHALL return an error when called from outside a git repository.

#### Scenario: Not a git repo

- **WHEN** `Load` is called with a path that is not inside a git repository
- **THEN** it SHALL return an error containing `"not a git repository"`

### Requirement: Save persists graph to correct storage

`Save` SHALL write to the shared ref if it exists, otherwise to the local file.

#### Scenario: Save to local file

- **WHEN** no shared ref exists
- **AND** `Save` is called
- **THEN** the graph SHALL be written to `.git/stack/graph.json`

#### Scenario: Save to shared ref

- **WHEN** a shared ref exists at `refs/staccato/graph`
- **AND** `Save` is called
- **THEN** the graph SHALL be updated in the shared ref
