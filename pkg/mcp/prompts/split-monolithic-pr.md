# Splitting a Large Branch into a Reviewable Stack

You are helping split one large branch into several smaller stacked branches, so each can be reviewed on its own.

## Context

- Base branch: {{base_branch}}
- Source branch: {{source_branch}}

Your job is to decide where the boundaries go. Staccato does the mechanics: you describe the split in a JSON plan, and it builds every slice, checks the result, and moves the branches in one step. You never reset, stage, cherry-pick, or rebase by hand.

Two guarantees hold for every split:

- **Nothing is lost.** The top slice must end with exactly the content the branch had. A plan that leaves something out, or changes anything, is refused.
- **Nothing is half-done.** Every slice is built before any branch moves. A plan that cannot be carried out changes nothing, so you can fix the plan and try again.

## Step 0: Make sure the branch is in the stack

The branch must be tracked by staccato and up to date with its parent.

- Not tracked yet: `st_attach` with `branch_name` and `parent` (`st attach <branch> --parent <parent>`).
- Behind its parent: `st_restack` (`st restack`) first.

## Step 1: Read the inventory

Call `st_split_describe` with the branch name (`st split describe [branch]`). It returns JSON with every commit, every changed file, and which commits touch which files. Keep `head`: the plan uses it.

What to look for:

- **`churn` against `totals`.** `totals` is the net change. `churn` adds up what every commit did. If churn is much larger, the commits rework each other (code written and later deleted, a cleanup commit at the end). Cutting at commit boundaries would then show reviewers code that a later slice removes, so split by **paths** instead.
- **`files[].commits`.** A file touched by several commits ties those commits together.
- **`files[].renamed_from`.** A rename is listed as a deleted path and an added path. Put both in the same slice.
- **`commits[].merge`.** Merge commits cannot be assigned by commit; use paths for that branch.
- **`children`.** Branches stacked on top are moved onto the top slice for you.

Read the diff itself where the inventory is not enough to judge a boundary.

## Step 2: Decide the boundaries

Aim for three to five slices that each tell one story. Order them so that each builds on the ones below it, usually:

1. Infrastructure and setup: migrations, configuration
2. Foundation: models, shared types, utilities
3. Core logic: services, business rules
4. API layer: endpoints, controllers
5. UI
6. Tests, with the code they cover unless there is a reason to separate them

Each slice must be:

- Reviewable on its own, with one clear concern
- Buildable and testable on its own: the branch is the unit of correctness

### Changes that must stay together

Work through these before grouping anything by preference. A slice that breaks one of them will not compile.

- **A changed function signature** goes with every call site.
- **A new type, constant, or interface** goes in the same slice as its first use, or an earlier one.
- **An import** goes with the code that needs it.
- **A deleted function or type** goes in the same slice as the removal of its last caller, or a later one.

If a forced grouping spans two slices you wanted to keep apart, merge the slices.

### Choosing how to describe each slice

| The branch looks like | Use |
|---|---|
| Clean, thematic commits already in a sensible order | `commits` |
| Messy history, rework, or a single large commit | `paths` |
| One commit mixes two concerns | `paths` for the whole plan |

Mixing is allowed, but `paths` means the **final** version of each file. A commit placed after a `paths` slice is replayed on top of it and is refused if it does not apply cleanly. When in doubt, use `paths` throughout.

### A file whose changes belong to two slices

Typical cases: a README, a registration or routing file, a shared types file.

- Usually simplest: give the whole file to the earliest slice that needs it, as long as that version does not refer to code from a later slice.
- Otherwise use an **override**: write the content the file should have at the earlier slice to a scratch file, and name it in that slice's `overrides`. A later slice then lists the path to bring it to its final content.

## Step 3: Write the plan

List slices from the bottom of the stack (nearest the parent) to the top.

```json
{
  "branch": "feature-big",
  "head": "<head from st_split_describe>",
  "slices": [
    {
      "name": "feature-big-core",
      "message": "feat: add order model and validation",
      "paths": ["pkg/core/", "go.mod"],
      "overrides": {"README.md": "/tmp/split/README.core.md"}
    },
    {
      "name": "feature-big-api",
      "message": "feat: expose orders over HTTP",
      "paths": ["pkg/api/", "README.md"]
    },
    {
      "name": "feature-big",
      "message": "feat: order list page",
      "rest": true
    }
  ]
}
```

Rules:

- A slice takes its content from exactly one of `paths` (optionally with `overrides`), `commits`, or `"rest": true`.
- `paths` are pathspecs relative to the repository root. A directory takes every changed file under it that no earlier slice has taken. Needs a `message`.
- `commits` are existing commits of the branch, kept with their message and author, in the order you list them. No `message`.
- `"rest": true` takes everything the earlier slices left out. Last slice only. Needs a `message`. Ending with it is the easy way to be complete.
- `head` makes the split refuse to run if the branch has moved since you described it. Include it.
- **Name the top slice after the original branch.** Its pull request and its child branches then stay attached. To give every slice a new name and delete the original, set `"drop_original": true`.
- A slice name cannot be nested under a branch that still exists: `feature/core` cannot exist next to `feature`.
- JSON does not allow comments.

Write commit messages that describe that slice's change, not "part 2 of the split". Use conventional prefixes: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`.

## Step 4: Dry run

Call `st_split_apply` with the plan and `dry_run: true` (`st split apply plan.json --dry-run`).

Nothing moves. You get each slice's size and a commit id you can inspect, for example `git show --stat <sha>`. Check that the sizes are balanced and that each slice holds what you intended. If the plan is refused, the message says what to change.

Show the user the proposed slices before applying if they have not seen the boundaries.

## Step 5: Verify that each slice builds

A slice that does not build cannot go through CI on its own.

From a shell, pass the project's build or test command:

```bash
st split apply plan.json --exec "<build or test command>"
```

Each slice is checked out in turn and the command runs in it. The split is applied only if the command succeeds in every slice; otherwise nothing changes and the failing slice is named. This needs a working tree without uncommitted changes. Add `--dry-run` to verify without applying.

`--exec` is not available through the MCP tools. Without a shell, build each commit id from the dry run yourself.

When a slice fails, the usual cause is a file that belongs in an earlier slice. Move it, then run again.

## Step 6: Apply

Call `st_split_apply` without `dry_run` (`st split apply plan.json`).

The result lists the new branches, any descendants that were moved onto the top slice, and:

- `backup`: a branch holding the original commits, kept whenever the split rewrote them. Leave it in place and tell the user its name. They can remove it later with `st backup clean` (interactive) or `git branch -D <backup>`.
- `needs_restack`: descendants that were already behind their parent before the split. They were left untouched; run `st_restack` on them.
- `warnings`: for example a commit that added nothing because an earlier slice already took its files.

## Step 7: Push and open pull requests

1. `st_log` to confirm the shape of the stack.
2. `st_sync` to push every branch.
3. `st_pr` for each new branch, bottom to top. Each pull request targets its parent branch.

If the original branch already had a pull request, it now belongs to the top slice. Change its base to the slice below it, otherwise it still shows the whole change.

## If something goes wrong

- **The plan is refused**: nothing changed. Read the message, fix the plan, run again.
- **The split is applied and you want it undone**: the `backup` branch from the result holds the original commits. Tell the user rather than rewriting branches by hand.
- **A slice needs a small fix afterwards**: check it out, commit the fix, and `st_restack`. Use `st modify` to amend instead.
