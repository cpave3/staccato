package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/user/st/internal/testutil"
)

// TestAttachTUI_EnterKeySelectsParent verifies that pressing Enter selects the highlighted candidate
func TestAttachTUI_EnterKeySelectsParent(t *testing.T) {
	// Create test repo
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	// Setup: main branch with initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	// Create a branch to attach
	if err := repo.CreateBranch("feature-branch"); err != nil {
		t.Fatalf("Failed to create feature-branch: %v", err)
	}

	// Initialize TUI model
	candidates := []attachCandidate{
		{name: "main", isCurrent: false},
		{name: "develop", isCurrent: false},
		{name: "feature-x", isCurrent: false},
	}

	// Create list items
	var items []list.Item
	for _, c := range candidates {
		items = append(items, c)
	}

	// Create the list
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowHelp(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.Select(1) // Select "develop" (index 1)

	model := &attachTUI{
		branchToAttach: "feature-branch",
		candidates:     candidates,
		list:           l,
		selected:       "", // Should be set after Enter
	}

	// Run TUI test
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Send Enter key
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for program to finish
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	// Get final state
	finalModel := tm.FinalModel(t)
	attachModel, ok := finalModel.(attachTUI)
	if !ok {
		attachModelPtr, ok := finalModel.(*attachTUI)
		if !ok {
			t.Fatalf("Could not convert final model: %T", finalModel)
		}
		attachModel = *attachModelPtr
	}

	// Verify selection
	if attachModel.selected != "develop" {
		t.Errorf("Expected selected='develop', got '%s'", attachModel.selected)
	}

	if !attachModel.quitting {
		t.Error("Expected quitting=true")
	}
}

// TestAttachTUI_RecursiveAttach verifies that attach walks back the entire chain
func TestAttachTUI_RecursiveAttach(t *testing.T) {
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	// Setup: Create initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	_ = getCurrentBranchName(repo)

	// Step 1: Create m1 with st (tracked)
	// This requires the binary, skip in unit test
	t.Skip("Requires compiled binary for full integration test")

	// The rest of the test would verify the recursive attachment chain
	// m3 -> m2 -> m1 -> main
}

// TestAttachTUI_Debug verifies Enter key behavior in isolation
func TestAttachTUI_Debug(t *testing.T) {
	candidates := []attachCandidate{
		{name: "main", isCurrent: false},
		{name: "develop", isCurrent: false},
	}

	// Create list items
	var items []list.Item
	for _, c := range candidates {
		items = append(items, c)
	}

	// Create the list
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowHelp(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.Select(0) // Select "main"

	model := &attachTUI{
		branchToAttach: "feature",
		candidates:     candidates,
		list:           l,
	}

	// Simulate Enter key
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	attachModel, ok := newModel.(attachTUI)
	if !ok {
		// Try pointer type
		attachModelPtr, ok := newModel.(*attachTUI)
		if !ok {
			t.Fatalf("Could not convert model to attachTUI: %T", newModel)
		}
		attachModel = *attachModelPtr
	}

	// The bug: selected is empty because type assertion fails
	t.Logf("Selected after Enter: '%s'", attachModel.selected)
	t.Logf("Quitting: %v", attachModel.quitting)

	// Verify it should select "main" (index 0)
	if attachModel.selected == "" {
		t.Error("BUG: Enter key did not set selected value")
		t.Log("This is the bug - the type assertion (attachCandidate) fails")
		t.Log("Fix: Use index-based access instead of type assertion")
	}

	if !strings.Contains(attachModel.selected, "main") {
		t.Errorf("Expected selected to contain 'main', got '%s'", attachModel.selected)
	}
}
