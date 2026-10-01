package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/graph"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestAttach
// ---------------------------------------------------------------------------

func TestAttach(t *testing.T) {
	t.Run("auto_attaches_untracked_branch", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")

		// Create an untracked branch manually
		repo.Checkout(root)
		repo.CreateBranch("manual-branch")
		repo.CreateFile("manual.txt", "manual")
		repo.AddAndCommit("manual commit")

		runSt(t, "attach", "--auto")

		if !graphContains(t, repo, "manual-branch") {
			t.Error("manual-branch should be in graph after attach --auto")
		}
	})

	t.Run("tui_enter_selects_branch", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
			{name: "feature-1", isCurrent: true},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Press Enter — should select index 0 ("main")
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m := updated.(attachTUI)
		if m.selected != "main" {
			t.Errorf("selected = %q, want main", m.selected)
		}
	})

	t.Run("tui_q_quits_without_selection", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		m := updated.(attachTUI)
		if m.selected != "" {
			t.Errorf("selected = %q, want empty", m.selected)
		}
		if !m.quitting {
			t.Error("quitting should be true")
		}
	})

	t.Run("tui_r_sets_root_flag", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
			{name: "develop", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Press 'r' — should set selected to current item AND setAsRoot = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		m := updated.(attachTUI)
		if m.selected != "main" {
			t.Errorf("selected = %q, want main", m.selected)
		}
		if !m.setAsRoot {
			t.Error("setAsRoot should be true after pressing r")
		}
	})

	t.Run("tui_r_quits", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Press 'r' — should also set quitting = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		m := updated.(attachTUI)
		if !m.quitting {
			t.Error("quitting should be true after pressing r")
		}
	})

	t.Run("relocates_already_attached_branch", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create two branches off root
		runSt(t, "new", "tests")
		repo.CreateFile("tests.txt", "tests content")
		repo.AddAndCommit("tests commit")

		// Go back to root and create m1 off root
		repo.Checkout(root)
		runSt(t, "new", "m1")
		repo.CreateFile("m1.txt", "m1 content")
		repo.AddAndCommit("m1 commit")

		// m1 parent should be root
		g := loadGraph(t, repo)
		if g.Branches["m1"].Parent != root {
			t.Fatalf("m1 parent = %q, want %q", g.Branches["m1"].Parent, root)
		}

		// Relocate m1 under tests
		runSt(t, "attach", "m1", "--parent", "tests")

		// m1 parent should now be tests
		g = loadGraph(t, repo)
		if g.Branches["m1"].Parent != "tests" {
			t.Errorf("after relocate, m1 parent = %q, want tests", g.Branches["m1"].Parent)
		}

		// m1 should have tests.txt (rebased onto tests)
		repo.Checkout("m1")
		if _, err := os.Stat(filepath.Join(repo.Dir, "tests.txt")); os.IsNotExist(err) {
			t.Error("m1 should have tests.txt after rebase onto tests")
		}
	})

	t.Run("relocate_same_parent_is_noop", func(t *testing.T) {
		_, root := setupRepoWithStack(t)

		runSt(t, "new", "m1")

		// Attach m1 to same parent (root) — should succeed without error
		out := runSt(t, "attach", "m1", "--parent", root)
		assertContains(t, out, "already has parent")
	})

	t.Run("solo_root_attach_builds_candidate_list", func(t *testing.T) {
		// When the current branch is the solo root, `st attach` should present
		// a TUI with other branches as candidates — not silently no-op.
		// We test this at the model level: a solo root should produce a
		// non-empty candidate list that excludes itself.
		candidates := []attachCandidate{
			{name: "feature-x", isCurrent: false},
			{name: "experiment", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)

		model := attachTUI{
			list:           l,
			branchToAttach: "main",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Verify candidates don't include the branch being attached
		for _, c := range model.candidates {
			if c.name == model.branchToAttach {
				t.Error("candidate list should not include the branch being attached")
			}
		}
		// Verify the TUI would have something to show
		if len(model.candidates) == 0 {
			t.Error("solo root should have candidates for TUI")
		}
	})

	t.Run("tui_search_filters_candidates", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
			{name: "feature-auth", isCurrent: false},
			{name: "feature-api", isCurrent: false},
			{name: "bugfix-login", isCurrent: false},
			{name: "develop", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Enter search mode
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		m := updated.(attachTUI)
		if !m.searchMode {
			t.Fatal("expected search mode after /")
		}

		// Type "feat" — should filter to only feature branches
		for _, ch := range "feat" {
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
			m = updated.(attachTUI)
		}

		// The visible candidates should only be the matches
		if len(m.candidates) != 2 {
			t.Fatalf("expected 2 filtered candidates, got %d", len(m.candidates))
		}
		if m.candidates[0].name != "feature-auth" || m.candidates[1].name != "feature-api" {
			t.Errorf("filtered candidates = %v, want [feature-auth, feature-api]", m.candidates)
		}

		// Press enter to confirm, select first match
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(attachTUI)
		if m.selected != "feature-auth" {
			t.Errorf("selected = %q, want feature-auth", m.selected)
		}
	})

	t.Run("tui_search_esc_restores_all_candidates", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
			{name: "feature-auth", isCurrent: false},
			{name: "bugfix-login", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Enter search, type "feat", then esc
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		m := updated.(attachTUI)
		for _, ch := range "feat" {
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
			m = updated.(attachTUI)
		}
		// Should be filtered now
		if len(m.candidates) != 1 {
			t.Fatalf("expected 1 filtered candidate, got %d", len(m.candidates))
		}

		// Press esc to cancel search
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		m = updated.(attachTUI)
		// All candidates should be restored
		if len(m.candidates) != 3 {
			t.Fatalf("expected 3 candidates after esc, got %d", len(m.candidates))
		}
	})

	t.Run("tui_search_arrows_navigate_filtered_list", func(t *testing.T) {
		candidates := []attachCandidate{
			{name: "main", isCurrent: false},
			{name: "feature-auth", isCurrent: false},
			{name: "feature-api", isCurrent: false},
			{name: "bugfix-login", isCurrent: false},
		}
		var items []list.Item
		for _, c := range candidates {
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Enter search mode, type "feat"
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		m := updated.(attachTUI)
		for _, ch := range "feat" {
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
			m = updated.(attachTUI)
		}

		// Should have 2 filtered items, cursor at 0 (feature-auth)
		if len(m.candidates) != 2 {
			t.Fatalf("expected 2 filtered candidates, got %d", len(m.candidates))
		}
		if m.list.Index() != 0 {
			t.Fatalf("expected index 0, got %d", m.list.Index())
		}

		// Press down arrow while still in search mode
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(attachTUI)

		// Should still be in search mode, cursor moved to 1 (feature-api)
		if !m.searchMode {
			t.Error("expected to still be in search mode")
		}
		if m.list.Index() != 1 {
			t.Errorf("expected index 1 after down arrow, got %d", m.list.Index())
		}

		// Press enter — should select feature-api
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(attachTUI)
		if m.selected != "feature-api" {
			t.Errorf("selected = %q, want feature-api", m.selected)
		}
	})

	t.Run("tui_viewport_scrolls_large_list", func(t *testing.T) {
		// Create 100 branches — View() should not render all of them
		var candidates []attachCandidate
		var items []list.Item
		for i := 0; i < 100; i++ {
			c := attachCandidate{name: fmt.Sprintf("branch-%03d", i)}
			candidates = append(candidates, c)
			items = append(items, c)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)

		model := attachTUI{
			list:           l,
			branchToAttach: "test",
			allCandidates:  candidates,
			candidates:     candidates,
		}

		// Set terminal size to 20 lines
		updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
		m := updated.(attachTUI)

		view := m.View()
		lines := strings.Split(view, "\n")
		// Should be significantly less than 100 lines (header + viewport + footer)
		if len(lines) > 30 {
			t.Errorf("view has %d lines, expected viewport to limit to ~20", len(lines))
		}
	})

	t.Run("solo_root_with_parent_flag_attaches", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create feature branch off root
		repo.CreateBranch("feature-x")
		repo.CreateFile("fx.txt", "feature x")
		repo.AddAndCommit("feature-x commit")

		// Attach feature-x with --parent <root>. root is the auto-created root.
		// This should work without needing trunk detection since root IS already the graph root.
		runSt(t, "attach", "feature-x", "--parent", root)

		if !graphContains(t, repo, "feature-x") {
			t.Error("feature-x should be in graph")
		}
		g := loadGraph(t, repo)
		if g.Branches["feature-x"].Parent != root {
			t.Errorf("feature-x parent = %q, want %q", g.Branches["feature-x"].Parent, root)
		}
	})

	t.Run("attach_parent_flag_with_untracked_parent_and_trunk", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create a develop branch (trunk name) that is NOT in the stack
		repo.CreateBranch("develop")
		repo.CreateFile("dev.txt", "develop content")
		repo.AddAndCommit("develop commit")

		// Create a feature branch off develop
		repo.CreateBranch("feature-y")
		repo.CreateFile("fy.txt", "feature y")
		repo.AddAndCommit("feature-y commit")

		// Attach feature-y with --parent develop.
		// develop is a trunk name and exists as a git branch but isn't in the stack.
		// Should auto-set develop as root and attach succeeds.
		runSt(t, "attach", "feature-y", "--parent", "develop")

		g := loadGraph(t, repo)
		if g.Root != "develop" {
			t.Errorf("root = %q, want develop", g.Root)
		}
		if !graphContains(t, repo, "feature-y") {
			t.Error("feature-y should be in graph after attach")
		}
		if g.Branches["feature-y"].Parent != "develop" {
			t.Errorf("feature-y parent = %q, want develop", g.Branches["feature-y"].Parent)
		}
	})

	t.Run("attach_parent_flag_with_tracked_branch_and_trunk_parent", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create a tracked branch
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 content")
		repo.AddAndCommit("f1 commit")

		// Create a develop branch (trunk name) not tracked
		repo.Checkout(root)
		repo.CreateBranch("develop")
		repo.CreateFile("dev.txt", "develop content")
		repo.AddAndCommit("develop commit")

		// Relocate f1 under develop (trunk name, not tracked).
		// Should auto-set develop as root and relocate.
		repo.Checkout("f1")
		runSt(t, "attach", "f1", "--parent", "develop")

		g := loadGraph(t, repo)
		if g.Root != "develop" {
			t.Errorf("root = %q, want develop", g.Root)
		}
		if g.Branches["f1"].Parent != "develop" {
			t.Errorf("f1 parent = %q, want develop", g.Branches["f1"].Parent)
		}
	})

	t.Run("attach_with_parent_flag", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create a tracked branch first so the graph is initialized
		runSt(t, "new", "f1")
		repo.Checkout(root)

		// Create an untracked branch manually
		repo.CreateBranch("manual-branch")
		repo.CreateFile("manual.txt", "manual")
		repo.AddAndCommit("manual commit")

		// Attach with --parent flag (skip TUI)
		runSt(t, "attach", "manual-branch", "--parent", root)

		if !graphContains(t, repo, "manual-branch") {
			t.Error("manual-branch should be in graph after attach --parent")
		}

		g := loadGraph(t, repo)
		if g.Branches["manual-branch"].Parent != root {
			t.Errorf("manual-branch parent = %q, want %q", g.Branches["manual-branch"].Parent, root)
		}
	})

	t.Run("attach_works_in_worktree", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create a tracked branch in the main repo first
		runSt(t, "new", "f1")

		// Create an untracked branch manually on top of root
		repo.Checkout(root)
		repo.CreateBranch("manual-branch")
		repo.CreateFile("manual.txt", "manual")
		repo.AddAndCommit("manual commit")
		repo.Checkout(root)

		// Create a worktree at HEAD — .git is a file here, not a directory
		wtDir, err := repo.AddWorktree("wt")
		if err != nil {
			t.Fatalf("AddWorktree: %v", err)
		}
		defer os.RemoveAll(wtDir)

		// Switch to the worktree and run attach from there.
		// This reproduces the exact bug: mkdir <worktree>/.git fails because
		// .git is a file in worktrees.
		oldWd, _ := os.Getwd()
		if err := os.Chdir(wtDir); err != nil {
			t.Fatalf("chdir to worktree: %v", err)
		}
		t.Cleanup(func() {
			os.Chdir(oldWd)
		})

		// Checkout the manual branch in the worktree so attach targets it
		wtCmd := exec.Command("git", "checkout", "manual-branch")
		wtCmd.Dir = wtDir
		if out, err := wtCmd.CombinedOutput(); err != nil {
			t.Fatalf("checkout manual-branch in worktree: %v\n%s", err, out)
		}

		// Attach --auto should succeed, not fail with "not a directory"
		out := runSt(t, "attach", "--auto")
		if strings.Contains(out, "not a directory") {
			t.Fatalf("attach failed in worktree with 'not a directory':\n%s", out)
		}

		// Verify the branch is in the graph (loaded from the worktree's git dir)
		sc, err := stcontext.Load(wtDir)
		if err != nil {
			t.Fatalf("Load from worktree: %v", err)
		}
		if _, ok := sc.Graph.Branches["manual-branch"]; !ok {
			t.Error("manual-branch should be in graph after attach --auto in worktree")
		}
	})
}

func TestAttachFilterStacked(t *testing.T) {
	t.Run("recursive_attach_excludes_stacked_branches", func(t *testing.T) {
		// Simulate the candidate-building logic from doAttachRecursively
		// with stopIfTracked=true (recursive path)
		g := graph.NewGraph("main")
		g.AddBranch("feature-1", "main", "abc", "def")
		g.AddBranch("feature-2", "feature-1", "def", "ghi")

		allBranches := []string{"main", "feature-1", "feature-2", "feature-3", "feature-4"}
		branchToAttach := "feature-3"
		stopIfTracked := true

		var candidates []attachCandidate
		seen := make(map[string]bool)
		for _, name := range allBranches {
			if name == branchToAttach {
				continue
			}
			if stopIfTracked && name != g.Root {
				if _, inGraph := g.Branches[name]; inGraph {
					continue
				}
			}
			if !seen[name] {
				seen[name] = true
				candidates = append(candidates, attachCandidate{name: name})
			}
		}

		// Should include: main (root), feature-4 (untracked)
		// Should exclude: feature-1, feature-2 (tracked in graph)
		if len(candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d: %v", len(candidates), candidateNames(candidates))
		}
		names := candidateNames(candidates)
		if names[0] != "main" {
			t.Errorf("expected first candidate 'main', got %q", names[0])
		}
		if names[1] != "feature-4" {
			t.Errorf("expected second candidate 'feature-4', got %q", names[1])
		}
	})

	t.Run("top_level_attach_shows_all_branches", func(t *testing.T) {
		// Simulate with stopIfTracked=false (top-level path)
		g := graph.NewGraph("main")
		g.AddBranch("feature-1", "main", "abc", "def")

		allBranches := []string{"main", "feature-1", "feature-2"}
		branchToAttach := "feature-2"
		stopIfTracked := false

		var candidates []attachCandidate
		seen := make(map[string]bool)
		for _, name := range allBranches {
			if name == branchToAttach {
				continue
			}
			if stopIfTracked && name != g.Root {
				if _, inGraph := g.Branches[name]; inGraph {
					continue
				}
			}
			if !seen[name] {
				seen[name] = true
				candidates = append(candidates, attachCandidate{name: name})
			}
		}

		// Should include all: main, feature-1 (even though tracked)
		if len(candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d: %v", len(candidates), candidateNames(candidates))
		}
	})
}

func candidateNames(candidates []attachCandidate) []string {
	var names []string
	for _, c := range candidates {
		names = append(names, c.name)
	}
	return names
}
