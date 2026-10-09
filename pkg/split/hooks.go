package split

import "github.com/cpave3/staccato/pkg/hooks"

// FireHooks runs the branch lifecycle hooks for an applied split: one
// post-branch-create per new slice branch, and a post-branch-delete when no
// slice kept the original branch's name.
func FireHooks(repoPath string, result *Result) {
	if result == nil || !result.Applied {
		return
	}
	runner := hooks.NewRunner(repoPath)
	for _, s := range result.Slices {
		if s.Name == result.Branch {
			continue
		}
		runner.Fire(hooks.Context{
			Event:    hooks.PostBranchCreate,
			RepoPath: repoPath,
			Branch:   s.Name,
			Data:     map[string]any{"parent": s.Parent, "split_from": result.Branch},
		})
	}
	if result.Removed != "" {
		runner.Fire(hooks.Context{
			Event:    hooks.PostBranchDelete,
			RepoPath: repoPath,
			Branch:   result.Removed,
			Data:     map[string]any{"parent": result.Parent},
		})
	}
}
