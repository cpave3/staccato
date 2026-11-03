---
name: staccato
description: |
  Manage git stacks using Staccato (st) — create stacked branches, restack after changes, sync with remote, and create stacked PRs. Use when working in a repo with stacked branches, when the user mentions stacks/stacking, when you need to split work into incremental PRs, or when you see `.git/stack/` or `refs/staccato/` in a repo. Also use when the user says "st", "staccato", "stack", "restack", or "stacked PRs".
---

# Staccato — Git Stack Management

Staccato (`st`) manages chains of dependent branches ("stacks") so that each pull request stays small. Branches form a tree on the trunk (main/master), and restacking rebases each branch onto its parent. Run it from a shell.

## Start with the help

```
st help                 # primer: concepts, the main workflows, every command
st --help               # every command, with examples
st <command> --help     # usage, arguments, options and examples for one command
```

The help ships with the binary, so it matches the installed version. Read `st help` once, and a command's `--help` before the first time you use it, instead of guessing flags.

## The workflows in brief

```
st new feature/api            # start a stack on the trunk
st append feature/frontend    # stack a branch on the current one
st log --json                 # the stack as a JSON tree; st status --json adds pull requests
st modify --all               # amend the current branch, restack the branches above it
st top && st restack          # after committing to a lower branch: restack from the tip
git add <files> && st continue    # after resolving a restack conflict; st abort gives up
st sync                       # fetch, drop merged branches, restack, push
st pr make --title "..." --description "..."    # pull request into the parent branch
st attach <branch> --parent <parent>            # adopt a branch made outside st
st exec "<build command>"     # which branches of the stack build?
st split --help               # split a branch that is too large to review
st graph export --output stack.json   # after st sync: hand the stack to another clone
st graph import stack.json    # in the other clone: fetch and merge the stack
```

## What to keep in mind

- **Commit with git, move branches with st.** Do not `git rebase` a stacked branch by hand: `st restack`, `st modify`, `st move` and `st insert` rebase the branches above it too and keep st's record right.
- **Commit or stash before a command that restacks.** Those commands back the branches up first, and `st abort` or `st restore` puts them back.
- **`st restack` runs from the tip.** From a branch with children it refuses unless `--to-current` is given, which leaves the branches above untouched.
- **A conflict pauses the restack.** Resolve the files, `git add` them, `st continue`. It can stop again on a later branch.
- **Use the non-interactive forms.** `st switch`, `st backup clean`, and `st attach` without `--parent` or `--auto` open a terminal UI. Use `git checkout <branch>` and `st attach --parent` instead. Pass `--title` and `--description` to `st pr make`, or `gh` will prompt.
- **`st exec` checks branches; it does not change them.** Fix a failing branch by checking it out and committing, then restack.
- **A refused push fails `st sync`.** It exits 1 and names the branch with git's reason; a push held for approval says so (`pending`). Wait or ask, then sync again.
- **Splitting is a plan, not surgery.** `st split describe`, write a JSON plan, `st split apply plan.json --dry-run`, then apply. Never reset, stage or cherry-pick by hand to split a branch.
