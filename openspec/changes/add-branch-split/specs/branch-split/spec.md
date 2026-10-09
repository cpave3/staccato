## ADDED Requirements

### Requirement: Describe a Branch for Splitting
The `st split describe [branch]` command SHALL print a JSON inventory of a tracked branch, defaulting to the current branch, without changing anything. The inventory SHALL include the branch, its parent, the worktree, the base and head commits, its child branches, every commit between base and head, every file changed between base and head, the net totals, and the churn (the sum of what each commit changed).

#### Scenario: Describe the current branch
- **WHEN** the user runs `st split describe` on a tracked branch with three commits
- **THEN** the output SHALL be a single JSON object
- **AND** `commits` SHALL list the three commits oldest first, each with an abbreviated sha, subject, files, insertions and deletions
- **AND** `files` SHALL list each changed path with its status (`added`, `modified` or `deleted`), insertions, deletions, and the commits that touch it
- **AND** `head` SHALL be the full SHA of the branch tip

#### Scenario: Rework is visible
- **WHEN** a branch writes a file in one commit and rewrites it in a later commit
- **THEN** `churn` SHALL count both commits' changes
- **AND** `totals` SHALL count only the net change

#### Scenario: Renamed file
- **WHEN** a branch renames a file
- **THEN** `files` SHALL contain the old path as `deleted` and the new path as `added`
- **AND** the new path's entry SHALL carry `renamed_from` with the old path

#### Scenario: Branch not in the stack
- **WHEN** the user runs `st split describe` on a branch that is not tracked
- **THEN** the command SHALL fail with an error stating the branch is not in the stack

### Requirement: Split Plan Format
A split plan SHALL be a single JSON object with an optional `branch` (default: the current branch), an optional `head`, an optional `drop_original`, and a `slices` array ordered from the bottom of the stack to the top. Each slice SHALL have a `name` and exactly one content source: `commits`, `paths` (optionally with `overrides`), or `rest`. Unknown fields SHALL be rejected.

#### Scenario: Valid plan
- **WHEN** a plan has at least two uniquely named slices, each with exactly one content source
- **THEN** the plan SHALL be accepted for building

#### Scenario: Unknown field
- **WHEN** a plan contains a field that is not part of the format, such as `path` instead of `paths`
- **THEN** the plan SHALL be rejected with an error naming the field

#### Scenario: Syntax error
- **WHEN** a plan is not valid JSON
- **THEN** the error SHALL give the line and column
- **AND** SHALL say that comments are not allowed when the offending character starts a comment

#### Scenario: Fewer than two slices
- **WHEN** a plan has fewer than two slices
- **THEN** the plan SHALL be rejected

#### Scenario: Mixed or missing content source
- **WHEN** a slice has no content source, or more than one
- **THEN** the plan SHALL be rejected with an error naming the slice

#### Scenario: Message rules
- **WHEN** a `paths` or `rest` slice has no `message`, or a `commits` slice has one
- **THEN** the plan SHALL be rejected with an error naming the slice

#### Scenario: Rest is last
- **WHEN** a slice other than the last uses `rest`
- **THEN** the plan SHALL be rejected

### Requirement: Slices by Path
A `paths` slice SHALL create one commit, on top of the previous slice, in which every changed file matched by its pathspecs has the content it has at the branch's head, including files the branch deletes. Pathspecs SHALL be relative to the repository root. A pathspec SHALL claim only files that are not yet at their final content.

#### Scenario: Directory path
- **WHEN** a slice lists `core/` and the branch adds `core/types.txt`
- **THEN** that slice's commit SHALL add `core/types.txt` and change nothing else

#### Scenario: Deleted file
- **WHEN** a slice lists a file that the branch deletes
- **THEN** that slice's commit SHALL delete the file

#### Scenario: Path matches nothing
- **WHEN** a pathspec matches no file changed on the branch
- **THEN** the plan SHALL be rejected with an error naming the slice and the path

#### Scenario: Path already placed
- **WHEN** a pathspec matches only files that an earlier slice brought to their final content
- **THEN** the plan SHALL be rejected with an error naming the slice, the path, and the earlier slice

### Requirement: Slices by Commit
A `commits` slice SHALL place existing commits of the branch on top of the previous slice in the order given, keeping each commit's message and author. A commit whose parent is already in place SHALL be reused unchanged. Any other commit SHALL be applied without using the working tree.

#### Scenario: Cut at existing commit boundaries
- **WHEN** a plan assigns the branch's commits to slices in their existing order
- **THEN** each slice branch SHALL point at an existing commit
- **AND** no commit SHALL be rewritten

#### Scenario: Reordered commits
- **WHEN** a plan places a later commit below earlier ones and it applies cleanly
- **THEN** the commits SHALL be re-created in the new order with their messages and authors kept
- **AND** the top slice SHALL hold the same content as the original branch

#### Scenario: Commit does not apply
- **WHEN** a commit is placed where it conflicts with the content below it
- **THEN** the plan SHALL be rejected with an error naming the commit and the conflicting files
- **AND** nothing SHALL be changed

#### Scenario: Commit from outside the branch, or listed twice
- **WHEN** a slice lists a commit that is not on the branch, or a commit appears in two slices
- **THEN** the plan SHALL be rejected

#### Scenario: Commit made redundant
- **WHEN** a commit adds nothing because an earlier slice already took its files by path
- **THEN** the commit SHALL be left out
- **AND** the result SHALL include a warning naming it

### Requirement: Rest Slice
A `rest` slice SHALL create one commit that brings the previous slice to exactly the content of the branch's head.

#### Scenario: Rest takes what is left
- **WHEN** earlier slices take `core/` and `api/` and the last slice is `rest`
- **THEN** the last slice's commit SHALL contain every other change on the branch

### Requirement: Overrides
A slice with `paths` and/or `overrides` MAY map a changed, regular file to a content file. That slice's commit SHALL give the path the content of the content file. A later slice SHALL bring the path to its final content. Relative content files SHALL be resolved against the plan file's directory, or the current directory when the plan is read from standard input.

#### Scenario: Shared file split across two slices
- **WHEN** slice one overrides `README.md` with intermediate content and slice two lists `README.md` in `paths`
- **THEN** `README.md` SHALL have the intermediate content at slice one
- **AND** its final content at slice two

#### Scenario: Override never finished
- **WHEN** a path is overridden and no later slice brings it to its final content
- **THEN** the plan SHALL be rejected with an error naming the slice and the path

#### Scenario: Override of an unchanged or non-regular file
- **WHEN** an override names a file the branch does not change, a symlink, or a submodule
- **THEN** the plan SHALL be rejected

### Requirement: Completeness
The top slice SHALL hold exactly the content the branch had before the split. A plan for which this does not hold SHALL be rejected, and the error SHALL list the files that are missing or differ.

#### Scenario: Files left out
- **WHEN** the slices of a plan do not cover every changed file
- **THEN** the plan SHALL be rejected
- **AND** the error SHALL list each uncovered file
- **AND** SHALL suggest ending the plan with a `rest` slice

#### Scenario: Commits left out
- **WHEN** a plan built from `commits` slices omits a commit
- **THEN** the error SHALL also list the unassigned commits

#### Scenario: Empty slice
- **WHEN** a slice would add nothing to the slice below it
- **THEN** the plan SHALL be rejected with an error naming the slice

### Requirement: Atomic Apply
`st split apply` SHALL build every slice before moving any branch, and SHALL move all branches in a single ref transaction. A plan that is rejected for any reason SHALL leave branches, the stack graph, HEAD, the index and the working tree unchanged. Building slices SHALL NOT modify the working tree or the index.

#### Scenario: Successful split
- **WHEN** a valid three-slice plan is applied to a branch with parent `main`
- **THEN** the three slice branches SHALL exist, each built on the one below, with the first built on `main`
- **AND** the graph SHALL record the same chain, with each slice's base SHA equal to its parent's tip

#### Scenario: Rejected plan
- **WHEN** a plan is rejected
- **THEN** no slice branch SHALL exist
- **AND** the original branch SHALL still point at its original commit

#### Scenario: Uncommitted changes
- **WHEN** the working tree has uncommitted changes and a split is applied without `--exec`
- **THEN** the split SHALL succeed
- **AND** the uncommitted changes SHALL be unchanged

#### Scenario: Branch moved since the plan was written
- **WHEN** a plan carries a `head` that is no longer the branch's tip
- **THEN** the plan SHALL be rejected with an error telling the user to run `st split describe` again

#### Scenario: Branch behind its parent
- **WHEN** the branch does not contain its parent's tip
- **THEN** the plan SHALL be rejected with an error telling the user to restack first

### Requirement: Slice Branch Names
Slice names SHALL be valid branch names that do not already exist, except that one slice MAY reuse the name of the branch being split. If no slice reuses that name, the plan SHALL set `drop_original`, and applying it SHALL delete the original branch and remove it from the graph.

#### Scenario: Top slice keeps the name
- **WHEN** the last slice is named after the original branch
- **THEN** the original branch SHALL point at the top slice's commit
- **AND** HEAD SHALL NOT be moved

#### Scenario: Lower slice keeps the name
- **WHEN** a slice other than the last is named after the original branch, and that branch is checked out
- **THEN** the checkout SHALL end on the top slice with no file changed

#### Scenario: Name dropped without saying so
- **WHEN** no slice is named after the original branch and `drop_original` is not set
- **THEN** the plan SHALL be rejected with an error explaining both options

#### Scenario: Name dropped explicitly
- **WHEN** no slice is named after the original branch and `drop_original` is true
- **THEN** the original branch SHALL be deleted and removed from the graph

#### Scenario: Name taken, invalid, or nested
- **WHEN** a slice name already exists, is not a valid branch name, or is nested under a branch that will still exist (or the reverse)
- **THEN** the plan SHALL be rejected with an error naming the slice

### Requirement: Descendants Follow the Split
Branches stacked on the split branch SHALL be re-parented onto the top slice as part of the same ref transaction, without checking anything out. Each descendant commit SHALL keep its content, message and author. A descendant that did not contain its parent's tip before the split SHALL be left untouched, together with its own descendants, and reported.

#### Scenario: Children and grandchildren
- **WHEN** a branch with a child and a grandchild is split by paths
- **THEN** the child SHALL be built on the top slice and the grandchild on the child
- **AND** each SHALL contain exactly the changes it contained before
- **AND** the graph SHALL record their new base and head SHAs

#### Scenario: Top commit kept
- **WHEN** a split cuts at existing commit boundaries so that the top slice is the original tip
- **THEN** no descendant SHALL be rewritten

#### Scenario: Descendant checked out elsewhere
- **WHEN** a descendant is checked out in another worktree
- **THEN** it SHALL be moved
- **AND** that worktree SHALL remain clean

#### Scenario: Descendant already out of date
- **WHEN** a child does not contain the split branch's tip
- **THEN** the split SHALL succeed
- **AND** the child SHALL NOT be rewritten
- **AND** the result SHALL list it as needing a restack
- **AND** a later `st restack` SHALL be able to restack it

### Requirement: Worktree Safety
A branch checked out in another worktree SHALL only be split when the top slice keeps its name.

#### Scenario: Split from a linked worktree
- **WHEN** `st split apply` runs inside a linked worktree where the branch is checked out
- **THEN** the split SHALL succeed and that worktree SHALL remain clean

#### Scenario: Branch checked out elsewhere, top slice keeps the name
- **WHEN** the branch is checked out in another worktree and the last slice is named after it
- **THEN** the split SHALL succeed and the other worktree SHALL remain clean

#### Scenario: Branch checked out elsewhere, name moved or dropped
- **WHEN** the branch is checked out in another worktree and the last slice has a different name
- **THEN** the plan SHALL be rejected with an error naming that worktree

### Requirement: Dry Run
With `--dry-run`, `st split apply` SHALL build and check the slices and report them without moving any branch.

#### Scenario: Dry run
- **WHEN** a valid plan is applied with `--dry-run`
- **THEN** the result SHALL list each slice with its parent, commit id, commit count, file count, insertions and deletions
- **AND** the reported commits SHALL exist as objects
- **AND** no branch, graph entry or checkout SHALL change

### Requirement: Verify Each Slice
With `--exec <command>`, `st split apply` SHALL check out each slice in turn, run the command through the shell at the repository root, and apply the split only if the command succeeds in every slice. The command SHALL receive `ST_SPLIT_SLICE`, `ST_SPLIT_SHA`, `ST_SPLIT_INDEX` and `ST_SPLIT_TOTAL`. The starting checkout SHALL be restored afterwards.

#### Scenario: Command passes everywhere
- **WHEN** the command succeeds in every slice
- **THEN** the split SHALL be applied
- **AND** the command SHALL have seen each slice's own content

#### Scenario: Command fails in a slice
- **WHEN** the command fails in a slice
- **THEN** nothing SHALL be applied
- **AND** the error SHALL name the slice
- **AND** the starting checkout SHALL be restored

#### Scenario: Uncommitted changes
- **WHEN** `--exec` is used with uncommitted changes to tracked files
- **THEN** the command SHALL fail before checking anything out

#### Scenario: Command changes tracked files
- **WHEN** the command modifies tracked files
- **THEN** nothing SHALL be applied
- **AND** the modifications SHALL be saved in a stash

#### Scenario: Interrupted
- **WHEN** the process receives SIGINT or SIGTERM during verification
- **THEN** the command SHALL be stopped, nothing SHALL be applied, and the starting checkout SHALL be restored

#### Scenario: Verify without applying
- **WHEN** `--exec` and `--dry-run` are both given
- **THEN** the command SHALL run in every slice and nothing SHALL be applied

### Requirement: Backup of Rewritten Commits
When a split rewrites the branch's tip, the original branch SHALL be copied to a manual backup branch, and the result SHALL name it. A split that rewrites nothing SHALL create no backup.

#### Scenario: Split by paths
- **WHEN** a branch is split by paths
- **THEN** a branch `backup/manual/<timestamp>/<branch>` SHALL point at the original tip

#### Scenario: Split at commit boundaries
- **WHEN** a split leaves the top slice at the original tip
- **THEN** no backup branch SHALL be created

### Requirement: Split Output and Hooks
`st split apply` SHALL print a summary of the slices, moved descendants, descendants needing a restack, warnings, a removed branch, and the backup. With `--json` it SHALL print the result as a JSON object instead. Passing `-` as the plan SHALL read it from standard input. After a split is applied, a `post-branch-create` hook SHALL fire for each new slice branch, and a `post-branch-delete` hook SHALL fire when the original branch was dropped. No hook SHALL fire on a dry run.

#### Scenario: JSON result
- **WHEN** `st split apply plan.json --json` succeeds
- **THEN** the output SHALL be one JSON object with `applied`, `dry_run`, and `slices`

#### Scenario: Plan from standard input
- **WHEN** the user runs `st split apply -` with a plan on standard input
- **THEN** the plan SHALL be applied

#### Scenario: Hooks
- **WHEN** a split creates two new branches and drops the original
- **THEN** `post-branch-create` SHALL fire once for each new branch
- **AND** `post-branch-delete` SHALL fire for the original
