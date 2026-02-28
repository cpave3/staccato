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

	switch {
	case gitRunner.RefExists(graph.SharedGraphRef):
		data, err := gitRunner.ReadBlobRef(graph.SharedGraphRef)
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
		Graph:    g,
		Git:      gitRunner,
		RepoPath: root,
	}, nil
}

// Save persists the graph to either the shared ref or local file.
func (sc *StaccatoContext) Save() error {
	if sc.Git.RefExists(graph.SharedGraphRef) {
		data, err := json.MarshalIndent(sc.Graph, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal graph: %w", err)
		}
		return sc.Git.WriteBlobRef(graph.SharedGraphRef, data)
	}
	graphPath := filepath.Join(sc.RepoPath, graph.DefaultGraphPath)
	return sc.Graph.Save(graphPath)
}
