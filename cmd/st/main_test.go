package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/st/internal/testutil"
	"github.com/user/st/pkg/graph"
)

// stBinary holds the path to the compiled binary, built once in TestMain.
var stBinary string

func TestMain(m *testing.M) {
	tmp, err := os.CreateTemp("", "st-bin-*")
	if err != nil {
		panic(err)
	}
	tmp.Close()
	stBinary = tmp.Name()

	build := exec.Command("go", "build", "-o", stBinary, "./cmd/st/")
	build.Dir = filepath.Join(mustGetwd(), "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	code := m.Run()
	os.Remove(stBinary)
	os.Exit(code)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupRepo creates a temp git repo and chdir into it.
func setupRepo(t *testing.T) *testutil.GitRepo {
	t.Helper()
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("NewGitRepo: %v", err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(repo.Dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(oldWd)
		repo.Cleanup()
	})
	return repo
}

// setupRepoWithStack creates a repo with InitStack called, returns the root branch name.
func setupRepoWithStack(t *testing.T) (*testutil.GitRepo, string) {
	t.Helper()
	repo := setupRepo(t)
	if err := repo.InitStack(); err != nil {
		t.Fatalf("InitStack: %v", err)
	}
	root := getCurrentBranch(t, repo)
	return repo, root
}

// runSt executes the st binary and fails on error.
func runSt(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(stBinary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st %v failed: %v\nOutput: %s", args, err, out)
	}
	return string(out)
}

// runStExpectError executes the st binary and fails if there is NO error.
func runStExpectError(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(stBinary, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("st %v expected error but succeeded\nOutput: %s", args, out)
	}
	return string(out)
}

func getCurrentBranch(t *testing.T, repo *testutil.GitRepo) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repo.Dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("getCurrentBranch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func getHeadSHA(t *testing.T, repo *testutil.GitRepo) string {
	t.Helper()
	return repo.HeadSHA()
}

func loadGraph(t *testing.T, repo *testutil.GitRepo) *graph.Graph {
	t.Helper()
	path := filepath.Join(repo.Dir, ".git", "stack", "graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("loadGraph: %v", err)
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("loadGraph unmarshal: %v", err)
	}
	return &g
}

func graphContains(t *testing.T, repo *testutil.GitRepo, branch string) bool {
	t.Helper()
	g := loadGraph(t, repo)
	_, ok := g.Branches[branch]
	return ok
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}

// ---------------------------------------------------------------------------
// TestNew
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Run("creates_branch_and_updates_graph", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		if !repo.BranchExists("feature-1") {
			t.Error("branch feature-1 not created")
		}

		g := loadGraph(t, repo)
		b, ok := g.Branches["feature-1"]
		if !ok {
			t.Fatal("feature-1 not in graph")
		}
		if b.Parent != root {
			t.Errorf("parent = %q, want %q", b.Parent, root)
		}
		if cur := getCurrentBranch(t, repo); cur != "feature-1" {
			t.Errorf("current branch = %q, want feature-1", cur)
		}
	})

	t.Run("multiple_branches_from_root", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "a")
		repo.Checkout(root)
		runSt(t, "new", "b")

		g := loadGraph(t, repo)
		for _, name := range []string{"a", "b"} {
			b, ok := g.Branches[name]
			if !ok {
				t.Fatalf("%s not in graph", name)
			}
			if b.Parent != root {
				t.Errorf("%s parent = %q, want %q", name, b.Parent, root)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestAppend
// ---------------------------------------------------------------------------

func TestAppend(t *testing.T) {
	t.Run("creates_child_from_current", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")

		g := loadGraph(t, repo)
		b, ok := g.Branches["f2"]
		if !ok {
			t.Fatal("f2 not in graph")
		}
		if b.Parent != "f1" {
			t.Errorf("f2 parent = %q, want f1", b.Parent)
		}
		if cur := getCurrentBranch(t, repo); cur != "f2" {
			t.Errorf("current branch = %q, want f2", cur)
		}
	})

	t.Run("error_current_not_in_stack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		// Create a tracked branch so graph exists with root != "untracked"
		runSt(t, "new", "tracked")
		repo.Checkout("main")
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "append", "child")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestInsert
// ---------------------------------------------------------------------------

func TestInsert(t *testing.T) {
	t.Run("inserts_before_current_and_reparents", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "insert", "pre-f")

		g := loadGraph(t, repo)
		pref, ok := g.Branches["pre-f"]
		if !ok {
			t.Fatal("pre-f not in graph")
		}
		if pref.Parent != root {
			t.Errorf("pre-f parent = %q, want %q", pref.Parent, root)
		}

		f1, ok := g.Branches["f1"]
		if !ok {
			t.Fatal("f1 not in graph")
		}
		if f1.Parent != "pre-f" {
			t.Errorf("f1 parent = %q, want pre-f", f1.Parent)
		}

		if cur := getCurrentBranch(t, repo); cur != "pre-f" {
			t.Errorf("current branch = %q, want pre-f", cur)
		}
	})

	t.Run("error_current_not_in_stack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "insert", "x")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestRestack
// ---------------------------------------------------------------------------

func TestRestack(t *testing.T) {
	t.Run("rebases_child_onto_diverged_parent", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// Diverge root
		repo.Checkout(root)
		repo.CreateFile("root-update.txt", "new stuff")
		repo.AddAndCommit("root diverge")

		// Restack from f1
		repo.Checkout("f1")
		runSt(t, "restack")

		// f1 should now have the root's file
		if !repo.FileExists("root-update.txt") {
			t.Error("f1 should have root-update.txt after restack")
		}
	})

	t.Run("to_current_flag", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Diverge root
		repo.Checkout(root)
		repo.CreateFile("root2.txt", "new")
		repo.AddAndCommit("root diverge")

		// Checkout f1 (not the tip), use --to-current
		repo.Checkout("f1")
		runSt(t, "restack", "--to-current")

		if !repo.FileExists("root2.txt") {
			t.Error("f1 should have root2.txt after restack --to-current")
		}
	})

	t.Run("error_not_at_tip_without_flag", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		repo.Checkout(root)
		repo.CreateFile("root3.txt", "diverge")
		repo.AddAndCommit("root diverge")

		repo.Checkout("f1")
		out := runStExpectError(t, "restack")
		assertContains(t, out, "--to-current")
	})
}

// ---------------------------------------------------------------------------
// TestContinue
// ---------------------------------------------------------------------------

func TestContinue(t *testing.T) {
	t.Run("resumes_after_conflict", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.AddAndCommit("f1 commit")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack should fail with conflict
		repo.Checkout("f1")
		runStExpectError(t, "restack")

		// Resolve the conflict
		repo.WriteFile("shared.txt", "resolved content")
		cmd := exec.Command("git", "add", "shared.txt")
		cmd.Dir = repo.Dir
		cmd.Run()

		// GIT_EDITOR=true so rebase --continue doesn't open editor
		contCmd := exec.Command(stBinary, "continue")
		contCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		out, err := contCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("st continue failed: %v\nOutput: %s", err, out)
		}
	})

	t.Run("error_no_rebase_in_progress", func(t *testing.T) {
		setupRepoWithStack(t)
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no rebase in progress")
	})
}

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
}

// ---------------------------------------------------------------------------
// TestRestore
// ---------------------------------------------------------------------------

func TestRestore(t *testing.T) {
	t.Run("restores_from_automatic_backup", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create f1 with a commit
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 original")
		repo.AddAndCommit("f1 commit")

		// Manually create a backup branch matching the automatic backup format
		// (backup/<branch>/<timestamp>) so restore can find it
		cmd := exec.Command("git", "branch", "backup/f1/1234567890", "f1")
		cmd.Dir = repo.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to create backup branch: %v\n%s", err, out)
		}

		// Restore f1 from the backup
		runSt(t, "restore", "f1")
	})

	t.Run("error_no_backups", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "restore", "f1")
		assertContains(t, out, "no backups found")
	})
}

// ---------------------------------------------------------------------------
// TestSync
// ---------------------------------------------------------------------------

func TestSync(t *testing.T) {
	t.Run("dry_run_lists_branches", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		out := runSt(t, "-v", "sync", "--dry-run")
		assertContains(t, out, "f1")
	})

	t.Run("error_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "sync")
		assertContains(t, out, "no remote")
	})
}

// ---------------------------------------------------------------------------
// TestLog
// ---------------------------------------------------------------------------

func TestLog(t *testing.T) {
	t.Run("displays_tree", func(t *testing.T) {
		_, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "log")
		assertContains(t, out, root)
		assertContains(t, out, "f1")
	})

	t.Run("shows_current_marker", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "log")
		assertContains(t, out, "●")
	})
}

// ---------------------------------------------------------------------------
// TestSwitchTUI — model-level tests, no binary
// ---------------------------------------------------------------------------

func TestSwitchTUI(t *testing.T) {
	makeSwitchModel := func(branches []branchItem) switchTUI {
		var items []list.Item
		for _, b := range branches {
			items = append(items, b)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)
		return switchTUI{
			list:     l,
			branches: branches,
			current:  "main",
		}
	}

	t.Run("enter_selects_branch", func(t *testing.T) {
		branches := []branchItem{
			{name: "main", current: true, depth: 0},
			{name: "feature-1", current: false, depth: 1},
		}
		model := makeSwitchModel(branches)
		// Select index 1
		model.list.Select(1)

		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m := updated.(switchTUI)
		if m.selected != "feature-1" {
			t.Errorf("selected = %q, want feature-1", m.selected)
		}
	})

	t.Run("search_mode", func(t *testing.T) {
		branches := []branchItem{
			{name: "main", current: true, depth: 0},
			{name: "api-endpoint", current: false, depth: 1},
			{name: "feature-1", current: false, depth: 1},
		}
		model := makeSwitchModel(branches)

		// Enter search mode
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		m := updated.(switchTUI)
		if !m.searchMode {
			t.Fatal("should be in search mode")
		}

		// Type "api"
		for _, ch := range "api" {
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
			m = updated.(switchTUI)
		}

		if len(m.matches) != 1 {
			t.Fatalf("matches = %d, want 1", len(m.matches))
		}
		if m.matches[0] != 1 {
			t.Errorf("match index = %d, want 1", m.matches[0])
		}
	})

	t.Run("q_quits", func(t *testing.T) {
		branches := []branchItem{
			{name: "main", current: true, depth: 0},
		}
		model := makeSwitchModel(branches)

		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		m := updated.(switchTUI)
		if !m.quitting {
			t.Error("should be quitting")
		}
		if m.selected != "" {
			t.Errorf("selected = %q, want empty", m.selected)
		}
	})
}

// ---------------------------------------------------------------------------
// TestBackup
// ---------------------------------------------------------------------------

func TestBackup(t *testing.T) {
	t.Run("creates_manual_backup_branches", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "backup")

		// Check for backups/* branches
		cmd := exec.Command("git", "branch", "--list", "backups/*")
		cmd.Dir = repo.Dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git branch --list: %v", err)
		}
		if !strings.Contains(string(out), "f1") {
			t.Errorf("backup branches should contain f1, got: %s", out)
		}
	})

	t.Run("error_no_branches", func(t *testing.T) {
		setupRepoWithStack(t)
		out := runStExpectError(t, "backup")
		assertContains(t, out, "no branches")
	})
}
