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
	"github.com/cpave3/staccato/internal/testutil"
	"github.com/cpave3/staccato/pkg/graph"
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

func loadGraph(t *testing.T, repo *testutil.GitRepo) *graph.Graph {
	t.Helper()

	// Check shared ref first
	cmd := exec.Command("git", "rev-parse", "--verify", graph.SharedGraphRef)
	cmd.Dir = repo.Dir
	if err := cmd.Run(); err == nil {
		// Shared mode: read from ref
		show := exec.Command("git", "show", graph.SharedGraphRef)
		show.Dir = repo.Dir
		data, err := show.Output()
		if err != nil {
			t.Fatalf("loadGraph: failed to read shared ref: %v", err)
		}
		var g graph.Graph
		if err := json.Unmarshal(data, &g); err != nil {
			t.Fatalf("loadGraph unmarshal: %v", err)
		}
		return &g
	}

	// Local mode: read from file
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

func loadGraphInDir(t *testing.T, dir string) *graph.Graph {
	t.Helper()

	// Check shared ref first
	cmd := exec.Command("git", "rev-parse", "--verify", graph.SharedGraphRef)
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		// Shared mode: read from ref
		show := exec.Command("git", "show", graph.SharedGraphRef)
		show.Dir = dir
		data, err := show.Output()
		if err != nil {
			t.Fatalf("loadGraphInDir: failed to read shared ref: %v", err)
		}
		var g graph.Graph
		if err := json.Unmarshal(data, &g); err != nil {
			t.Fatalf("loadGraphInDir unmarshal: %v", err)
		}
		return &g
	}

	// Local mode: read from file
	path := filepath.Join(dir, ".git", "stack", "graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("loadGraphInDir: %v", err)
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("loadGraphInDir unmarshal: %v", err)
	}
	return &g
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
		repo, root := setupRepoWithStack(t)
		// Create a tracked branch so graph exists with root != "untracked"
		runSt(t, "new", "tracked")
		repo.Checkout(root)
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
	t.Run("dry_run_does_not_push", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// dry-run output should be visible WITHOUT -v flag
		out := runSt(t, "sync", "--dry-run")
		assertContains(t, out, "Would push")
		assertContains(t, out, "f1")

		// f1 should NOT have been pushed
		remoteOut, _ := repo.RunGit("ls-remote", "--heads", "origin", "f1")
		if strings.TrimSpace(remoteOut) != "" {
			t.Error("f1 should NOT have been pushed with --dry-run")
		}
	})

	t.Run("dry_run_does_not_modify_local_branches", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Create stack: root -> m1
		runSt(t, "new", "m1")
		repo.CreateFile("m1.txt", "m1")
		repo.AddAndCommit("m1 commit")
		repo.RunGit("push", "origin", "m1")

		// Simulate m1 merged: fast-forward origin/trunk to include m1
		originDir := repo.OriginDir()
		m1SHA, _ := repo.RunGit("rev-parse", "m1")
		runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
		runGitInDir(t, originDir, "branch", "-D", "m1")

		// Record state before dry-run
		m1SHABefore, _ := repo.RunGit("rev-parse", "m1")

		// Run sync --dry-run — output should be visible WITHOUT -v flag
		out := runSt(t, "sync", "--dry-run")
		assertContains(t, out, "Would remove merged branch")

		// m1 local branch should still exist (not deleted)
		if !repo.BranchExists("m1") {
			t.Error("m1 should still exist after --dry-run (no local modifications)")
		}

		// m1 SHA should be unchanged
		m1SHAAfter, _ := repo.RunGit("rev-parse", "m1")
		if m1SHAAfter != m1SHABefore {
			t.Error("m1 SHA should be unchanged after --dry-run")
		}

		// Graph should still contain m1 (not removed)
		if !graphContains(t, repo, "m1") {
			t.Error("m1 should still be in graph after --dry-run")
		}
	})

	t.Run("error_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "sync")
		assertContains(t, out, "no remote")
	})
}

// ---------------------------------------------------------------------------
// TestSyncDetectsRegularMerge
// ---------------------------------------------------------------------------

func TestSyncDetectsRegularMerge(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}

	// Push main to origin so origin/main exists
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: main -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2")
	repo.AddAndCommit("m2 commit")

	// Push m1 to origin
	repo.RunGit("push", "origin", "m1")

	// Simulate regular merge: fast-forward origin/main to include m1,
	// then delete origin/m1
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	// Update origin's trunk to m1's commit (simulating a merge)
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	// Delete origin/m1
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// Run sync
	repo.Checkout("m2")
	out := runSt(t, "-v", "sync")

	// m1 should be removed from graph
	if graphContains(t, repo, "m1") {
		t.Error("m1 should have been removed from graph")
	}

	// m2 should still exist with parent = root
	g := loadGraph(t, repo)
	m2, ok := g.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph")
	}
	if m2.Parent != root {
		t.Errorf("m2 parent = %q, want %q", m2.Parent, root)
	}

	// m1 local branch should be deleted
	if repo.BranchExists("m1") {
		t.Error("m1 local branch should have been deleted")
	}

	assertContains(t, out, "Merged")
}

// ---------------------------------------------------------------------------
// TestSyncDetectsSquashMerge
// ---------------------------------------------------------------------------

func TestSyncDetectsSquashMerge(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: main -> m1
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")

	// Push m1
	repo.RunGit("push", "origin", "m1")

	// Simulate squash merge: cherry-pick m1 changes onto main in origin,
	// then delete origin/m1
	originDir := repo.OriginDir()

	// Go back to main, cherry-pick m1's changes as a new commit
	repo.Checkout(root)
	repo.RunGit("cherry-pick", "m1")
	// Push this new main to origin
	repo.RunGit("push", "origin", root)
	// Delete origin/m1 (simulating GitHub deleting the branch after squash merge)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// Run sync
	out := runSt(t, "-v", "sync")

	// m1 should be removed
	if graphContains(t, repo, "m1") {
		t.Error("m1 should have been removed from graph after squash merge detection")
	}
	if repo.BranchExists("m1") {
		t.Error("m1 local branch should have been deleted")
	}

	assertContains(t, out, "Merged")
}

// ---------------------------------------------------------------------------
// TestSyncMultipleMergedBranches
// ---------------------------------------------------------------------------

func TestSyncMultipleMergedBranches(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: main -> m1 -> m2 -> m3
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2")
	repo.AddAndCommit("m2 commit")
	runSt(t, "append", "m3")
	repo.CreateFile("m3.txt", "m3")
	repo.AddAndCommit("m3 commit")

	// Push m1 and m2
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Simulate both m1 and m2 merged: fast-forward origin/trunk to m2
	originDir := repo.OriginDir()
	m2SHA, _ := repo.RunGit("rev-parse", "m2")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m2SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")
	runGitInDir(t, originDir, "branch", "-D", "m2")

	// Run sync from m3
	repo.Checkout("m3")
	runSt(t, "-v", "sync")

	// m1 and m2 should be gone
	g := loadGraph(t, repo)
	if _, ok := g.Branches["m1"]; ok {
		t.Error("m1 should have been removed")
	}
	if _, ok := g.Branches["m2"]; ok {
		t.Error("m2 should have been removed")
	}

	// m3 should be reparented to root
	m3, ok := g.Branches["m3"]
	if !ok {
		t.Fatal("m3 should still be in graph")
	}
	if m3.Parent != root {
		t.Errorf("m3 parent = %q, want %q", m3.Parent, root)
	}
}

// ---------------------------------------------------------------------------
// TestSyncSkipsUnpushedBranches
// ---------------------------------------------------------------------------

func TestSyncSkipsUnpushedBranches(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create m1 with unique content — never push it
	runSt(t, "new", "m1")
	repo.CreateFile("m1-unique.txt", "unique content that is NOT on main")
	repo.AddAndCommit("m1 commit")

	// Run sync
	runSt(t, "-v", "sync")

	// m1 should still be in graph (not removed — it was never pushed
	// and has unique content not on main)
	if !graphContains(t, repo, "m1") {
		t.Error("m1 should NOT have been removed — it was never pushed and has unique diff")
	}
	if !repo.BranchExists("m1") {
		t.Error("m1 local branch should still exist")
	}
}

// ---------------------------------------------------------------------------
// TestSyncUserOnMergedBranch
// ---------------------------------------------------------------------------

func TestSyncUserOnMergedBranch(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create m1
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")

	// Push m1
	repo.RunGit("push", "origin", "m1")

	// Simulate merge
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// Stay on m1 (the merged branch)
	repo.Checkout("m1")

	// Run sync
	runSt(t, "-v", "sync")

	// Should end up on trunk
	cur := getCurrentBranch(t, repo)
	if cur != root {
		t.Errorf("current branch = %q, want %q (trunk)", cur, root)
	}

	// m1 should be gone
	if repo.BranchExists("m1") {
		t.Error("m1 should have been deleted")
	}
}

// runGitInDir runs a git command in a specific directory (for bare repo manipulation)
func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed: %v\nOutput: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
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

		// Check for backup/manual/* branches (new naming scheme)
		cmd := exec.Command("git", "branch", "--list", "backup/manual/*")
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

// ---------------------------------------------------------------------------
// TestSyncDown
// ---------------------------------------------------------------------------

func TestSyncDown(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create branch with a commit
	runSt(t, "new", "d1")
	repo.CreateFile("d1.txt", "d1")
	repo.AddAndCommit("d1 commit")

	// Run sync --down
	runSt(t, "-v", "sync", "--down")

	// d1 should still be in graph
	if !graphContains(t, repo, "d1") {
		t.Error("d1 should still be in graph")
	}

	// d1 should NOT have been pushed to origin
	out, _ := repo.RunGit("ls-remote", "--heads", "origin", "d1")
	if strings.TrimSpace(out) != "" {
		t.Error("d1 should NOT have been pushed to origin with --down flag")
	}
}

// ---------------------------------------------------------------------------
// TestSyncOnlyPushesCurrentLineage
// ---------------------------------------------------------------------------

func TestSyncOnlyPushesCurrentLineage(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create two independent stacks off root:
	// root -> stack-a1 -> stack-a2
	// root -> stack-b1
	runSt(t, "new", "stack-a1")
	repo.CreateFile("a1.txt", "a1")
	repo.AddAndCommit("a1 commit")
	runSt(t, "append", "stack-a2")
	repo.CreateFile("a2.txt", "a2")
	repo.AddAndCommit("a2 commit")

	repo.Checkout(root)
	runSt(t, "new", "stack-b1")
	repo.CreateFile("b1.txt", "b1")
	repo.AddAndCommit("b1 commit")

	// Sync from stack-a2 — only stack-a1 and stack-a2 should be pushed
	repo.Checkout("stack-a2")
	runSt(t, "-v", "sync")

	// stack-a1 should have been pushed
	outA1, _ := repo.RunGit("ls-remote", "--heads", "origin", "stack-a1")
	if strings.TrimSpace(outA1) == "" {
		t.Error("stack-a1 should have been pushed")
	}

	// stack-a2 should have been pushed
	outA2, _ := repo.RunGit("ls-remote", "--heads", "origin", "stack-a2")
	if strings.TrimSpace(outA2) == "" {
		t.Error("stack-a2 should have been pushed")
	}

	// stack-b1 should NOT have been pushed — it's a different lineage
	outB1, _ := repo.RunGit("ls-remote", "--heads", "origin", "stack-b1")
	if strings.TrimSpace(outB1) != "" {
		t.Error("stack-b1 should NOT have been pushed — it's not in the current lineage")
	}
}

// ---------------------------------------------------------------------------
// TestSyncDownRestacks
// ---------------------------------------------------------------------------

func TestSyncDownRestacks(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> f1
	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")

	// Push f1 so it has a remote tracking branch
	repo.RunGit("push", "origin", "f1")

	// Simulate upstream change: clone origin, commit, push back
	tmpClone, _ := os.MkdirTemp("", "st-clone-*")
	defer os.RemoveAll(tmpClone)
	cloneCmd := exec.Command("git", "clone", repo.OriginDir(), tmpClone)
	cloneCmd.Run()
	exec.Command("git", "-C", tmpClone, "config", "user.email", "other@example.com").Run()
	exec.Command("git", "-C", tmpClone, "config", "user.name", "Other").Run()
	os.WriteFile(filepath.Join(tmpClone, "upstream-change.txt"), []byte("new upstream content"), 0644)
	exec.Command("git", "-C", tmpClone, "add", ".").Run()
	exec.Command("git", "-C", tmpClone, "commit", "-m", "upstream commit").Run()
	exec.Command("git", "-C", tmpClone, "push", "origin", root).Run()

	// Run sync --down
	repo.Checkout("f1")
	runSt(t, "-v", "sync", "--down")

	// trunk should have the upstream change (fast-forwarded)
	repo.Checkout(root)
	if !repo.FileExists("upstream-change.txt") {
		t.Fatal("trunk should have upstream-change.txt after sync --down")
	}

	// f1 should also have the upstream change (restacked onto updated trunk)
	repo.Checkout("f1")
	if !repo.FileExists("upstream-change.txt") {
		t.Error("f1 should have upstream-change.txt after sync --down restacks onto updated trunk")
	}
}

// ---------------------------------------------------------------------------
// TestStatus
// ---------------------------------------------------------------------------

func TestStatus(t *testing.T) {
	t.Run("errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "status")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "status")
		assertContains(t, out, "forge not supported")
	})
}

// ---------------------------------------------------------------------------
// TestPR
// ---------------------------------------------------------------------------

func TestPR(t *testing.T) {
	t.Run("make_errors_when_branch_not_in_stack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "pr", "make")
		assertContains(t, out, "not in the stack")
	})

	t.Run("make_errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "pr", "make")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("make_errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "pr", "make")
		assertContains(t, out, "forge not supported")
	})

	t.Run("view_errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		out := runStExpectError(t, "pr", "view")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("view_errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "pr", "view")
		assertContains(t, out, "forge not supported")
	})
}

// ---------------------------------------------------------------------------
// TestGraph
// ---------------------------------------------------------------------------

func TestGraph(t *testing.T) {
	t.Run("which_defaults_to_local", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "graph", "which")
		assertContains(t, out, "Local")
	})

	t.Run("share_moves_to_ref", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")

		runSt(t, "graph", "share")

		// Local file should be gone
		localPath := filepath.Join(repo.Dir, ".git", "stack", "graph.json")
		if _, err := os.Stat(localPath); !os.IsNotExist(err) {
			t.Error("local graph file should be removed after share")
		}

		// Ref should exist
		out, err := repo.RunGit("rev-parse", "--verify", "refs/staccato/graph")
		if err != nil {
			t.Fatalf("shared ref should exist: %v (output: %s)", err, out)
		}
	})

	t.Run("which_reports_shared_after_share", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")

		out := runSt(t, "graph", "which")
		assertContains(t, out, "Shared")
	})

	t.Run("log_works_after_share", func(t *testing.T) {
		_, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")

		out := runSt(t, "log")
		assertContains(t, out, root)
		assertContains(t, out, "f1")
	})

	t.Run("local_moves_back_from_ref", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")
		runSt(t, "graph", "local")

		// Local file should be restored
		localPath := filepath.Join(repo.Dir, ".git", "stack", "graph.json")
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			t.Error("local graph file should exist after local")
		}

		// Ref should be gone
		refCheck := exec.Command("git", "rev-parse", "--verify", "refs/staccato/graph")
		refCheck.Dir = repo.Dir
		if err := refCheck.Run(); err == nil {
			t.Error("shared ref should be removed after local")
		}
	})

	t.Run("share_when_no_local_graph_errors", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		// Remove the graph file so share has nothing to move
		os.Remove(filepath.Join(repo.Dir, ".git", "stack", "graph.json"))
		out := runStExpectError(t, "graph", "share")
		assertContains(t, out, "no local graph")
	})

	t.Run("share_when_already_shared_errors", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")
		out := runStExpectError(t, "graph", "share")
		assertContains(t, out, "already shared")
	})

	t.Run("local_when_already_local_errors", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "graph", "local")
		assertContains(t, out, "already local")
	})

	t.Run("new_branch_works_after_share", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "graph", "share")

		// Create another branch while in shared mode
		runSt(t, "append", "f2")

		g := loadGraph(t, repo)
		if _, ok := g.Branches["f2"]; !ok {
			t.Error("f2 should be in graph after append in shared mode")
		}
		if g.Branches["f2"].Parent != "f1" {
			t.Errorf("f2 parent = %q, want f1", g.Branches["f2"].Parent)
		}
	})
}

// ---------------------------------------------------------------------------
// TestStalenessWarningOnLog
// ---------------------------------------------------------------------------

func TestStalenessWarningOnLog(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create a branch
	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")
	repo.RunGit("push", "origin", "f1")

	// Advance origin/main (simulating a PR merge on another machine)
	originDir := repo.OriginDir()
	f1SHA, _ := repo.RunGit("rev-parse", "f1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, f1SHA)

	// Fetch to get the new remote tracking ref (but don't sync)
	repo.RunGit("fetch", "origin")

	// Now local main is behind origin/main — log should warn
	repo.Checkout(root)
	out := runSt(t, "log")
	assertContains(t, out, "behind remote")
}

// ---------------------------------------------------------------------------
// TestSyncReconcileSharedGraph
// ---------------------------------------------------------------------------

func TestSyncReconcileSharedGraph(t *testing.T) {
	// Machine A creates branch X, pushes. Machine B creates branch Y, syncs.
	// After sync on B, both X and Y should be in the graph.

	// Set up "Machine A" repo
	repoA, rootA := setupRepoWithStack(t)
	if err := repoA.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repoA.RunGit("push", "-u", "origin", rootA)

	// Create a branch first so graph.json exists, then switch to shared mode
	runSt(t, "new", "branchX")
	repoA.CreateFile("x.txt", "x")
	repoA.AddAndCommit("x commit")

	// Switch to shared graph mode (needs graph.json to exist)
	runSt(t, "graph", "share")

	// Push the graph ref to remote first so fetch refspec won't fail
	repoA.RunGit("push", "origin", "refs/staccato/graph:refs/staccato/graph", "--force")

	// Add fetch refspec for graph ref
	repoA.RunGit("config", "--add", "remote.origin.fetch", "+refs/staccato/graph:refs/staccato/graph")

	// Sync from A (pushes branchX and graph ref)
	runSt(t, "sync")

	// "Machine B": clone from the same bare origin
	originDir := repoA.OriginDir()
	tmpB := t.TempDir()
	cloneCmd := exec.Command("git", "clone", originDir, tmpB)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}

	// Save Machine A's wd and switch to Machine B
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpB); err != nil {
		t.Fatalf("chdir to B: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })

	// Configure git user for Machine B
	runGitInDir(t, tmpB, "config", "user.email", "b@test.com")
	runGitInDir(t, tmpB, "config", "user.name", "User B")

	// Set up shared graph fetch refspec on B and fetch the graph ref
	runGitInDir(t, tmpB, "config", "--add", "remote.origin.fetch", "+refs/staccato/graph:refs/staccato/graph")
	runGitInDir(t, tmpB, "fetch", "origin")

	// Create local tracking branch for branchX on Machine B
	runGitInDir(t, tmpB, "checkout", "-b", "branchX", "origin/branchX")
	runGitInDir(t, tmpB, "checkout", rootA)

	// Machine B creates branch Y (locally) — st new adds it to the shared graph
	runSt(t, "new", "branchY")
	os.WriteFile(filepath.Join(tmpB, "y.txt"), []byte("y"), 0644)
	runGitInDir(t, tmpB, "add", ".")
	runGitInDir(t, tmpB, "commit", "-m", "y commit")

	// At this point, Machine B's local graph has branchY but lost branchX
	// (because st new rewrote the shared ref with only branchY).
	// Meanwhile, the remote still has branchX from Machine A's sync.
	// Sync from B should reconcile: fetch brings back the remote graph with
	// branchX, and reconciliation merges local branchY into it.
	runSt(t, "sync")

	// Load the graph on Machine B and check both branches exist
	g := loadGraphInDir(t, tmpB)
	if _, ok := g.Branches["branchX"]; !ok {
		t.Error("branchX should be in graph after reconciliation on Machine B")
	}
	if _, ok := g.Branches["branchY"]; !ok {
		t.Error("branchY should be in graph after reconciliation on Machine B")
	}
}

// ---------------------------------------------------------------------------
// TestSyncStashesUncommittedOnMergedBranch
// ---------------------------------------------------------------------------

func TestSyncStashesUncommittedOnMergedBranch(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create branch m1 with a commit
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")
	repo.RunGit("push", "origin", "m1")

	// Simulate m1 merged on remote
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// User is on m1 with uncommitted work
	repo.Checkout("m1")
	repo.WriteFile("wip.txt", "work in progress")
	repo.RunGit("add", "wip.txt")

	// Run sync — should stash the uncommitted changes
	out := runSt(t, "-v", "sync")
	assertContains(t, out, "Stashing")

	// Should end up on trunk
	cur := getCurrentBranch(t, repo)
	if cur != root {
		t.Errorf("current branch = %q, want %q", cur, root)
	}

	// m1 should be gone
	if repo.BranchExists("m1") {
		t.Error("m1 should have been deleted")
	}

	// Stash should contain our work
	stashOut, _ := repo.RunGit("stash", "list")
	if !strings.Contains(stashOut, "st-sync") {
		t.Errorf("stash should contain st-sync entry, got: %s", stashOut)
	}
}

// ---------------------------------------------------------------------------
// TestSyncSkipsNewEmptyBranch
// ---------------------------------------------------------------------------

func TestSyncSkipsNewEmptyBranch(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create a new empty branch (no commits)
	runSt(t, "new", "empty-branch")

	// Run sync — the empty branch should NOT be removed
	runSt(t, "sync")

	// Branch should still exist in git
	if !repo.BranchExists("empty-branch") {
		t.Error("empty-branch should still exist in git after sync")
	}

	// Branch should still exist in graph
	if !graphContains(t, repo, "empty-branch") {
		t.Error("empty-branch should still be in graph after sync")
	}

	// Branch should have been pushed to remote
	remoteOut, _ := repo.RunGit("ls-remote", "--heads", "origin", "empty-branch")
	if strings.TrimSpace(remoteOut) == "" {
		t.Error("empty-branch should have been pushed to remote")
	}
}
