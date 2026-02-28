package restack

import (
	"testing"

	"github.com/cpave3/staccato/pkg/graph"
)

func TestTopologicalSort_SortsBranchesInDependencyOrder(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")
	g.AddBranch("feature-c", "feature-b", "ghi789", "jkl012")

	sorted, err := TopologicalSort(g, "main")
	if err != nil {
		t.Fatalf("failed to sort: %v", err)
	}

	if len(sorted) != 3 {
		t.Errorf("expected 3 branches, got: %d", len(sorted))
	}

	// Check order: feature-a -> feature-b -> feature-c (main is root, not in graph)
	expectedOrder := []string{"feature-a", "feature-b", "feature-c"}
	for i, expected := range expectedOrder {
		if sorted[i] != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, sorted[i])
		}
	}
}

func TestTopologicalSort_HandlesMultipleChildren(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "main", "abc123", "ghi789")
	g.AddBranch("feature-c", "feature-a", "def456", "jkl012")
	g.AddBranch("feature-d", "feature-b", "ghi789", "mno345")

	sorted, err := TopologicalSort(g, "main")
	if err != nil {
		t.Fatalf("failed to sort: %v", err)
	}

	if len(sorted) != 4 {
		t.Errorf("expected 4 branches, got: %d", len(sorted))
	}

	// Check that dependencies come before dependents
	featureAIdx := indexOf(sorted, "feature-a")
	featureBIdx := indexOf(sorted, "feature-b")
	featureCIdx := indexOf(sorted, "feature-c")
	featureDIdx := indexOf(sorted, "feature-d")
	if featureAIdx > featureCIdx {
		t.Error("feature-a should come before feature-c")
	}
	if featureBIdx > featureDIdx {
		t.Error("feature-b should come before feature-d")
	}
}

func TestTopologicalSort_DetectsCycle(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")

	// Manually create a cycle (this wouldn't normally happen through the API)
	// feature-c depends on feature-b
	g.AddBranch("feature-c", "feature-b", "ghi789", "jkl012")
	// Now change feature-a to depend on feature-c, creating a cycle
	g.Branches["feature-a"].Parent = "feature-c"

	_, err := TopologicalSort(g, "main")
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestGetStackBranches_ReturnsAllBranchesInStack(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")
	g.AddBranch("feature-c", "feature-b", "ghi789", "jkl012")
	// This branch is not in the main stack
	g.AddBranch("other-feature", "main", "abc123", "nop678")

	branches := GetStackBranches(g, "feature-a")

	if len(branches) != 3 {
		t.Errorf("expected 3 branches in stack, got: %d", len(branches))
	}

	expected := map[string]bool{
		"feature-a": true,
		"feature-b": true,
		"feature-c": true,
	}

	for _, branch := range branches {
		if !expected[branch] {
			t.Errorf("unexpected branch in stack: %s", branch)
		}
	}
}

func TestGetDownstreamBranches_ReturnsBranchesAfterSpecified(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")
	g.AddBranch("feature-c", "feature-b", "ghi789", "jkl012")

	branches := GetDownstreamBranches(g, "feature-a")

	if len(branches) != 2 {
		t.Errorf("expected 2 downstream branches, got: %d", len(branches))
	}

	// Should include feature-b and feature-c, but not feature-a
	for _, branch := range branches {
		if branch == "feature-a" {
			t.Error("should not include the starting branch")
		}
	}
}

func TestEngine_IdentifiesConflictingBranches(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")

	// Simulate a conflict scenario by checking if branch exists in graph
	engine := NewEngine(nil, nil) // git and graph will be nil for this test

	exists := engine.BranchInGraph(g, "feature-a")
	if !exists {
		t.Error("expected feature-a to be in graph")
	}

	exists = engine.BranchInGraph(g, "nonexistent")
	if exists {
		t.Error("expected nonexistent to not be in graph")
	}
}

// ---------------------------------------------------------------------------
// TestGetLineage
// ---------------------------------------------------------------------------

func TestGetLineage(t *testing.T) {
	t.Run("linear_chain_from_middle", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")
		g.AddBranch("b", "a", "def", "ghi")
		g.AddBranch("c", "b", "ghi", "jkl")

		lineage := GetLineage(g, "b")
		expected := []string{"main", "a", "b", "c"}
		if len(lineage) != len(expected) {
			t.Fatalf("expected %d branches, got %d: %v", len(expected), len(lineage), lineage)
		}
		for i, name := range expected {
			if lineage[i] != name {
				t.Errorf("position %d: expected %s, got %s", i, name, lineage[i])
			}
		}
	})

	t.Run("from_root", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")
		g.AddBranch("b", "a", "def", "ghi")
		g.AddBranch("c", "b", "ghi", "jkl")

		lineage := GetLineage(g, "main")
		expected := []string{"main", "a", "b", "c"}
		if len(lineage) != len(expected) {
			t.Fatalf("expected %d branches, got %d: %v", len(expected), len(lineage), lineage)
		}
		for i, name := range expected {
			if lineage[i] != name {
				t.Errorf("position %d: expected %s, got %s", i, name, lineage[i])
			}
		}
	})

	t.Run("from_leaf", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")
		g.AddBranch("b", "a", "def", "ghi")
		g.AddBranch("c", "b", "ghi", "jkl")

		lineage := GetLineage(g, "c")
		expected := []string{"main", "a", "b", "c"}
		if len(lineage) != len(expected) {
			t.Fatalf("expected %d branches, got %d: %v", len(expected), len(lineage), lineage)
		}
		for i, name := range expected {
			if lineage[i] != name {
				t.Errorf("position %d: expected %s, got %s", i, name, lineage[i])
			}
		}
	})

	t.Run("forked_excludes_other_lineage", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")
		g.AddBranch("b", "a", "def", "ghi")
		g.AddBranch("d", "main", "abc", "mno")

		lineage := GetLineage(g, "a")
		expected := []string{"main", "a", "b"}
		if len(lineage) != len(expected) {
			t.Fatalf("expected %d branches, got %d: %v", len(expected), len(lineage), lineage)
		}
		for i, name := range expected {
			if lineage[i] != name {
				t.Errorf("position %d: expected %s, got %s", i, name, lineage[i])
			}
		}
		// d should not be in the lineage
		for _, name := range lineage {
			if name == "d" {
				t.Error("lineage of 'a' should not include 'd'")
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetAncestors
// ---------------------------------------------------------------------------

func TestGetAncestors(t *testing.T) {
	t.Run("linear_chain", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")
		g.AddBranch("b", "a", "def", "ghi")
		g.AddBranch("c", "b", "ghi", "jkl")

		ancestors := GetAncestors(g, "c")
		expected := []string{"main", "a", "b", "c"}
		if len(ancestors) != len(expected) {
			t.Fatalf("expected %d ancestors, got %d: %v", len(expected), len(ancestors), ancestors)
		}
		for i, name := range expected {
			if ancestors[i] != name {
				t.Errorf("position %d: expected %s, got %s", i, name, ancestors[i])
			}
		}
	})

	t.Run("root_returns_self", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")

		ancestors := GetAncestors(g, "main")
		if len(ancestors) != 1 || ancestors[0] != "main" {
			t.Errorf("expected [main], got %v", ancestors)
		}
	})

	t.Run("direct_child", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("a", "main", "abc", "def")

		ancestors := GetAncestors(g, "a")
		expected := []string{"main", "a"}
		if len(ancestors) != len(expected) {
			t.Fatalf("expected %d ancestors, got %d: %v", len(expected), len(ancestors), ancestors)
		}
		for i, name := range expected {
			if ancestors[i] != name {
				t.Errorf("position %d: expected %s, got %s", i, name, ancestors[i])
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestIsBranchAtTip
// ---------------------------------------------------------------------------

func TestIsBranchAtTip(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("a", "main", "abc", "def")
	g.AddBranch("b", "a", "def", "ghi")

	t.Run("tip_no_children", func(t *testing.T) {
		if !IsBranchAtTip(g, "b") {
			t.Error("b should be at tip (no children)")
		}
	})

	t.Run("non_tip_has_children", func(t *testing.T) {
		if IsBranchAtTip(g, "a") {
			t.Error("a should NOT be at tip (has child b)")
		}
	})

	t.Run("root_with_children", func(t *testing.T) {
		if IsBranchAtTip(g, "main") {
			t.Error("main should NOT be at tip (has child a)")
		}
	})
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
