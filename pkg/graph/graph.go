package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	CurrentVersion   = 1
	DefaultGraphPath = ".git/stack/graph.json"
	SharedGraphRef   = "refs/staccato/graph"
)

// Branch represents a branch in the stack with its metadata
type Branch struct {
	Name    string `json:"name"`
	Parent  string `json:"parent"`
	BaseSHA string `json:"base_sha"`
	HeadSHA string `json:"head_sha"`
}

// Graph represents the entire stack structure
type Graph struct {
	Version  int                `json:"version"`
	Root     string             `json:"root"`
	Branches map[string]*Branch `json:"branches"`
}

// NewGraph creates a new stack graph with the given root branch
func NewGraph(root string) *Graph {
	return &Graph{
		Version:  CurrentVersion,
		Root:     root,
		Branches: make(map[string]*Branch),
	}
}

// AddBranch adds a new branch to the graph
func (g *Graph) AddBranch(name, parent, baseSHA, headSHA string) {
	g.Branches[name] = &Branch{
		Name:    name,
		Parent:  parent,
		BaseSHA: baseSHA,
		HeadSHA: headSHA,
	}
}

// GetBranch retrieves a branch by name
func (g *Graph) GetBranch(name string) (*Branch, bool) {
	branch, exists := g.Branches[name]
	return branch, exists
}

// UpdateBranch updates the base and head SHAs of a branch
func (g *Graph) UpdateBranch(name, baseSHA, headSHA string) {
	if branch, exists := g.Branches[name]; exists {
		branch.BaseSHA = baseSHA
		branch.HeadSHA = headSHA
	}
}

// RemoveBranch removes a branch from the graph
func (g *Graph) RemoveBranch(name string) {
	delete(g.Branches, name)
}

// GetChildren returns all branches that have the given branch as their parent
func (g *Graph) GetChildren(parentName string) []*Branch {
	var children []*Branch
	for _, branch := range g.Branches {
		if branch.Parent == parentName {
			children = append(children, branch)
		}
	}
	return children
}

// ReparentChildren sets the parent of all children of branchName to newParent
func (g *Graph) ReparentChildren(branchName, newParent string) {
	for _, branch := range g.Branches {
		if branch.Parent == branchName {
			branch.Parent = newParent
		}
	}
}

// ValidateNoCycle checks if adding a branch would create a cycle
func (g *Graph) ValidateNoCycle(branchName, parentName string) error {
	// Check if parent would create a cycle
	current := parentName
	for current != "" {
		if current == branchName {
			return fmt.Errorf("adding branch %s with parent %s would create a cycle", branchName, parentName)
		}
		if branch, exists := g.Branches[current]; exists {
			current = branch.Parent
		} else {
			break
		}
	}
	return nil
}

// Save persists the graph to disk
func (g *Graph) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write graph file: %w", err)
	}

	return nil
}

// LoadGraph reads a graph from disk
func LoadGraph(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read graph file: %w", err)
	}

	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("failed to unmarshal graph: %w", err)
	}

	// Initialize map if nil (for backward compatibility)
	if g.Branches == nil {
		g.Branches = make(map[string]*Branch)
	}

	return &g, nil
}
