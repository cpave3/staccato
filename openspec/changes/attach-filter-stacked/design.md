## Context

The `doAttachRecursively` function builds a candidate list from ALL local git branches (via `gitRunner.GetAllBranches()`), excluding only the branch being attached. During recursive attachment of a chain (e.g., attaching 5 untracked branches one after another), the TUI shows branches that were already placed in the graph during earlier recursion steps. This clutters the list and risks the user selecting an already-stacked branch as a parent, which would create an incorrect graph.

Current flow:
1. User runs `st attach branchE`
2. TUI shows all branches — user picks `branchD` as parent
3. `branchD` isn't tracked → recursive attach for `branchD`
4. TUI again shows all branches including `branchE` (being attached) is excluded, but `branchD` still appears as a candidate for itself — wait, no, the branch being attached is excluded. But branches already placed (none yet at step 4) could appear. The real issue: after `branchD` is attached under `branchC`, the next recursion for `branchC` still shows `branchD` and `branchE` as candidates.

## Goals / Non-Goals

**Goals:**
- Filter branches already tracked in the graph from the TUI candidate list during recursive attachment
- Keep the root branch visible (it's a valid parent choice)
- Maintain current behavior for top-level `st attach` (which intentionally shows tracked branches to support relocation)

**Non-Goals:**
- Changing `--auto` or `--parent` behavior (they don't use the TUI candidate list)
- Filtering in the top-level `attachInteractive` path (tracked branches should remain visible there for relocation)
- Adding any new flags or user-facing options

## Decisions

### Filter only in the recursive path (`stopIfTracked=true`)

The `doAttachRecursively` function already has a `stopIfTracked` parameter that distinguishes top-level attach (`false`) from recursive attach (`true`). We use this same flag to control candidate filtering: when `stopIfTracked` is true, exclude graph-tracked branches from candidates.

**Alternative considered**: Always filter tracked branches. Rejected — top-level attach deliberately shows tracked branches to allow relocation/re-parenting via the TUI.

### Filter using `graph.Branches` map lookup

Check each candidate against `g.Branches` (O(1) lookup). Also exclude the root since it's the graph root and always a valid parent. Simple, no new data structures needed.

**Alternative considered**: Pass an explicit "exclude set" through the recursion. Rejected — the graph already tracks what's been attached, so it's redundant.

## Risks / Trade-offs

- **[Risk] Root branch filtered incorrectly** → Mitigated: explicitly keep root in candidates since it's a valid parent target.
- **[Trade-off] Candidate list may be very short during deep recursion** → Acceptable: fewer choices means less confusion. If no candidates remain, the existing "no existing branches" error handles it.
