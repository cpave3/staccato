package restack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

// Result represents the outcome of a restack operation
type Result struct {
	Success     bool
	Conflicts   bool
	ConflictsAt string
	Completed   []string
	Backups     map[string]string
	Error       error
}

// RestackState persists lineage information across restack/continue invocations.
// Saved to .git/stack/restack-state.json when a restack hits a conflict.
type RestackState struct {
	Lineage []string `json:"lineage"`
}

// SaveRestackState writes the restack state to disk.
func SaveRestackState(repoPath string, state *RestackState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal restack state: %w", err)
	}
	stateDir := filepath.Join(repoPath, ".git", "stack")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	return os.WriteFile(filepath.Join(stateDir, "restack-state.json"), data, 0644)
}

// LoadRestackState reads the restack state from disk.
func LoadRestackState(repoPath string) (*RestackState, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, ".git", "stack", "restack-state.json"))
	if err != nil {
		return nil, err
	}
	var state RestackState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal restack state: %w", err)
	}
	return &state, nil
}

// ClearRestackState removes the restack state file.
func ClearRestackState(repoPath string) {
	os.Remove(filepath.Join(repoPath, ".git", "stack", "restack-state.json"))
}

// Engine handles the restack logic
type Engine struct {
	git    *git.Runner
	backup *backup.Manager
}

// NewEngine creates a new restack engine
func NewEngine(git *git.Runner, backupMgr *backup.Manager) *Engine {
	return &Engine{
		git:    git,
		backup: backupMgr,
	}
}

// TopologicalSort returns branches in topological order (parents before children)
func TopologicalSort(g *graph.Graph, root string) ([]string, error) {
	visited := make(map[string]bool)
	result := []string{}

	var visit func(string, map[string]bool) error
	visit = func(branch string, visiting map[string]bool) error {
		if visited[branch] {
			return nil
		}

		// Check for cycle using visiting set
		if visiting[branch] {
			return fmt.Errorf("cycle detected involving branch: %s", branch)
		}

		visiting[branch] = true

		// First, visit parent (unless this is the root)
		if branch != g.Root {
			b, exists := g.GetBranch(branch)
			if !exists {
				delete(visiting, branch)
				return fmt.Errorf("branch %s not found in graph", branch)
			}

			// Recursively visit parent
			if err := visit(b.Parent, visiting); err != nil {
				delete(visiting, branch)
				return err
			}
		}

		visited[branch] = true
		delete(visiting, branch)

		// Only add to result if it's a tracked branch (not the root)
		if branch != g.Root {
			result = append(result, branch)
		}
		return nil
	}

	// Visit all branches
	for name := range g.Branches {
		visiting := make(map[string]bool)
		if err := visit(name, visiting); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// GetStackBranches returns all branches that are descendants of the given branch
func GetStackBranches(g *graph.Graph, startBranch string) []string {
	result := []string{startBranch}
	visited := map[string]bool{startBranch: true}

	var collectChildren func(string)
	collectChildren = func(parent string) {
		children := g.GetChildren(parent)
		for _, child := range children {
			if !visited[child.Name] {
				visited[child.Name] = true
				result = append(result, child.Name)
				collectChildren(child.Name)
			}
		}
	}

	collectChildren(startBranch)
	return result
}

// GetDownstreamBranches returns all branches downstream from the given branch
func GetDownstreamBranches(g *graph.Graph, startBranch string) []string {
	return GetStackBranches(g, startBranch)[1:] // Exclude start branch
}

// GetLineage returns all branches in the lineage from root to the given branch and its descendants
func GetLineage(g *graph.Graph, branch string) []string {
	result := []string{}

	// Get ancestors from root to this branch
	ancestors := []string{}
	current := branch
	for current != "" && current != g.Root {
		ancestors = append([]string{current}, ancestors...)
		if b, exists := g.GetBranch(current); exists {
			current = b.Parent
		} else {
			break
		}
	}
	// Add root if it's not already the branch we're looking for
	if branch != g.Root {
		ancestors = append([]string{g.Root}, ancestors...)
	} else {
		// The branch itself is the root
		ancestors = []string{g.Root}
	}

	result = append(result, ancestors...)

	// Get all descendants of this branch
	descendants := GetStackBranches(g, branch)
	// Skip the first one since it's the branch itself (already included)
	if len(descendants) > 1 {
		result = append(result, descendants[1:]...)
	}

	return result
}

// GetAncestors returns the ancestor chain from root to the given branch, excluding descendants.
func GetAncestors(g *graph.Graph, branch string) []string {
	ancestors := []string{}
	current := branch
	for current != "" && current != g.Root {
		ancestors = append([]string{current}, ancestors...)
		if b, exists := g.GetBranch(current); exists {
			current = b.Parent
		} else {
			break
		}
	}
	if branch != g.Root {
		ancestors = append([]string{g.Root}, ancestors...)
	} else {
		ancestors = []string{g.Root}
	}
	return ancestors
}

// IsBranchAtTip checks if a branch has no children (is at the tip of its lineage)
func IsBranchAtTip(g *graph.Graph, branch string) bool {
	children := g.GetChildren(branch)
	return len(children) == 0
}

// restackBranches is the core restack logic that rebases a set of branches
// in topological order. It does NOT create backups — callers handle that.
func (e *Engine) restackBranches(g *graph.Graph, branches []string) (*Result, error) {
	result := &Result{
		Success:   false,
		Completed: []string{},
		Backups:   make(map[string]string),
	}

	// Enable rerere for conflict resolution
	if e.git != nil {
		if err := e.git.EnableRerere(); err != nil {
			// Non-fatal: continue even if rerere can't be enabled
			fmt.Printf("Warning: could not enable rerere: %v\n", err)
		}
	}

	// Get topological order
	sorted, err := TopologicalSort(g, g.Root)
	if err != nil {
		result.Error = fmt.Errorf("failed to sort branches: %w", err)
		return result, result.Error
	}

	// Filter to only branches in our set
	var branchesToRestack []string
	for _, branch := range sorted {
		if contains(branches, branch) {
			branchesToRestack = append(branchesToRestack, branch)
		}
	}

	// Rebase each branch onto its parent
	for _, branch := range branchesToRestack {
		b, exists := g.GetBranch(branch)
		if !exists {
			result.Error = fmt.Errorf("branch %s not found in graph", branch)
			return result, result.Error
		}

		// Checkout the branch
		if err := e.git.CheckoutBranch(branch); err != nil {
			result.Error = fmt.Errorf("failed to checkout %s: %w", branch, err)
			return result, result.Error
		}

		// Use --onto with BaseSHA for correct stacked rebasing.
		// This replays only the branch's own commits (BaseSHA..HEAD) onto the parent,
		// avoiding conflicts from replaying ancestor commits.
		var rebaseErr error
		if b.BaseSHA != "" {
			rebaseErr = e.git.RebaseOnto(b.Parent, b.BaseSHA)
		} else {
			rebaseErr = e.git.Rebase(b.Parent)
		}

		if rebaseErr != nil {
			// Check if there's a conflict
			inProgress, _ := e.git.IsRebaseInProgress()
			if inProgress {
				result.Conflicts = true
				result.ConflictsAt = branch
				result.Error = fmt.Errorf("conflict while rebasing %s onto %s", branch, b.Parent)
				return result, result.Error
			}
			result.Error = fmt.Errorf("failed to rebase %s: %w", branch, rebaseErr)
			return result, result.Error
		}

		// Update branch metadata with new SHAs
		newBaseSHA, _ := e.git.GetCommitSHA(b.Parent)
		newHeadSHA, _ := e.git.GetCommitSHA(branch)
		g.UpdateBranch(branch, newBaseSHA, newHeadSHA)

		result.Completed = append(result.Completed, branch)
	}

	result.Success = true
	return result, nil
}

// Restack performs a restack operation starting from the root branch
func (e *Engine) Restack(g *graph.Graph, startBranch string) (*Result, error) {
	// Get all branches in the stack
	stackBranches := GetStackBranches(g, startBranch)

	// Create backups before any destructive operations
	var backups map[string]string
	if e.backup != nil {
		var err error
		backups, err = e.backup.CreateBackupsForStack(stackBranches)
		if err != nil {
			return &Result{
				Error:   fmt.Errorf("failed to create backups: %w", err),
				Backups: backups,
			}, err
		}
	}

	result, err := e.restackBranches(g, stackBranches)
	if backups != nil {
		result.Backups = backups
	}
	return result, err
}

// RestackLineage performs a restack operation for a specific set of branches (a lineage)
func (e *Engine) RestackLineage(g *graph.Graph, startBranch string, lineageBranches []string) (*Result, error) {
	// Create backups before any destructive operations
	var backups map[string]string
	if e.backup != nil {
		var err error
		backups, err = e.backup.CreateBackupsForStack(lineageBranches)
		if err != nil {
			return &Result{
				Error:   fmt.Errorf("failed to create backups: %w", err),
				Backups: backups,
			}, err
		}
	}

	result, err := e.restackBranches(g, lineageBranches)
	if backups != nil {
		result.Backups = backups
	}
	return result, err
}

// Continue resumes a restack after conflict resolution.
// lineageBranches specifies which branches to continue restacking.
// If nil, falls back to restacking all branches from root.
func (e *Engine) Continue(g *graph.Graph, lineageBranches []string) (*Result, error) {
	result := &Result{
		Success:   false,
		Completed: []string{},
		Backups:   make(map[string]string),
	}

	// Check if rebase is in progress
	inProgress, err := e.git.IsRebaseInProgress()
	if err != nil {
		result.Error = fmt.Errorf("failed to check rebase status: %w", err)
		return result, result.Error
	}

	if !inProgress {
		result.Error = fmt.Errorf("no rebase in progress")
		return result, result.Error
	}

	// Continue the rebase
	if err := e.git.RebaseContinue(); err != nil {
		// Check if still conflicting
		stillInProgress, _ := e.git.IsRebaseInProgress()
		if stillInProgress {
			result.Conflicts = true
			result.Error = fmt.Errorf("still have conflicts to resolve")
			return result, result.Error
		}
		result.Error = fmt.Errorf("failed to continue rebase: %w", err)
		return result, result.Error
	}

	// Get current branch and update metadata
	currentBranch, _ := e.git.GetCurrentBranch()
	b, _ := g.GetBranch(currentBranch)
	if b != nil {
		newBaseSHA, _ := e.git.GetCommitSHA(b.Parent)
		newHeadSHA, _ := e.git.GetCommitSHA(currentBranch)
		g.UpdateBranch(currentBranch, newBaseSHA, newHeadSHA)
	}

	// Continue with remaining branches — no backup creation (preserves originals)
	if lineageBranches != nil {
		return e.restackBranches(g, lineageBranches)
	}

	// Fallback: restack all from root (backwards compatibility)
	stackBranches := GetStackBranches(g, g.Root)
	return e.restackBranches(g, stackBranches)
}

// Abort cancels the current restack and restores from backups
func (e *Engine) Abort(g *graph.Graph, backups map[string]string) error {
	// Abort any in-progress rebase
	inProgress, _ := e.git.IsRebaseInProgress()
	if inProgress {
		if err := e.git.RebaseAbort(); err != nil {
			return fmt.Errorf("failed to abort rebase: %w", err)
		}
	}

	// Restore all branches from backups
	if e.backup != nil {
		return e.backup.RestoreStack(backups)
	}

	return nil
}

// BranchInGraph checks if a branch exists in the graph
func (e *Engine) BranchInGraph(g *graph.Graph, branch string) bool {
	_, exists := g.GetBranch(branch)
	return exists
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
