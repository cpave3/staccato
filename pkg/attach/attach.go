package attach

import (
	"fmt"
	"sort"

	"github.com/user/st/pkg/git"
	"github.com/user/st/pkg/graph"
	"github.com/user/st/pkg/output"
)

// Attacher handles lazy attachment of unknown branches
type Attacher struct {
	git     *git.Runner
	printer *output.Printer
}

// NewAttacher creates a new attacher
func NewAttacher(git *git.Runner, printer *output.Printer) *Attacher {
	return &Attacher{
		git:     git,
		printer: printer,
	}
}

// IsBranchInGraph checks if a branch is already tracked in the graph
func (a *Attacher) IsBranchInGraph(g *graph.Graph, branch string) bool {
	if branch == g.Root {
		return true
	}
	_, exists := g.GetBranch(branch)
	return exists
}

// SuggestParents suggests possible parent branches based on merge-base
func (a *Attacher) SuggestParents(g *graph.Graph, branch string) ([]string, error) {
	if a.git == nil {
		return nil, fmt.Errorf("git runner not available")
	}

	// Get all known branches in the graph
	var candidates []string
	for name := range g.Branches {
		candidates = append(candidates, name)
	}
	if g.Root != "" {
		candidates = append(candidates, g.Root)
	}

	// Score candidates by how recent their merge-base is
	type scoredBranch struct {
		name  string
		score int
	}

	var scored []scoredBranch

	for _, candidate := range candidates {
		// Check if merge-base exists (branches have common history)
		_, err := a.git.GetMergeBase(branch, candidate)
		if err != nil {
			// No common history, skip
			continue
		}

		// Calculate score based on recency
		// For now, use a simple heuristic: branches with closer merge-base
		// This could be improved by comparing commit counts
		scored = append(scored, scoredBranch{name: candidate, score: 1})
	}

	// Sort by score (higher is better)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract names
	result := make([]string, 0, len(scored))
	for _, s := range scored {
		result = append(result, s.name)
	}

	return result, nil
}

// AttachBranch adds a branch to the graph with the specified parent
func (a *Attacher) AttachBranch(g *graph.Graph, branch, parent string) error {
	if a.git == nil {
		return fmt.Errorf("git runner not available")
	}

	// Validate that parent exists
	if parent != g.Root {
		if _, exists := g.GetBranch(parent); !exists {
			return fmt.Errorf("parent branch '%s' not found in graph", parent)
		}
	}

	// Validate that branch exists in git
	exists, err := a.git.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("branch '%s' does not exist in git", branch)
	}

	// Get current SHAs
	baseSHA, err := a.git.GetMergeBase(branch, parent)
	if err != nil {
		return fmt.Errorf("failed to get merge-base: %w", err)
	}

	headSHA, err := a.git.GetBranchSHA(branch)
	if err != nil {
		return fmt.Errorf("failed to get branch SHA: %w", err)
	}

	// Add to graph
	g.AddBranch(branch, parent, baseSHA, headSHA)

	if a.printer != nil {
		a.printer.Success("Attached '%s' as child of '%s'", branch, parent)
	}

	return nil
}

// AutoAttach attempts to automatically attach a branch using best candidate
func (a *Attacher) AutoAttach(g *graph.Graph, branch string, autoSelect bool) error {
	// Check if already in graph
	if a.IsBranchInGraph(g, branch) {
		return nil
	}

	// Suggest parents
	candidates, err := a.SuggestParents(g, branch)
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no suitable parent found for branch '%s'", branch)
	}

	if autoSelect {
		// Auto-select the first (best) candidate
		return a.AttachBranch(g, branch, candidates[0])
	}

	// Interactive mode: prompt user
	if a.printer != nil {
		a.printer.AttachPrompt(branch, candidates)
	}

	// In a real implementation, this would read user input
	// For now, we return an error indicating manual selection needed
	return fmt.Errorf("manual parent selection required for branch '%s'", branch)
}

// RecursivelyAttach attaches a branch and all its ancestors up to the root
func (a *Attacher) RecursivelyAttach(g *graph.Graph, branch string, parent string) error {
	// First, ensure parent is attached
	if !a.IsBranchInGraph(g, parent) && parent != g.Root {
		// Need to find parent's parent first
		// This is a simplified version - in reality, we'd prompt for each level
		return fmt.Errorf("parent branch '%s' not in graph, manual attachment required", parent)
	}

	// Attach the branch
	if err := a.AttachBranch(g, branch, parent); err != nil {
		return err
	}

	return nil
}

// FindRoot finds the root branch for any branch in the graph
func (a *Attacher) FindRoot(g *graph.Graph, branch string) string {
	current := branch
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			// Cycle detected, return what we have
			return current
		}
		visited[current] = true

		if current == g.Root {
			return g.Root
		}

		b, exists := g.GetBranch(current)
		if !exists {
			// Branch not in graph, can't find root
			return ""
		}

		current = b.Parent
	}

	return ""
}

// GetUnattachedBranches returns all git branches not in the graph
func (a *Attacher) GetUnattachedBranches(g *graph.Graph) ([]string, error) {
	if a.git == nil {
		return nil, fmt.Errorf("git runner not available")
	}

	// Get all local branches
	output, err := a.git.Run("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var unattached []string
	for _, line := range splitLines(output) {
		branch := line
		if branch == "" {
			continue
		}

		if !a.IsBranchInGraph(g, branch) {
			unattached = append(unattached, branch)
		}
	}

	return unattached, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
