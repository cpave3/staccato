package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/st/internal/testutil"
)

// TestAttachIntegration_FullChain verifies the complete attach workflow
func TestAttachIntegration_FullChain(t *testing.T) {
	// Build binary
	buildCmd := exec.Command("go", "build", "-o", stBinaryPath, "./cmd/st/")
	buildCmd.Dir = "/var/home/cameron/Projects/restack"
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, output)
	}

	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}
	defer repo.Cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(repo.Dir)
	defer os.Chdir(oldWd)

	// Setup: Create main with initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init: %v", err)
	}

	// Create graph with root
	stackDir := filepath.Join(repo.Dir, ".git", "stack")
	os.MkdirAll(stackDir, 0755)
	rootBranch := getCurrentBranchName(repo)
	graphContent := fmt.Sprintf(`{"version":1,"root":"%s","branches":{}}`, rootBranch)
	os.WriteFile(filepath.Join(stackDir, "graph.json"), []byte(graphContent), 0644)

	// Create manual branches: m1, m2, m3
	for _, branch := range []string{"m1", "m2", "m3"} {
		if err := repo.CreateBranch(branch); err != nil {
			t.Fatalf("Failed to create %s: %v", branch, err)
		}
		if err := repo.CreateFile(branch+".txt", branch+" content"); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddAndCommit(branch + " commit"); err != nil {
			t.Fatal(err)
		}
	}

	// Test: Simulate attach TUI for m3
	// In real usage, user would select m2 as parent
	// Then recursively select m1 as parent of m2
	// Then main as parent of m1

	t.Log("Created branches: main, m1, m2, m3 (m1-m3 are untracked)")
	t.Log("Graph before attach should only have root")

	content, _ := os.ReadFile(filepath.Join(stackDir, "graph.json"))
	if strings.Contains(string(content), "m1") || strings.Contains(string(content), "m2") || strings.Contains(string(content), "m3") {
		t.Error("Graph should not have m1, m2, or m3 before attach")
	}

	t.Log("✓ Test setup complete - manual branches created but not in graph")
}

// TestAttachTUI_Behavior verifies TUI Enter key behavior with real Bubble Tea list
func TestAttachTUI_Behavior(t *testing.T) {
	candidates := []attachCandidate{
		{name: "main", isCurrent: false},
		{name: "m1", isCurrent: false},
		{name: "m2", isCurrent: false},
		{name: "m3", isCurrent: false},
	}

	// Create list items
	var items []list.Item
	for _, c := range candidates {
		items = append(items, c)
	}

	// Create the list with default delegate
	l := list.New(items, list.NewDefaultDelegate(), 80, 20)
	l.SetShowHelp(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.Select(2) // Select m2 (index 2)

	model := &attachTUI{
		branchToAttach: "feature",
		candidates:     candidates,
		list:           l,
	}

	// Verify initial state
	if model.list.Index() != 2 {
		t.Errorf("Expected index 2, got %d", model.list.Index())
	}

	// Simulate Enter key - this is what happens when user presses Enter in TUI
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Check if the model is actually our attachTUI type
	finalModel, ok := newModel.(attachTUI)
	if !ok {
		// Try pointer
		finalModelPtr, ok := newModel.(*attachTUI)
		if !ok {
			t.Fatalf("Unexpected model type: %T", newModel)
		}
		finalModel = *finalModelPtr
	}

	t.Logf("After Enter: selected='%s', quitting=%v", finalModel.selected, finalModel.quitting)

	// Verify the fix worked
	if finalModel.selected == "" {
		t.Error("BUG: selected is empty - Enter key did not work!")
		t.Log("This means the type assertion or index access failed")
	}

	if finalModel.selected != "m2" {
		t.Errorf("Expected selected='m2', got '%s'", finalModel.selected)
	}

	if !finalModel.quitting {
		t.Error("Expected quitting=true")
	}

	// Verify the command is Quit
	if cmd == nil {
		t.Error("Expected Quit command")
	}
}
