package context

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

// TrunkBranches are common trunk/root branch names that should be auto-detected as roots.
var TrunkBranches = []string{"main", "master", "develop", "trunk"}

// StaccatoContext holds the shared state needed by both CLI commands and the MCP server.
type StaccatoContext struct {
	Graph    *graph.Graph
	Git      *git.Runner
	RepoPath string
	// sharedRef is the per-user ref path when in shared mode, or "" for local mode.
	sharedRef string
}

// IsTrunkBranch returns true if the branch name is a common trunk/root branch.
func IsTrunkBranch(name string) bool {
	for _, t := range TrunkBranches {
		if name == t {
			return true
		}
	}
	return false
}

// NewContext creates a StaccatoContext with the correct shared ref resolved.
// Use this when you have a graph in memory that needs to be saved.
func NewContext(g *graph.Graph, gitRunner *git.Runner, repoPath string) *StaccatoContext {
	return &StaccatoContext{
		Graph:     g,
		Git:       gitRunner,
		RepoPath:  repoPath,
		sharedRef: resolveSharedRef(gitRunner),
	}
}

// IsShared returns true if the context is using a shared graph ref.
func (sc *StaccatoContext) IsShared() bool {
	return sc.sharedRef != ""
}

// SharedRef returns the per-user shared graph ref path, or "" if local mode.
func (sc *StaccatoContext) SharedRef() string {
	return sc.sharedRef
}

// resolveSharedRef determines the per-user shared ref for this repo.
// Returns the ref path, or "" if no shared ref is active.
// Also handles migration from the legacy single ref.
func resolveSharedRef(gitRunner *git.Runner) string {
	email, err := gitRunner.GetUserEmail()
	if err != nil {
		// No email configured — check legacy ref only
		if gitRunner.RefExists(graph.SharedGraphRefLegacy) {
			return graph.SharedGraphRefLegacy
		}
		return ""
	}

	userRef := graph.UserGraphRef(email)

	// Per-user ref takes priority
	if gitRunner.RefExists(userRef) {
		return userRef
	}

	// Fall back to legacy ref (will be migrated on save)
	if gitRunner.RefExists(graph.SharedGraphRefLegacy) {
		return userRef // return the per-user ref — migration happens on save
	}

	return ""
}

// Load discovers the git root (starting from repoPath, or cwd if empty)
// and loads the stack graph from either the shared ref or local file.
func Load(repoPath string) (*StaccatoContext, error) {
	gitRunner := git.NewRunner(repoPath)
	root, err := gitRunner.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository")
	}

	gitRunner = git.NewRunner(root)

	var g *graph.Graph
	sharedRef := resolveSharedRef(gitRunner)

	switch {
	case sharedRef != "":
		// Try per-user ref first, then legacy
		readRef := sharedRef
		if !gitRunner.RefExists(sharedRef) {
			readRef = graph.SharedGraphRefLegacy
		}
		data, err := gitRunner.ReadBlobRef(readRef)
		if err != nil {
			return nil, fmt.Errorf("failed to read shared graph ref: %w", err)
		}
		g = &graph.Graph{}
		if err := json.Unmarshal(data, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal shared graph: %w", err)
		}
		if g.Branches == nil {
			g.Branches = make(map[string]*graph.Branch)
		}

	default:
		graphPath := filepath.Join(root, graph.DefaultGraphPath)
		g, err = graph.LoadGraph(graphPath)
		if err != nil {
			currentBranch, branchErr := gitRunner.GetCurrentBranch()
			if branchErr != nil {
				return nil, fmt.Errorf("failed to get current branch: %w", branchErr)
			}
			g = graph.NewGraph(currentBranch)
		}
	}

	return &StaccatoContext{
		Graph:     g,
		Git:       gitRunner,
		RepoPath:  root,
		sharedRef: sharedRef,
	}, nil
}

// Save persists the graph to either the shared ref or local file.
func (sc *StaccatoContext) Save() error {
	if sc.sharedRef != "" {
		// Increment version on each shared save for reconciliation ordering.
		sc.Graph.Version++
		data, err := json.MarshalIndent(sc.Graph, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal graph: %w", err)
		}
		if err := sc.Git.WriteBlobRef(sc.sharedRef, data); err != nil {
			return fmt.Errorf("failed to write shared ref: %w", err)
		}
		// Migrate: clean up legacy ref if it still exists
		if sc.sharedRef != graph.SharedGraphRefLegacy && sc.Git.RefExists(graph.SharedGraphRefLegacy) {
			sc.Git.DeleteRef(graph.SharedGraphRefLegacy)
		}
		return nil
	}
	graphPath := filepath.Join(sc.RepoPath, graph.DefaultGraphPath)
	return sc.Graph.Save(graphPath)
}
