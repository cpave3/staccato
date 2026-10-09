## Context

Stack state is a parent-pointer graph (`pkg/graph`) plus ordinary git branches. Commands that change stack shape (`insert`, `move`, `modify`) rebase through the restack engine, which checks out each branch in turn and can stop on a conflict, saving a lineage for `st continue`.

Branches that need splitting come in three shapes, taken from a real stack: clean thematic commits with one mixed commit; many commits that rework each other, where about a third of what was written is later deleted; and a single large commit. Only the first can be cut at commit boundaries. A shared file such as a README is often edited by several intended slices.

## Goals / Non-Goals

**Goals:**
- The agent's only job is choosing boundaries
- A split cannot lose or alter changes
- A split cannot leave the repository half-done
- Cuts at commit, file and partial-file granularity
- Descendant branches keep working
- One implementation behind both the CLI and MCP

**Non-Goals:**
- Choosing boundaries automatically
- An interactive, stateful split session
- Undoing an applied split (the original commits are kept on a backup branch)
- Merging slices back together
- Retargeting pull requests

## Decisions

### 1. Declarative plan, not an interactive session

**Decision**: `st split describe` emits an inventory; the agent writes a JSON plan; `st split apply` carries it out in one call.

**Rationale**: A session (reset, take files, commit, repeat) needs saved state with abort and resume, moves the working tree repeatedly, and has a half-split state by design. A plan can be validated whole, previewed with `--dry-run`, shown to a human, and applied atomically.

**Alternative**: Session commands (`start` / `take` / `commit` / `finish`). Rejected for the reasons above. Its one advantage, a working tree equal to the current slice so it can be built in place, is covered by `--exec`.

### 2. A split is a chain of commits whose last tree equals the original tree

**Decision**: Every slice kind reduces to "produce the next commit on top of the previous one". After building, the top commit's tree must equal the original tip's tree, or the plan is refused with the differing files listed.

**Rationale**: One invariant gives completeness (nothing lost, nothing altered) and makes descendants trivial (decision 4). Slice kinds can be mixed in one plan because they share this model.

### 3. Build slices with plumbing in a scratch index

**Decision**: `paths` slices are built with `GIT_INDEX_FILE` pointing at a temporary index: `read-tree`, `update-index --index-info` with the mode and blob of each matched path at the original tip (or a removal record for a deleted path), `write-tree`, `commit-tree`. Pathspecs are resolved by `git diff-tree <base> <tip> -- <pathspec>`, so git's own matcher is used and only changed files can match. `commits` slices reuse a commit when its parent is already in place and otherwise apply it with `git merge-tree --write-tree --merge-base`.

**Rationale**: Nothing touches the working tree or the real index, so a split works with uncommitted changes present, and nothing needs restoring on failure. Building by path cannot conflict. A reordered commit that conflicts is reported and the plan refused; there is no conflict state to resume.

**Alternative**: `git restore --staged --source` on the scratch index. Rejected: it respects sparse-checkout patterns and matches unchanged files silently.

### 4. Re-parent descendants with `commit-tree` instead of the restack engine

**Decision**: Each descendant commit is re-created with the same tree and its parent mapped to the new chain, and the descendant refs join the same ref transaction.

**Rationale**: Because the top slice has the same content as the old tip, a descendant commit's diff against its new parent is unchanged. This needs no checkout, cannot conflict, works for a descendant checked out in another worktree, and keeps the whole operation in one transaction. Going through the restack engine would check out every descendant and could stop on a conflict after the split was already applied.

A descendant that does not contain its parent's old tip was already waiting for a restack. It is left untouched and reported, and its recorded base stays valid for a later `st restack`.

### 5. One ref transaction, then the graph

**Decision**: All branch creations, updates and deletions go through one `git update-ref --stdin` call with expected old values. The graph is saved afterwards; if that fails, the inverse transaction runs.

**Rationale**: `update-ref --stdin` applies all updates or none. The old values also catch a branch that moved while slices were being built, which matters when another agent shares the repository.

### 6. Move refs under a checkout when content is identical

**Decision**: HEAD is left alone when the original branch is checked out and the top slice keeps its name. It is detached and re-attached to the top slice only when the name goes to a lower slice or is dropped. A branch checked out in another worktree can only be split if the top slice keeps its name.

**Rationale**: Moving a ref to a commit with the same tree leaves index and files consistent. Any other move under a checkout would desynchronize it.

### 7. Dropping the original name is explicit

**Decision**: A plan in which no slice is named after the original branch is refused unless it sets `"drop_original": true`.

**Rationale**: The original name carries a pull request and child branches. Losing it by forgetting to reuse the name is easy for an agent and costly to notice.

### 8. A path claims only files that are still open

**Decision**: A pathspec that matches only files an earlier slice already brought to final content is an error naming that slice.

**Rationale**: It means the author believes a file is in a slice where it is not, typically because an earlier directory path swept it up. Accepting it silently would produce a split that differs from the one intended.

### 9. Overrides are whole-file intermediate content

**Decision**: A slice may map a changed path to a file holding the content it should have at that slice. A later slice lists the path to bring it to final.

**Rationale**: An agent edits files well and crafts patches badly. Whole-file content needs no hunk selection and no `git add -p`. Only regular files can be overridden.

### 10. Verification runs before anything moves, in place

**Decision**: `--exec` checks out each slice (detached) in the current working tree, runs the command, and restores the starting checkout. It requires no uncommitted tracked changes, stashes any tracked changes the command makes and then fails, and restores the checkout on SIGINT and SIGTERM.

**Rationale**: Running in place reuses installed dependencies and incremental build state, which a temporary worktree would lack. Running before the ref transaction keeps the atomic guarantee.

### 11. `--exec` is not reachable over MCP

**Decision**: `st_split_apply` has no exec argument, and `st_run` rejects commands containing `--exec`.

**Rationale**: A user who approves "split a branch" or "run an st command" for an agent has not approved arbitrary shell execution.

### 12. Keep the original commits on a manual backup

**Decision**: When a split rewrites the branch's tip, the original is copied to `backup/manual/<timestamp>/<branch>`. A split that only cuts at existing commits creates no backup.

**Rationale**: Automatic backups are deleted by the next successful restack. Manual backups persist until removed, and `st restore` does not try to consume them.

## Risks / Trade-offs

- **Path slices squash history.** A `paths` or `rest` slice is one new commit; the original commits survive only on the backup branch. Acceptable for review-oriented stacks; `commits` slices keep history where it is worth keeping.
- **`--exec` moves the working tree.** A shared worktree sees files change during verification. It is opt-in and refuses to start with uncommitted tracked changes.
- **Dry-run commit ids are previews.** Applying builds the commits again, with a new committer time and therefore new ids.
- **The MCP server acts on its own worktree.** `st_split_apply` requires `branch` in the plan so it never assumes the server's current branch.
