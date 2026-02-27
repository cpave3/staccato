# Splitting a Monolithic PR into Stacked PRs

You are helping split a large PR into a logical stack of smaller, focused PRs using Staccato.

## Context
- Base branch: {{base_branch}}
- Source branch: {{source_branch}}

## Step 1: Analyze the Changes

Use `st_git_log` with `stat: true` to see all commits and files changed.
Look for:
- Logical groupings by feature or subsystem
- Dependencies between changes
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

Each branch should be:
- Independently reviewable
- Buildable and testable on its own
- Focused on one logical concern

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
3. Verify it builds

If cherry-pick conflicts:
- Resolve manually
- Run `git cherry-pick --continue`

### Option B: Soft Reset (when commits need to be split/reorganized)

Use when a single commit contains changes for multiple branches, or when you want cleaner history:

1. `st_git_checkout` to the source branch
2. `st_git_reset` with `mode: "soft"` and `ref` pointing to base branch
   - This unwinds all commits but keeps changes staged
3. `st_git_status` to see all changed files
4. `st_git_checkout` to first stack branch
5. For each logical group:
   - `st_git_add` specific files/paths for this branch
   - `st_git_commit` with descriptive message
   - `st_git_checkout` to next branch (changes carry forward)
   - Repeat until all changes are committed

Benefits of soft reset:
- Can split a single commit into multiple logical commits
- Creates cleaner history (no "cherry-pick from X" markers)
- More control over exactly what goes where
- Can reorganize code that was poorly structured

Caveats:
- Requires careful tracking of which changes belong where
- Use `st_git_diff` and `st_git_status` frequently to verify state

## Step 5: Verify and Push

1. `st_log` - Verify stack structure
2. `st_restack` - Ensure all branches are rebased correctly
3. `st_sync` - Push all branches

## Step 6: Create PRs

For each branch (bottom to top):
- `st_pr` creates a PR targeting the parent branch
- Add description explaining this piece of the feature

## Tips

- **Naming**: Use numbered prefixes (`01-`, `02-`) for ordering
- **Conflicts**: If `st_restack` conflicts, resolve and `st_continue`
- **Merged branches**: `st_sync` auto-detects and removes merged branches
- **Iteration**: Edit earlier branches, then `st_restack` to propagate
- **Soft reset safety**: Before soft resetting, note the original branch SHA so you can recover if needed
- **Verify often**: Use `st_git_status` and `st_git_diff` to check state before each commit
