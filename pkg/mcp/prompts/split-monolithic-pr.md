# Splitting Changes into Focused Commits

You are helping split a large set of changes into focused, logical commits. This can be used for stacked PRs (multiple branches for team review) or cleaning up history on a single branch.

## Context

- Base branch: {{base_branch}}
- Source branch: {{source_branch}}

## Mode Selection

Choose your approach based on the goal:

- **Stacked PRs**: Create multiple branches with `st new`/`st append` for independent review of each piece
- **Single branch**: Split into sequential commits on the current branch for clean history

The techniques below apply to both modes. When instructions reference branches, single-branch mode uses commits on the same branch instead.

## Step 1: Analyze the Changes

Use `st_git_log` with `stat: true` to see all commits and files changed.
Look for:

- Logical groupings by feature or subsystem
- Dependencies between changes (see "Atomic Change Groups" in Step 2)
- Natural split points

**Determine your approach:**

- If commits are already logically organized -> use **cherry-pick** (Option A)
- If commits are messy or need splitting -> use **soft reset** (Option B)

## Step 2: Plan the Stack

Group changes by these principles (in priority order):

1. **Infrastructure/setup** - Database migrations, config changes
2. **Foundation code** - Models, utilities, shared components
3. **Core features** - Business logic, services
4. **API layer** - Endpoints, controllers
5. **UI components** - Frontend changes
6. **Tests** - Can be grouped with their feature or separate

Each **branch** must be:

- Independently reviewable
- Buildable and testable on its own
- Focused on one logical concern

Individual commits within a branch do **not** need to compile. The branch as a whole is the unit of correctness. This means you can commit files freely and fix up compilation issues with additional commits before moving to the next branch.

### Atomic Change Groups

Before choosing voluntary groupings, identify **forced groupings** — changes that _must_ land on the same branch to keep that branch compilable:

- **Function/method signature changes** must include all callers on the same branch. If you change a function's parameters, every call site must be on that branch or the branch won't compile.
- **New types, constants, or interfaces** must be on the same branch as (or an earlier branch than) their first usage.
- **Import/require additions** must accompany the code that uses them.
- **Deletion of a function/type** must be on the same branch as (or a later branch than) the removal of all callers.

Work through these constraints first, then arrange the remaining changes into logical groups. If a forced grouping spans what you'd otherwise split into separate branches, merge those branches.

### Commit Message Conventions

Each commit message should describe the "what and why" of that specific change, not the overall feature. Use conventional commit prefixes:

- `feat:` — new functionality
- `fix:` — bug fix
- `refactor:` — restructuring without behavior change
- `test:` — adding or updating tests
- `docs:` — documentation changes
- `chore:` — build, tooling, or config changes

Example: `feat: add graph storage mode for shared commit data` rather than `part 3 of PR split`.

## Step 3: Create the Stack

Starting from base, create your stack:

```bash
# First branch (from trunk)
st new feature/01-database-schema

# Subsequent branches (from current)
st append feature/02-models
st append feature/03-api
st append feature/04-ui
```

## Step 4: Populate Each Branch

There are two approaches. Choose based on your situation:

### Option A: Cherry-Pick (when commits are already well-organized)

Use when each commit cleanly maps to one stack branch:

1. `st_git_checkout` to the branch
2. `st_git_cherry_pick` the relevant commits
3. Verify the branch builds

If cherry-pick conflicts:

- Resolve manually
- Run `git cherry-pick --continue`

### Option B: Soft Reset (when commits need to be split/reorganized)

Use when a single commit contains changes for multiple branches, or when you want cleaner history:

1. **Note the original SHA** — run `st_git_log` and record the current HEAD SHA for recovery
2. `st_git_checkout` to the source branch
3. `st_git_reset` with `mode: "soft"` and `ref` pointing to base branch
   - This moves HEAD to the base but keeps all changes **staged**
4. `st_git_reset` with `mode: "mixed"` (no ref needed)
   - This **unstages everything**, leaving all changes in the working tree as unstaged modifications
5. `st_git_status` to see all changed files as unstaged modifications
6. For each branch (in dependency order):
   - `st_git_checkout` to the target branch — uncommitted changes stay in the working tree when there are no conflicts. With stacked branches, `st append` creates the next branch from the current commit, so uncommitted changes carry forward cleanly.
   - `st_git_add` whole files that belong on this branch
   - `st_git_commit` with a descriptive message
   - If a file has changes belonging to **different branches**, see "Partial File Staging" below
   - Repeat adding and committing until all files for this branch are included
   - **Verify the branch compiles** — build and run tests. If something is missing or broken, add fixup commits until the branch is green.
   - Move to the next branch

#### Working Tree State

During a soft-reset split, the working tree contains ALL changes simultaneously. This means:

- The working tree won't compile (it's a mix of old and new code from all branches)
- You cannot run tests from the working tree directly
- Build verification happens **after committing all files for a branch**, not from the working tree
- This is expected — don't be alarmed by errors in unstaged files

#### Partial File Staging

Most files belong entirely to one branch and can be staged with a simple `st_git_add`. Partial file staging is only needed when a single file has changes destined for **different branches** — for example, a cleanup (branch 1) and a feature change (branch 2) in the same file.

**Pattern: Save, checkout base, apply, stage, restore**

1. Save the full modified file: copy its contents or `st_git_stash` with `push: true`
2. `st_git_checkout` the file from the base branch to get the original version
3. Manually apply only the changes for this branch (edit the file to include just the relevant hunks)
4. `st_git_add` the file
5. Restore the full modified version from stash or your saved copy

The remaining changes stay in the working tree for later branches.

Benefits of soft reset:

- Can split a single commit into multiple logical commits
- Creates cleaner history (no "cherry-pick from X" markers)
- More control over exactly what goes where
- Can reorganize code that was poorly structured

Caveats:

- Requires careful tracking of which changes belong where
- Use `st_git_diff` and `st_git_status` frequently to verify state

## Step 5: Verify Completeness

After all commits are made, verify nothing was lost:

```bash
git diff <original-sha>..HEAD
```

This should produce **empty output**. If it shows differences, changes were lost or altered during the split. Go back and fix the relevant commit.

Also run `st_git_status` to confirm no unstaged changes remain.

## Step 6: Verify and Push

1. `st_log` - Verify stack structure
2. `st_restack` - Ensure all branches are rebased correctly
3. `st_sync` - Push all branches

## Step 7: Create PRs

For each branch (bottom to top):

- `st_pr` creates a PR targeting the parent branch
- Add description explaining this piece of the feature

## Tips

- **Naming**: Use numbered prefixes (`01-`, `02-`) for ordering
- **Conflicts**: If `st_restack` conflicts, resolve and `st_continue`
- **Merged branches**: `st_sync` auto-detects and removes merged branches
- **Iteration**: Edit earlier branches, then `st_restack` to propagate
- **Recovery**: If the split goes wrong, `git reset --hard <original-sha>` restores the original state. Always note the SHA before starting.
- **Verify often**: Use `st_git_status` and `st_git_diff` to check state before each commit
