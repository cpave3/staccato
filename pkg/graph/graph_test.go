package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraph_CanAddBranch(t *testing.T) {
	g := NewGraph("main")

	g.AddBranch("feature-a", "main", "abc123", "def456")

	branch, exists := g.GetBranch("feature-a")
	if !exists {
		t.Fatal("expected branch to exist")
	}
	if branch.Name != "feature-a" {
		t.Errorf("expected name feature-a, got %s", branch.Name)
	}
	if branch.Parent != "main" {
		t.Errorf("expected parent main, got %s", branch.Parent)
	}
	if branch.BaseSHA != "abc123" {
		t.Errorf("expected base_sha abc123, got %s", branch.BaseSHA)
	}
	if branch.HeadSHA != "def456" {
		t.Errorf("expected head_sha def456, got %s", branch.HeadSHA)
	}
}

func TestGraph_CanSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	graphPath := filepath.Join(tmpDir, "graph.json")

	g := NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")

	err := g.Save(graphPath)
	if err != nil {
		t.Fatalf("failed to save graph: %v", err)
	}

	loaded, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("failed to load graph: %v", err)
	}

	if loaded.Root != "main" {
		t.Errorf("expected root main, got %s", loaded.Root)
	}

	if len(loaded.Branches) != 2 {
		t.Errorf("expected 2 branches, got %d", len(loaded.Branches))
	}

	branchA, exists := loaded.GetBranch("feature-a")
	if !exists {
		t.Fatal("expected feature-a to exist")
	}
	if branchA.Parent != "main" {
		t.Errorf("expected parent main, got %s", branchA.Parent)
	}
}

func TestGraph_CanUpdateBranch(t *testing.T) {
	g := NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")

	g.UpdateBranch("feature-a", "def456", "xyz789")

	branch, _ := g.GetBranch("feature-a")
	if branch.BaseSHA != "def456" {
		t.Errorf("expected base_sha def456, got %s", branch.BaseSHA)
	}
	if branch.HeadSHA != "xyz789" {
		t.Errorf("expected head_sha xyz789, got %s", branch.HeadSHA)
	}
}

func TestGraph_CanRemoveBranch(t *testing.T) {
	g := NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")

	g.RemoveBranch("feature-a")

	_, exists := g.GetBranch("feature-a")
	if exists {
		t.Error("expected branch to be removed")
	}
}

func TestGraph_CanGetChildren(t *testing.T) {
	g := NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")
	g.AddBranch("feature-c", "feature-a", "def456", "jkl012")

	children := g.GetChildren("feature-a")
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}

	childMap := make(map[string]bool)
	for _, child := range children {
		childMap[child.Name] = true
	}
	if !childMap["feature-b"] || !childMap["feature-c"] {
		t.Error("expected feature-b and feature-c as children")
	}
}

func TestGraph_CanDetectCycle(t *testing.T) {
	g := NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")

	// Creating a cycle: main -> feature-a -> feature-b -> feature-a
	// Try to set feature-a's parent to feature-b (which has parent feature-a)
	err := g.ValidateNoCycle("feature-a", "feature-b")
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestGraph_ReturnsErrorForNonExistentFile(t *testing.T) {
	_, err := LoadGraph("/nonexistent/path/graph.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestGraph_ReparentChildren(t *testing.T) {
	g := NewGraph("main")
	g.AddBranch("m1", "main", "a", "b")
	g.AddBranch("m2", "m1", "b", "c")
	g.AddBranch("m3", "m1", "b", "d")
	g.AddBranch("other", "main", "a", "e")

	g.ReparentChildren("m1", "main")

	// m2 and m3 should now have parent "main"
	if b, _ := g.GetBranch("m2"); b.Parent != "main" {
		t.Errorf("m2 parent = %q, want main", b.Parent)
	}
	if b, _ := g.GetBranch("m3"); b.Parent != "main" {
		t.Errorf("m3 parent = %q, want main", b.Parent)
	}
	// "other" should be unchanged
	if b, _ := g.GetBranch("other"); b.Parent != "main" {
		t.Errorf("other parent = %q, want main", b.Parent)
	}
}

func TestUserGraphRef(t *testing.T) {
	ref := UserGraphRef("alice@example.com")

	// Should be under refs/staccato/graphs/
	if ref == SharedGraphRefLegacy {
		t.Error("per-user ref should differ from legacy ref")
	}
	if ref[:len("refs/staccato/graphs/")] != "refs/staccato/graphs/" {
		t.Errorf("ref should start with refs/staccato/graphs/, got %s", ref)
	}

	// Same email produces same ref
	if UserGraphRef("alice@example.com") != ref {
		t.Error("same email should produce same ref")
	}

	// Different email produces different ref
	if UserGraphRef("bob@example.com") == ref {
		t.Error("different email should produce different ref")
	}
}

func TestGraph_CreatesDirectoryOnSave(t *testing.T) {
	tmpDir := t.TempDir()
	graphPath := filepath.Join(tmpDir, "nested", "deep", "graph.json")

	g := NewGraph("main")
	err := g.Save(graphPath)
	if err != nil {
		t.Fatalf("failed to save graph: %v", err)
	}

	_, err = os.Stat(graphPath)
	if os.IsNotExist(err) {
		t.Error("expected file to be created in nested directory")
	}
}
