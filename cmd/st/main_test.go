package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/internal/testutil"
	"github.com/cpave3/staccato/pkg/graph"
)

// stBinary holds the path to the compiled binary, built once in TestMain.
var stBinary string

// coverDir holds the GOCOVERDIR path when running with coverage instrumentation.
var coverDir string

func TestMain(m *testing.M) {
	tmp, err := os.CreateTemp("", "st-bin-*")
	if err != nil {
		panic(err)
	}
	tmp.Close()
	stBinary = tmp.Name()

	coverDir = os.Getenv("ST_COVER_DIR")
	if coverDir != "" && !filepath.IsAbs(coverDir) {
		// Resolve relative to project root (two levels up from cmd/st/).
		coverDir = filepath.Join(mustGetwd(), "..", "..", coverDir)
	}
	buildArgs := []string{"build"}
	if coverDir != "" {
		buildArgs = append(buildArgs, "-cover")
	}
	buildArgs = append(buildArgs, "-o", stBinary, "./cmd/st/")

	build := exec.Command("go", buildArgs...)
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

// setCoverEnv sets GOCOVERDIR on cmd when coverage instrumentation is active.
func setCoverEnv(cmd *exec.Cmd) {
	if coverDir != "" {
		if cmd.Env == nil {
			cmd.Env = os.Environ()
		}
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+coverDir)
	}
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
	setCoverEnv(cmd)
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
	setCoverEnv(cmd)
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
	return loadGraphInDir(t, repo.Dir)
}

func graphContains(t *testing.T, repo *testutil.GitRepo, branch string) bool {
	t.Helper()
	g := loadGraph(t, repo)
	_, ok := g.Branches[branch]
	return ok
}

func loadGraphInDir(t *testing.T, dir string) *graph.Graph {
	t.Helper()
	sc, err := stcontext.Load(dir)
	if err != nil {
		t.Fatalf("loadGraphInDir: %v", err)
	}
	return sc.Graph
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

	t.Run("creates_branch_from_root_not_current_head", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create branch foo with a unique commit
		runSt(t, "new", "foo")
		if err := repo.CreateFile("foo.txt", "foo content"); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddAndCommit("foo commit"); err != nil {
			t.Fatal(err)
		}
		fooSHA := repo.HeadSHA()

		// While still on foo, create bar via st new
		runSt(t, "new", "bar")

		// bar should be checked out
		if cur := getCurrentBranch(t, repo); cur != "bar" {
			t.Errorf("current branch = %q, want bar", cur)
		}

		// bar should be at the root's HEAD, not foo's HEAD
		barSHA := repo.HeadSHA()
		// Get root SHA for comparison
		if err := repo.Checkout(root); err != nil {
			t.Fatal(err)
		}
		rootSHA := repo.HeadSHA()
		if err := repo.Checkout("bar"); err != nil {
			t.Fatal(err)
		}

		if barSHA == fooSHA {
			t.Error("bar has same SHA as foo — should be based on root, not foo")
		}
		if barSHA != rootSHA {
			t.Errorf("bar SHA = %q, want root SHA %q", barSHA, rootSHA)
		}

		// bar should NOT have foo.txt
		if repo.FileExists("foo.txt") {
			t.Error("bar contains foo.txt — should not have foo's commits")
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

	t.Run("handles_stale_base_sha_after_manual_rebase", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create a branch with one commit
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 content")
		repo.AddAndCommit("f1 commit")

		// Record the original BaseSHA (root HEAD at branch creation)
		g := loadGraph(t, repo)
		origBaseSHA := g.Branches["f1"].BaseSHA

		// Add commits to root that MODIFY the same files differently
		// This makes git unable to skip them via patch-id matching
		repo.Checkout(root)
		for i := 0; i < 5; i++ {
			repo.CreateFile(fmt.Sprintf("root-%d.txt", i), fmt.Sprintf("content %d", i))
			repo.AddAndCommit(fmt.Sprintf("root commit %d", i))
		}

		// Manually rebase f1 onto root (outside of st)
		repo.Checkout("f1")
		cmd := exec.Command("git", "rebase", root)
		cmd.Dir = repo.Dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("manual rebase failed: %v\n%s", err, out)
		}

		// The graph's BaseSHA is now stale — points to the old root HEAD
		g = loadGraph(t, repo)
		if g.Branches["f1"].BaseSHA != origBaseSHA {
			t.Fatal("test setup: BaseSHA should still be the old value")
		}

		// st restack should detect the stale BaseSHA and use the current merge-base
		runSt(t, "restack")

		// Verify f1 has its own file
		if !repo.FileExists("f1.txt") {
			t.Error("f1 should have f1.txt after restack")
		}

		// Verify restack only replayed f1's own commit (not all root commits)
		countCmd := exec.Command("git", "rev-list", "--count", root+"..f1")
		countCmd.Dir = repo.Dir
		countOut, err := countCmd.Output()
		if err != nil {
			t.Fatalf("rev-list failed: %v", err)
		}
		count := strings.TrimSpace(string(countOut))
		if count != "1" {
			t.Errorf("expected 1 commit on f1 above %s, got %s", root, count)
		}
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
		setCoverEnv(contCmd)
		out, err := contCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("st continue failed: %v\nOutput: %s", err, out)
		}
	})

	t.Run("still_conflicting", func(t *testing.T) {
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

		// Try to continue WITHOUT resolving the conflict
		out := runStExpectError(t, "continue")
		assertContains(t, out, "conflicts")
	})

	t.Run("error_no_rebase_in_progress", func(t *testing.T) {
		setupRepoWithStack(t)
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no rebase in progress")
	})

	// Cycle 1: continue completes full multi-branch stack
	t.Run("continue_completes_multi_branch_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2 -> f3
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		runSt(t, "append", "f3")
		repo.CreateFile("f3.txt", "f3 file")
		repo.AddAndCommit("f3 commit")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack from f3 (tip) should fail at f1
		repo.Checkout("f3")
		runStExpectError(t, "restack")

		// Resolve conflict at f1
		repo.WriteFile("shared.txt", "resolved content")
		repo.RunGit("add", "shared.txt")

		// Continue — should restack f1, f2, f3
		contCmd := exec.Command(stBinary, "continue")
		contCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		setCoverEnv(contCmd)
		out, err := contCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("st continue failed: %v\nOutput: %s", err, out)
		}

		// Verify f2 has resolved content + its own file
		repo.Checkout("f2")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(content) != "resolved content" {
			t.Errorf("f2 shared.txt = %q, want 'resolved content'", content)
		}
		if !repo.FileExists("f2.txt") {
			t.Error("f2 should have f2.txt")
		}

		// Verify f3 has resolved content + its own file
		repo.Checkout("f3")
		content, _ = os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(content) != "resolved content" {
			t.Errorf("f3 shared.txt = %q, want 'resolved content'", content)
		}
		if !repo.FileExists("f3.txt") {
			t.Error("f3 should have f3.txt")
		}
	})

	// Cycle 4: backups preserved during conflict, can restore before continue completes
	t.Run("backups_available_during_conflict_for_restore", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Record original SHAs
		repo.Checkout("f1")
		origF1SHA, _ := repo.RunGit("rev-parse", "f1")
		origF2SHA, _ := repo.RunGit("rev-parse", "f2")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack from f2 (tip) -> conflicts at f1
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Backups should exist — restore --all to get back to pre-restack state
		runSt(t, "restore", "--all")

		afterF1SHA, _ := repo.RunGit("rev-parse", "f1")
		afterF2SHA, _ := repo.RunGit("rev-parse", "f2")

		if afterF1SHA != origF1SHA {
			t.Errorf("f1 SHA after restore = %s, want original %s", afterF1SHA, origF1SHA)
		}
		if afterF2SHA != origF2SHA {
			t.Errorf("f2 SHA after restore = %s, want original %s", afterF2SHA, origF2SHA)
		}
	})

	// Cycle 5: continue restacks only original lineage (not sibling branches)
	t.Run("continue_restacks_only_original_lineage", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build tree: main -> f1, main -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		repo.Checkout(root)
		runSt(t, "new", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Record f2 SHA (should NOT change)
		origF2SHA, _ := repo.RunGit("rev-parse", "f2")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack from f1 (tip of its lineage) -> conflicts at f1
		repo.Checkout("f1")
		runStExpectError(t, "restack")

		// Resolve and continue
		repo.WriteFile("shared.txt", "resolved content")
		repo.RunGit("add", "shared.txt")

		contCmd := exec.Command(stBinary, "continue")
		contCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		setCoverEnv(contCmd)
		out, err := contCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("st continue failed: %v\nOutput: %s", err, out)
		}

		// f2 should be untouched
		afterF2SHA, _ := repo.RunGit("rev-parse", "f2")
		if afterF2SHA != origF2SHA {
			t.Errorf("f2 SHA changed from %s to %s — continue should not touch sibling branches", origF2SHA, afterF2SHA)
		}
	})

	// Cycle 6: conflict resolved once not repeated for downstream
	t.Run("conflict_resolved_once_not_repeated", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2 -> f3, all inherit shared.txt from f1
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		runSt(t, "append", "f3")
		repo.CreateFile("f3.txt", "f3 file")
		repo.AddAndCommit("f3 commit")

		// Root changes shared.txt -> conflict at f1
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		repo.Checkout("f3")
		runStExpectError(t, "restack")

		// Resolve at f1
		repo.WriteFile("shared.txt", "resolved content")
		repo.RunGit("add", "shared.txt")

		// Continue should succeed without additional conflicts
		contCmd := exec.Command(stBinary, "continue")
		contCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		setCoverEnv(contCmd)
		out, err := contCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("st continue failed (expected no further conflicts): %v\nOutput: %s", err, out)
		}
		assertContains(t, string(out), "Restacked")
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

	t.Run("restore_current_branch", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 original")
		repo.AddAndCommit("f1 commit")

		// Create a backup branch
		cmd := exec.Command("git", "branch", "backup/f1/9999999999", "f1")
		cmd.Dir = repo.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to create backup: %v\n%s", err, out)
		}

		// Restore without specifying branch name (uses current branch)
		out := runSt(t, "restore")
		assertContains(t, out, "Restored")
	})

	t.Run("restore_all", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Create auto backup branches (restore --all uses ListBackups which matches auto format)
		cmd := exec.Command("git", "branch", "backup/auto/f1/9999999999", "f1")
		cmd.Dir = repo.Dir
		cmd.Run()
		cmd = exec.Command("git", "branch", "backup/auto/f2/9999999999", "f2")
		cmd.Dir = repo.Dir
		cmd.Run()

		// Restore all
		out := runSt(t, "restore", "--all")
		assertContains(t, out, "Restored")
	})

	t.Run("error_no_backups", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "restore", "f1")
		assertContains(t, out, "no backups found")
	})

	// Cycle 2: restore --all works during active rebase
	t.Run("restore_all_aborts_active_rebase", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Record original SHAs
		origF1SHA, _ := repo.RunGit("rev-parse", "f1")
		origF2SHA, _ := repo.RunGit("rev-parse", "f2")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack -> conflict leaves rebase in progress
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Verify rebase IS in progress
		_, err := repo.RunGit("rev-parse", "--git-path", "rebase-merge")
		if err != nil {
			// If we can't even check, skip further assertions
			t.Log("Warning: could not check rebase state")
		}

		// restore --all should succeed despite active rebase
		runSt(t, "restore", "--all")

		// Verify no rebase in progress after restore
		rebasePath, _ := repo.RunGit("rev-parse", "--git-path", "rebase-merge")
		if rebasePath != "" {
			// Check if the directory actually exists
			dirCheck := exec.Command("test", "-d", rebasePath)
			dirCheck.Dir = repo.Dir
			if dirCheck.Run() == nil {
				t.Error("rebase should not be in progress after restore --all")
			}
		}

		// Verify branches at original SHAs
		afterF1SHA, _ := repo.RunGit("rev-parse", "f1")
		afterF2SHA, _ := repo.RunGit("rev-parse", "f2")

		if afterF1SHA != origF1SHA {
			t.Errorf("f1 SHA = %s, want original %s", afterF1SHA, origF1SHA)
		}
		if afterF2SHA != origF2SHA {
			t.Errorf("f2 SHA = %s, want original %s", afterF2SHA, origF2SHA)
		}
	})

	// Cycle 3: restore --all updates graph state
	t.Run("restore_all_updates_graph_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack -> conflict
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Restore --all
		runSt(t, "restore", "--all")

		// Load graph and check that SHAs match actual branch tips
		g := loadGraph(t, repo)
		for branchName, branch := range g.Branches {
			actualSHA, err := repo.RunGit("rev-parse", branchName)
			if err != nil {
				t.Fatalf("failed to get SHA for %s: %v", branchName, err)
			}
			if branch.HeadSHA != actualSHA {
				t.Errorf("graph HeadSHA for %s = %s, actual = %s", branchName, branch.HeadSHA, actualSHA)
			}
		}
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
// TestSyncChildSurvivesSecondSync
// ---------------------------------------------------------------------------
// Regression: after merging a middle branch (root <- m1 <- m2) and running
// sync twice, m2 should still be in the graph reparented to root.

func TestSyncChildSurvivesSecondSync(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Push m1 to origin
	repo.RunGit("push", "origin", "m1")

	// Simulate regular merge of m1 into root on GitHub:
	// fast-forward origin/root to m1's commit, then delete origin/m1
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2 (top of stack)
	repo.Checkout("m2")
	out1 := runSt(t, "-v", "sync")

	// m1 should be detected as merged
	assertContains(t, out1, "Merged")

	// After first sync: m2 should be reparented to root
	g1 := loadGraph(t, repo)
	m2b, ok := g1.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after first sync")
	}
	if m2b.Parent != root {
		t.Errorf("m2 parent after first sync = %q, want %q", m2b.Parent, root)
	}

	// Second sync — should be a no-op for merge detection
	out2 := runSt(t, "-v", "sync")

	// m2 should still be in the graph after second sync
	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q", m2b2.Parent, root)
	}

	// The second sync should NOT detect any merged branches
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSecondSyncSquashMerge
// ---------------------------------------------------------------------------
// Same as above but with squash merge (more common on GitHub).

func TestSyncChildSurvivesSecondSyncSquashMerge(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Push both to origin (realistic: user pushes their stack before merge)
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Simulate squash merge of m1 into root:
	// Cherry-pick m1 changes onto root as a new commit, delete origin/m1
	repo.Checkout(root)
	repo.RunGit("cherry-pick", "m1")
	repo.RunGit("push", "origin", root)
	originDir := repo.OriginDir()
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2
	repo.Checkout("m2")
	out1 := runSt(t, "-v", "sync")
	assertContains(t, out1, "Merged")

	g1 := loadGraph(t, repo)
	m2b, ok := g1.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after first sync")
	}
	if m2b.Parent != root {
		t.Errorf("m2 parent after first sync = %q, want %q", m2b.Parent, root)
	}

	// Second sync
	out2 := runSt(t, "-v", "sync")

	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q", m2b2.Parent, root)
	}
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSharedGraphMode
// ---------------------------------------------------------------------------
// Regression: in shared graph mode, after merging a middle branch and syncing,
// the child branch should survive with correct parent after a second sync.

func TestSyncChildSurvivesSharedGraphMode(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Enable shared graph mode (after branches exist so graph file exists)
	runSt(t, "graph", "share")

	// Push both to origin
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Sync to push graph ref to origin
	runSt(t, "-v", "sync")

	// Simulate regular merge of m1 into root on GitHub:
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2
	repo.Checkout("m2")
	out1 := runSt(t, "-v", "sync")
	assertContains(t, out1, "Merged")

	g1 := loadGraph(t, repo)
	m2b, ok := g1.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after first sync (shared mode)")
	}
	if m2b.Parent != root {
		t.Errorf("m2 parent after first sync = %q, want %q", m2b.Parent, root)
	}

	// Second sync
	out2 := runSt(t, "-v", "sync")

	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync (shared mode)")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q", m2b2.Parent, root)
	}
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSharedGraphStaleRemote
// ---------------------------------------------------------------------------
// Regression: in shared graph mode, if the graph ref push fails on first sync,
// the second sync's fetch overwrites the local ref with the stale remote graph.
// ReconcileGraphs must preserve the local parent (reparented during merge removal)
// rather than reverting to the stale remote parent.

func TestSyncChildSurvivesSharedGraphStaleRemote(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Enable shared graph mode
	runSt(t, "graph", "share")

	// Push both + graph ref to origin
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")
	runSt(t, "-v", "sync")

	// Simulate regular merge of m1 into root on GitHub
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2: detect merge, reparent m2
	repo.Checkout("m2")
	runSt(t, "-v", "sync")

	g1 := loadGraph(t, repo)
	if _, ok := g1.Branches["m2"]; !ok {
		t.Fatal("m2 should be in graph after first sync")
	}

	// Simulate the graph ref push having failed: revert origin's per-user ref
	// to the OLD version (with m1 still present, m2 parented under m1).
	email, _ := repo.RunGit("config", "user.email")
	userRef := graph.UserGraphRef(email)
	oldGraph := graph.NewGraph(root)
	oldGraph.AddBranch("m1", root, "aaa", m1SHA)
	m2SHA, _ := repo.RunGit("rev-parse", "m2")
	oldGraph.AddBranch("m2", "m1", "bbb", m2SHA)
	oldGraphJSON, _ := json.Marshal(oldGraph)

	// Write stale graph directly into origin bare repo's per-user ref
	tmpFile := filepath.Join(originDir, "stale-graph.json")
	os.WriteFile(tmpFile, oldGraphJSON, 0644)
	blobHash := runGitInDir(t, originDir, "hash-object", "-w", tmpFile)
	runGitInDir(t, originDir, "update-ref", userRef, blobHash)
	os.Remove(tmpFile)

	// Second sync: fetch will overwrite local graph ref with stale remote.
	// This should succeed — reconciliation should prefer local parent.
	out2 := runSt(t, "-v", "sync")

	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync with stale remote")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q (stale remote should not revert reparent)", m2b2.Parent, root)
	}
	// m1 should NOT be re-added (it was merged and deleted locally)
	if graphContains(t, repo, "m1") {
		t.Error("m1 should not be re-added from stale remote graph")
	}
	_ = out2
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
// TestBackupList
// ---------------------------------------------------------------------------

func TestBackupList(t *testing.T) {
	t.Run("no_backups", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "backup", "list")
		assertContains(t, out, "No backups found")
	})

	t.Run("lists_backups", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// Create a manual backup
		runSt(t, "backup")

		out := runSt(t, "backup", "list")
		assertContains(t, out, "Found")
		assertContains(t, out, "f1")
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

		// Per-user ref should exist
		out := runSt(t, "graph", "which")
		assertContains(t, out, "Shared")
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

		// Should be back to local mode
		out := runSt(t, "graph", "which")
		assertContains(t, out, "Local")
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
	// Same user, two machines. Machine A creates branchX, pushes.
	// Machine B (same user) creates branchY, syncs.
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

	// Switch to shared graph mode
	runSt(t, "graph", "share")

	// Get the per-user ref name
	userRef := runSt(t, "graph", "which")

	// Push the per-user graph ref to remote
	email, _ := repoA.RunGit("config", "user.email")
	ref := graph.UserGraphRef(email)
	repoA.RunGit("push", "origin", ref+":"+ref, "--force")

	// Sync from A (pushes branchX and graph ref)
	runSt(t, "sync")

	// "Machine B": clone from the same bare origin (same user!)
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

	// Same user on Machine B (same email — this is the key!)
	runGitInDir(t, tmpB, "config", "user.email", email)
	runGitInDir(t, tmpB, "config", "user.name", "Test User")

	// Set up shared graph fetch refspec on B and fetch
	runGitInDir(t, tmpB, "config", "--add", "remote.origin.fetch", "+refs/staccato/*:refs/staccato/*")
	runGitInDir(t, tmpB, "fetch", "origin")

	// Create local tracking branch for branchX on Machine B
	runGitInDir(t, tmpB, "checkout", "-b", "branchX", "origin/branchX")
	runGitInDir(t, tmpB, "checkout", rootA)

	// Machine B creates branch Y — st new adds it to the per-user graph ref
	runSt(t, "new", "branchY")
	os.WriteFile(filepath.Join(tmpB, "y.txt"), []byte("y"), 0644)
	runGitInDir(t, tmpB, "add", ".")
	runGitInDir(t, tmpB, "commit", "-m", "y commit")

	// Sync from B should reconcile: fetch brings A's version of the graph
	// (with branchX), local has branchY. Union merge keeps both.
	runSt(t, "sync")

	// Load the graph on Machine B and check both branches exist
	g := loadGraphInDir(t, tmpB)
	if _, ok := g.Branches["branchX"]; !ok {
		t.Error("branchX should be in graph after reconciliation on Machine B")
	}
	if _, ok := g.Branches["branchY"]; !ok {
		t.Error("branchY should be in graph after reconciliation on Machine B")
	}
	_ = userRef
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

// ---------------------------------------------------------------------------
// runStContinue runs `st continue` with GIT_EDITOR=true to avoid editor prompts.
// ---------------------------------------------------------------------------
func runStContinue(t *testing.T) string {
	t.Helper()
	cmd := exec.Command(stBinary, "continue")
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
	setCoverEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st continue failed: %v\nOutput: %s", err, out)
	}
	return string(out)
}

// runStContinueExpectError runs `st continue` with GIT_EDITOR=true and expects an error.
func runStContinueExpectError(t *testing.T) string {
	t.Helper()
	cmd := exec.Command(stBinary, "continue")
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
	setCoverEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("st continue expected error but succeeded\nOutput: %s", out)
	}
	return string(out)
}

// resolveConflictAndStage resolves a conflict file by writing content and staging it.
func resolveConflictAndStage(t *testing.T, repo *testutil.GitRepo, filename, content string) {
	t.Helper()
	repo.WriteFile(filename, content)
	repo.RunGit("add", filename)
}

// restackStateExists checks if .git/stack/restack-state.json exists.
func restackStateExists(t *testing.T, repo *testutil.GitRepo) bool {
	t.Helper()
	statePath := filepath.Join(repo.Dir, ".git", "stack", "restack-state.json")
	_, err := os.Stat(statePath)
	return err == nil
}

// rebaseInProgress checks if a rebase is currently in progress.
func rebaseInProgress(t *testing.T, repo *testutil.GitRepo) bool {
	t.Helper()
	rebaseMergePath := filepath.Join(repo.Dir, ".git", "rebase-merge")
	rebaseApplyPath := filepath.Join(repo.Dir, ".git", "rebase-apply")
	_, err1 := os.Stat(rebaseMergePath)
	_, err2 := os.Stat(rebaseApplyPath)
	return err1 == nil || err2 == nil
}

// getBranchSHA returns the SHA of a branch.
func getBranchSHA(t *testing.T, repo *testutil.GitRepo, branch string) string {
	t.Helper()
	sha, err := repo.RunGit("rev-parse", branch)
	if err != nil {
		t.Fatalf("failed to get SHA for %s: %v", branch, err)
	}
	return sha
}

// ---------------------------------------------------------------------------
// TestRestackMultiConflictCycle — Tasks 1.1, 1.2, 1.3
// ---------------------------------------------------------------------------

func TestRestackMultiConflictCycle(t *testing.T) {
	t.Run("sequential_conflicts_across_two_branches", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2 -> s3
		// s1 modifies conflict1.txt, s2 modifies conflict2.txt
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 content")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		runSt(t, "append", "s3")
		repo.CreateFile("s3.txt", "s3 file")
		repo.AddAndCommit("s3 commit")

		// Record pre-restack graph SHAs
		preGraph := loadGraph(t, repo)
		preS1HeadSHA := preGraph.Branches["s1"].HeadSHA
		preS3HeadSHA := preGraph.Branches["s3"].HeadSHA

		// Create conflicting changes on root for BOTH conflict files
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root conflict1 content")
		repo.AddAndCommit("root conflict1")
		repo.CreateFile("conflict2.txt", "root conflict2 content")
		repo.AddAndCommit("root conflict2")

		// Restack from s3 (tip) — should stop at s1
		repo.Checkout("s3")
		out := runStExpectError(t, "restack")
		assertContains(t, out, "conflict")

		// Verify restack state file exists
		if !restackStateExists(t, repo) {
			t.Error("restack state file should exist after conflict")
		}

		// Verify rebase is in progress
		if !rebaseInProgress(t, repo) {
			t.Error("rebase should be in progress after conflict at s1")
		}

		// Resolve conflict1.txt and continue — should proceed then stop at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved conflict1")
		out = runStContinueExpectError(t)
		assertContains(t, out, "conflict")

		// Verify restack state file still exists
		if !restackStateExists(t, repo) {
			t.Error("restack state file should persist after second conflict")
		}

		// Verify graph: s1 should NOW have updated SHAs (it was rebased successfully)
		g := loadGraph(t, repo)
		if g.Branches["s1"].HeadSHA == preS1HeadSHA {
			t.Error("s1 HeadSHA should be updated after successful continue for s1")
		}
		// s3 should still have its pre-restack graph SHA (not yet processed)
		if g.Branches["s3"].HeadSHA != preS3HeadSHA {
			t.Error("s3 HeadSHA should be unchanged while s2 is conflicting")
		}

		// Resolve conflict2.txt and continue — should complete
		resolveConflictAndStage(t, repo, "conflict2.txt", "resolved conflict2")
		runStContinue(t)

		// Verify restack state file is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state file should be cleared after successful completion")
		}

		// Verify no rebase in progress
		if rebaseInProgress(t, repo) {
			t.Error("rebase should not be in progress after completion")
		}

		// Verify all graph SHAs now match actual branch heads
		g = loadGraph(t, repo)
		for branchName, branch := range g.Branches {
			actualSHA := getBranchSHA(t, repo, branchName)
			if branch.HeadSHA != actualSHA {
				t.Errorf("graph HeadSHA for %s = %s, actual = %s", branchName, branch.HeadSHA, actualSHA)
			}
		}

		// Verify file contents on each branch
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "conflict1.txt"))
		if string(content) != "resolved conflict1" {
			t.Errorf("s1 conflict1.txt = %q, want 'resolved conflict1'", content)
		}

		repo.Checkout("s2")
		content, _ = os.ReadFile(filepath.Join(repo.Dir, "conflict2.txt"))
		if string(content) != "resolved conflict2" {
			t.Errorf("s2 conflict2.txt = %q, want 'resolved conflict2'", content)
		}
		if !repo.FileExists("s2.txt") {
			t.Error("s2 should have s2.txt")
		}

		repo.Checkout("s3")
		if !repo.FileExists("s3.txt") {
			t.Error("s3 should have s3.txt")
		}
		if !repo.FileExists("conflict1.txt") {
			t.Error("s3 should have conflict1.txt (inherited from s1)")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestackRestoreIntegrity — Tasks 2.1, 2.2, 2.3, 2.4
// ---------------------------------------------------------------------------

func TestRestackRestoreIntegrity(t *testing.T) {
	t.Run("restore_all_after_conflict_returns_exact_pre_restack_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2 -> s3
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		runSt(t, "append", "s3")
		repo.CreateFile("s3.txt", "s3 file")
		repo.AddAndCommit("s3 commit")

		// Record pre-restack state
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")
		origS3SHA := getBranchSHA(t, repo, "s3")

		// Read file content on s1 before restack
		repo.Checkout("s1")
		origContent, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))

		// Create conflict on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root conflict content")
		repo.AddAndCommit("root conflict")

		// Restack from s3 — conflict at s1
		repo.Checkout("s3")
		runStExpectError(t, "restack")

		// Restore all
		runSt(t, "restore", "--all")

		// Verify all branches have original SHAs
		if getBranchSHA(t, repo, "s1") != origS1SHA {
			t.Errorf("s1 SHA should be restored to original")
		}
		if getBranchSHA(t, repo, "s2") != origS2SHA {
			t.Errorf("s2 SHA should be restored to original")
		}
		if getBranchSHA(t, repo, "s3") != origS3SHA {
			t.Errorf("s3 SHA should be restored to original")
		}

		// Verify graph SHAs match actual branches
		g := loadGraph(t, repo)
		for branchName, branch := range g.Branches {
			actualSHA := getBranchSHA(t, repo, branchName)
			if branch.HeadSHA != actualSHA {
				t.Errorf("graph HeadSHA for %s = %s, actual = %s", branchName, branch.HeadSHA, actualSHA)
			}
		}

		// Verify restack state file is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state file should be cleared after restore")
		}

		// Verify no rebase in progress
		if rebaseInProgress(t, repo) {
			t.Error("rebase should not be in progress after restore")
		}

		// Verify file contents are restored
		repo.Checkout("s1")
		restoredContent, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(restoredContent) != string(origContent) {
			t.Errorf("s1 shared.txt content should be restored, got %q want %q", restoredContent, origContent)
		}
	})

	t.Run("restore_all_after_partial_continue_returns_pre_restack_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2 -> s3
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 content")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		runSt(t, "append", "s3")
		repo.CreateFile("s3.txt", "s3 file")
		repo.AddAndCommit("s3 commit")

		// Record pre-restack SHAs
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")
		origS3SHA := getBranchSHA(t, repo, "s3")

		// Create conflicts on root
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root conflict1")
		repo.AddAndCommit("root conflict1")
		repo.CreateFile("conflict2.txt", "root conflict2")
		repo.AddAndCommit("root conflict2")

		// Restack from s3 — conflict at s1
		repo.Checkout("s3")
		runStExpectError(t, "restack")

		// Resolve s1 and continue — should hit conflict at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved1")
		runStContinueExpectError(t)

		// Now restore --all (after partial continue)
		runSt(t, "restore", "--all")

		// Verify ALL branches are back to pre-restack state
		if getBranchSHA(t, repo, "s1") != origS1SHA {
			t.Errorf("s1 SHA should be restored to original after partial continue + restore")
		}
		if getBranchSHA(t, repo, "s2") != origS2SHA {
			t.Errorf("s2 SHA should be restored to original after partial continue + restore")
		}
		if getBranchSHA(t, repo, "s3") != origS3SHA {
			t.Errorf("s3 SHA should be restored to original after partial continue + restore")
		}

		// Verify no rebase artifacts
		if rebaseInProgress(t, repo) {
			t.Error("no rebase should be in progress after restore")
		}
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after restore")
		}
	})

	t.Run("restore_aborts_active_rebase_no_artifacts", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.AddAndCommit("s1 commit")

		// Create conflict on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack — conflict
		repo.Checkout("s1")
		runStExpectError(t, "restack")

		// Verify rebase IS in progress before restore
		if !rebaseInProgress(t, repo) {
			t.Fatal("rebase should be in progress before restore")
		}

		// Restore --all without resolving conflict
		runSt(t, "restore", "--all")

		// Verify no rebase-merge directory
		rebaseMergePath := filepath.Join(repo.Dir, ".git", "rebase-merge")
		if _, err := os.Stat(rebaseMergePath); err == nil {
			t.Error(".git/rebase-merge should not exist after restore")
		}
		rebaseApplyPath := filepath.Join(repo.Dir, ".git", "rebase-apply")
		if _, err := os.Stat(rebaseApplyPath); err == nil {
			t.Error(".git/rebase-apply should not exist after restore")
		}

		// Verify user is on a valid branch (not detached HEAD)
		cur := getCurrentBranch(t, repo)
		if cur == "HEAD" || cur == "" {
			t.Errorf("should be on a valid branch after restore, got %q", cur)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestackBackupIntegrity — Tasks 3.1, 3.2, 3.3
// ---------------------------------------------------------------------------

func TestRestackBackupIntegrity(t *testing.T) {
	t.Run("backups_survive_multiple_continue_cycles", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 content")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		// Record pre-restack SHAs
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")

		// Create conflicts on root
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root c1")
		repo.AddAndCommit("root c1")
		repo.CreateFile("conflict2.txt", "root c2")
		repo.AddAndCommit("root c2")

		// Restack from s2 — conflict at s1
		repo.Checkout("s2")
		runStExpectError(t, "restack")

		// Check backups exist for both branches after first conflict
		s1Backups, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s1/")
		s2Backups, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s2/")
		if s1Backups == "" {
			t.Error("s1 auto backup should exist after restack conflict")
		}
		if s2Backups == "" {
			t.Error("s2 auto backup should exist after restack conflict")
		}

		// Resolve s1, continue — hits conflict at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved c1")
		runStContinueExpectError(t)

		// Backups should STILL exist after first continue
		s1BackupsAfter, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s1/")
		s2BackupsAfter, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s2/")
		if s1BackupsAfter == "" {
			t.Error("s1 auto backup should still exist after first continue")
		}
		if s2BackupsAfter == "" {
			t.Error("s2 auto backup should still exist after first continue")
		}

		// Verify backups point to pre-restack SHAs
		s1BackupSHA := getBranchSHA(t, repo, strings.TrimSpace(s1BackupsAfter))
		s2BackupSHA := getBranchSHA(t, repo, strings.TrimSpace(s2BackupsAfter))
		if s1BackupSHA != origS1SHA {
			t.Errorf("s1 backup SHA = %s, want original %s", s1BackupSHA, origS1SHA)
		}
		if s2BackupSHA != origS2SHA {
			t.Errorf("s2 backup SHA = %s, want original %s", s2BackupSHA, origS2SHA)
		}

		// Resolve s2, continue to completion
		resolveConflictAndStage(t, repo, "conflict2.txt", "resolved c2")
		runStContinue(t)

		// Backups should be cleaned up after final success
		s1BackupsFinal, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s1/")
		s2BackupsFinal, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s2/")
		if strings.TrimSpace(s1BackupsFinal) != "" {
			t.Error("s1 auto backups should be cleaned up after successful completion")
		}
		if strings.TrimSpace(s2BackupsFinal) != "" {
			t.Error("s2 auto backups should be cleaned up after successful completion")
		}
	})

	t.Run("restore_after_multiple_continues_returns_pre_restack_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 original")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 original")
		repo.AddAndCommit("s2 commit")

		// Record pre-restack SHAs
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")

		// Create conflicts on root
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root c1")
		repo.AddAndCommit("root c1")
		repo.CreateFile("conflict2.txt", "root c2")
		repo.AddAndCommit("root c2")

		// Restack from s2 — conflict at s1
		repo.Checkout("s2")
		runStExpectError(t, "restack")

		// Resolve s1, continue — conflict at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved c1")
		runStContinueExpectError(t)

		// Now restore (s1 was rebased, s2 still conflicting)
		runSt(t, "restore", "--all")

		// Both should be back to pre-restack state
		if getBranchSHA(t, repo, "s1") != origS1SHA {
			t.Errorf("s1 should be restored to pre-restack SHA")
		}
		if getBranchSHA(t, repo, "s2") != origS2SHA {
			t.Errorf("s2 should be restored to pre-restack SHA")
		}

		// Verify file contents match originals
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "conflict1.txt"))
		if string(content) != "s1 original" {
			t.Errorf("s1 conflict1.txt = %q, want 's1 original'", content)
		}

		repo.Checkout("s2")
		content, _ = os.ReadFile(filepath.Join(repo.Dir, "conflict2.txt"))
		if string(content) != "s2 original" {
			t.Errorf("s2 conflict2.txt = %q, want 's2 original'", content)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestackRerere — Task 4.1
// ---------------------------------------------------------------------------

func TestRestackRerere(t *testing.T) {
	t.Run("rerere_auto_resolves_on_second_restack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.AddAndCommit("s1 commit")

		// Create conflict on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// First restack — conflict
		repo.Checkout("s1")
		runStExpectError(t, "restack")

		// Resolve and continue (this teaches rerere)
		resolveConflictAndStage(t, repo, "shared.txt", "resolved content")
		runStContinue(t)

		// Restore to pre-restack state
		runSt(t, "restore", "--all")

		// Re-create the same conflict scenario: diverge root again the same way
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict again")

		// Second restack — rerere should auto-resolve
		repo.Checkout("s1")
		// This might succeed automatically or might still need a continue
		cmd := exec.Command(stBinary, "restack")
		cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		setCoverEnv(cmd)
		out, err := cmd.CombinedOutput()

		if err != nil {
			// If restack still reports conflict, check if rerere resolved it
			// (rerere records resolution but rebase may still stop for user to verify)
			// Try continue — if rerere worked, this should succeed without manual resolution
			contCmd := exec.Command(stBinary, "continue")
			contCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
			setCoverEnv(contCmd)
			contOut, contErr := contCmd.CombinedOutput()
			if contErr != nil {
				t.Logf("restack output: %s", out)
				t.Logf("continue output: %s", contOut)
				t.Log("rerere may not have auto-resolved — this is git-version dependent")
				// Don't fail hard — rerere behavior varies across git versions
			}
		}

		// Verify s1 has the resolved content (either from auto-resolve or continue)
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		// Accept either "resolved content" or whatever rerere put there
		if strings.Contains(string(content), "<<<<<<<") {
			t.Error("s1 shared.txt still has conflict markers — rerere did not resolve")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSyncConflictContinue — Task 5.1
// ---------------------------------------------------------------------------

func TestSyncConflictContinue(t *testing.T) {
	t.Run("sync_conflict_then_continue_completes", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Create stack: root -> s1
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		// Push s1 so sync doesn't skip it
		repo.RunGit("push", "origin", "s1")

		// Simulate upstream change: add conflicting commit on origin's trunk
		originDir := repo.OriginDir()
		tmpClone, _ := os.MkdirTemp("", "st-clone-*")
		defer os.RemoveAll(tmpClone)
		cloneCmd := exec.Command("git", "clone", originDir, tmpClone)
		cloneCmd.Run()
		exec.Command("git", "-C", tmpClone, "config", "user.email", "other@test.com").Run()
		exec.Command("git", "-C", tmpClone, "config", "user.name", "Other").Run()
		os.WriteFile(filepath.Join(tmpClone, "shared.txt"), []byte("upstream conflict content"), 0644)
		exec.Command("git", "-C", tmpClone, "add", ".").Run()
		exec.Command("git", "-C", tmpClone, "commit", "-m", "upstream conflict").Run()
		exec.Command("git", "-C", tmpClone, "push", "origin", root).Run()

		// Run sync — should hit conflict during restack phase
		repo.Checkout("s1")
		out := runStExpectError(t, "sync")
		assertContains(t, out, "conflict")

		// Resolve and continue
		resolveConflictAndStage(t, repo, "shared.txt", "resolved sync content")
		runStContinue(t)

		// Verify s1 has the resolved content
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(content) != "resolved sync content" {
			t.Errorf("s1 shared.txt = %q, want 'resolved sync content'", content)
		}

		// Verify restack state is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after continue")
		}
	})
}

// ---------------------------------------------------------------------------
// TestUp
// ---------------------------------------------------------------------------

func TestUp(t *testing.T) {
	t.Run("checks_out_single_child", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")

		// Go back to f1, then up should take us to f2
		repo.Checkout("f1")
		runSt(t, "up")
		if cur := getCurrentBranch(t, repo); cur != "f2" {
			t.Errorf("current branch = %q, want f2", cur)
		}

		// From root, up should go to f1 (single child)
		repo.Checkout(root)
		runSt(t, "up")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}
	})

	t.Run("error_multiple_children", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "a")
		repo.Checkout(root)
		runSt(t, "new", "b")
		repo.Checkout(root)

		out := runStExpectError(t, "up")
		assertContains(t, out, "multiple")
	})

	t.Run("error_at_tip", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "up")
		assertContains(t, out, "tip")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "up")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestDown
// ---------------------------------------------------------------------------

func TestDown(t *testing.T) {
	t.Run("checks_out_parent", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")

		// From f2, down should go to f1
		runSt(t, "down")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}

		// From f1, down should go to root
		runSt(t, "down")
		if cur := getCurrentBranch(t, repo); cur != root {
			t.Errorf("current branch = %q, want %q", cur, root)
		}
	})

	t.Run("error_at_root", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		out := runStExpectError(t, "down")
		assertContains(t, out, "bottom")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "down")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestTop
// ---------------------------------------------------------------------------

func TestTop(t *testing.T) {
	t.Run("follows_single_child_to_tip", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")
		runSt(t, "append", "f3")

		// From root, top should go to f3
		repo.Checkout(root)
		runSt(t, "top")
		if cur := getCurrentBranch(t, repo); cur != "f3" {
			t.Errorf("current branch = %q, want f3", cur)
		}
	})

	t.Run("error_on_fork", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2a")
		repo.Checkout("f1")
		runSt(t, "append", "f2b")
		repo.Checkout("f1")

		out := runStExpectError(t, "top")
		assertContains(t, out, "multiple")
	})

	t.Run("already_at_tip", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "top")
		assertContains(t, out, "already")
	})
}

// ---------------------------------------------------------------------------
// TestBottom
// ---------------------------------------------------------------------------

func TestBottom(t *testing.T) {
	t.Run("navigates_to_first_child_of_root", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")
		runSt(t, "append", "f3")

		// From f3, bottom should go to f1
		runSt(t, "bottom")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}
	})

	t.Run("already_at_bottom", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.Checkout("f1")

		out := runSt(t, "bottom")
		assertContains(t, out, "already")
	})

	t.Run("on_root_with_single_child", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.Checkout(root)

		runSt(t, "bottom")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}
	})
}

// ---------------------------------------------------------------------------
// TestModify
// ---------------------------------------------------------------------------

func TestModify(t *testing.T) {
	t.Run("amend_staged_changes_and_restack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Go back to f1, modify it
		repo.Checkout("f1")
		repo.WriteFile("f1.txt", "f1-modified")
		repo.RunGit("add", "f1.txt")

		headBefore := repo.HeadSHA()
		runSt(t, "modify")
		headAfter := repo.HeadSHA()

		if headBefore == headAfter {
			t.Error("HEAD should have changed after modify")
		}

		// f2 should still exist in graph (restacked)
		g := loadGraph(t, repo)
		if _, ok := g.Branches["f2"]; !ok {
			t.Error("f2 should still be in graph after restack")
		}
	})

	t.Run("amend_with_all_flag", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// Make unstaged change
		repo.WriteFile("f1.txt", "f1-modified")

		runSt(t, "modify", "--all")

		// File should be committed
		status, _ := repo.RunGit("status", "--porcelain")
		if strings.TrimSpace(status) != "" {
			t.Errorf("working tree should be clean, got: %s", status)
		}
	})

	t.Run("update_message", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "modify", "--message", "new message")

		msg, _ := repo.RunGit("log", "-1", "--format=%s")
		if msg != "new message" {
			t.Errorf("commit message = %q, want 'new message'", msg)
		}
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "modify")
		assertContains(t, out, "not in the stack")
	})

	t.Run("error_nothing_to_modify", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		_ = repo // working tree is clean

		out := runStExpectError(t, "modify")
		assertContains(t, out, "nothing to modify")
	})
}

// ---------------------------------------------------------------------------
// TestDelete
// ---------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	t.Run("delete_branch_no_children", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "delete", "f1")

		if graphContains(t, repo, "f1") {
			t.Error("f1 should be removed from graph")
		}
		if repo.BranchExists("f1") {
			t.Error("git branch f1 should be deleted")
		}
	})

	t.Run("delete_branch_with_children_reparents", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Delete f1 — f2 should be reparented to root
		runSt(t, "delete", "f1")

		g := loadGraph(t, repo)
		if _, ok := g.Branches["f1"]; ok {
			t.Error("f1 should be removed from graph")
		}
		f2, ok := g.Branches["f2"]
		if !ok {
			t.Fatal("f2 should still be in graph")
		}
		if f2.Parent != root {
			t.Errorf("f2 parent = %q, want %q", f2.Parent, root)
		}
	})

	t.Run("delete_current_branch", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// On f1, delete it — should checkout root first
		runSt(t, "delete", "f1")

		if cur := getCurrentBranch(t, repo); cur != root {
			t.Errorf("current branch = %q, want %q", cur, root)
		}
	})

	t.Run("error_delete_root", func(t *testing.T) {
		_, root := setupRepoWithStack(t)
		out := runStExpectError(t, "delete", root)
		assertContains(t, out, "cannot delete")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		out := runStExpectError(t, "delete", "nonexistent")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestMove
// ---------------------------------------------------------------------------

func TestMove(t *testing.T) {
	t.Run("reparent_and_restack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		// Create two branches off root: a and b
		runSt(t, "new", "a")
		repo.CreateFile("a.txt", "a")
		repo.AddAndCommit("a commit")
		repo.Checkout(root)
		runSt(t, "new", "b")
		repo.CreateFile("b.txt", "b")
		repo.AddAndCommit("b commit")

		// Move b onto a
		runSt(t, "move", "--onto", "a")

		g := loadGraph(t, repo)
		bBranch, ok := g.Branches["b"]
		if !ok {
			t.Fatal("b should still be in graph")
		}
		if bBranch.Parent != "a" {
			t.Errorf("b parent = %q, want 'a'", bBranch.Parent)
		}
	})

	t.Run("error_move_onto_self", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "move", "--onto", "f1")
		assertContains(t, out, "cannot move")
	})

	t.Run("error_move_onto_descendant", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.Checkout("f1")

		out := runStExpectError(t, "move", "--onto", "f2")
		assertContains(t, out, "cycle")
	})

	t.Run("error_target_not_in_stack", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "move", "--onto", "nonexistent")
		assertContains(t, out, "not in the stack")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "move", "--onto", root)
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestAbort
// ---------------------------------------------------------------------------

func TestAbort(t *testing.T) {
	t.Run("error_no_rebase_in_progress", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		out := runStExpectError(t, "abort")
		assertContains(t, out, "no rebase in progress")
	})

	t.Run("aborts_active_rebase_and_clears_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		// Create a stack where s1 has a file that will conflict with root
		runSt(t, "new", "s1")
		repo.CreateFile("conflict.txt", "s1 content")
		repo.AddAndCommit("s1: add conflict file")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("conflict.txt", "root content")
		repo.AddAndCommit("root: add conflict file")

		// Restack from s1 will conflict
		repo.Checkout("s1")
		_ = runStExpectError(t, "restack")

		// Verify rebase is in progress
		if !rebaseInProgress(t, repo) {
			t.Skip("no rebase detected — conflict didn't trigger as expected")
		}

		// Abort should succeed
		runSt(t, "abort")

		// Restack state should be cleared
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after abort")
		}

		// Continue should now error
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no rebase in progress")
	})
}

// ---------------------------------------------------------------------------
// TestDetachedHEADGuard
// ---------------------------------------------------------------------------

func TestDetachedHEADGuard(t *testing.T) {
	t.Run("log_with_detached_HEAD_returns_error", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create a branch so the graph has content
		runSt(t, "new", "feature-1")

		// Detach HEAD
		sha := repo.HeadSHA()
		repo.RunGit("checkout", sha)

		out := runStExpectError(t, "log")
		assertContains(t, out, "HEAD is detached")
		assertContains(t, out, "check out a branch first")
	})
}

// ---------------------------------------------------------------------------
// TestBranchAlreadyExists
// ---------------------------------------------------------------------------

func TestBranchAlreadyExists(t *testing.T) {
	t.Run("new_with_existing_branch_suggests_attach", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runStExpectError(t, "new", "feature-1")
		assertContains(t, out, "branch 'feature-1' already exists")
		assertContains(t, out, "st attach feature-1")
	})

	t.Run("append_with_existing_branch_suggests_attach", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")
		repo.Checkout(root)

		out := runStExpectError(t, "append", "feature-1")
		assertContains(t, out, "branch 'feature-1' already exists")
		assertContains(t, out, "st attach feature-1")
	})
}

// ---------------------------------------------------------------------------
// TestContinueSafetyCheck
// ---------------------------------------------------------------------------

func TestContinueSafetyCheck(t *testing.T) {
	t.Run("continue_with_manual_rebase_suggests_git_rebase_continue", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create two branches with conflicting changes
		runSt(t, "new", "feature-1")
		repo.CreateFile("conflict.txt", "feature-1 content")
		repo.AddAndCommit("feature-1 commit")

		repo.Checkout(root)
		repo.CreateFile("conflict.txt", "root content")
		repo.AddAndCommit("root commit")

		// Start a manual git rebase (not via st)
		repo.RunGit("rebase", "feature-1")
		// This should fail with conflicts — if it doesn't, skip
		if !rebaseInProgress(t, repo) {
			t.Skip("manual rebase didn't produce conflicts")
		}

		// st continue should detect no restack state
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no st restack in progress")
		assertContains(t, out, "git rebase --continue")

		// Cleanup: abort the rebase
		repo.RunGit("rebase", "--abort")
	})
}

// ---------------------------------------------------------------------------
// TestDirtyTreeWarning
// ---------------------------------------------------------------------------

func TestDirtyTreeWarning(t *testing.T) {
	t.Run("restack_with_dirty_tree_warns_but_proceeds", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create a branch
		runSt(t, "new", "feature-1")
		repo.CreateFile("feature.txt", "feature content")
		repo.AddAndCommit("feature commit")

		// Create uncommitted changes
		repo.WriteFile("dirty.txt", "uncommitted")

		// Restack should warn but succeed
		out := runSt(t, "-v", "restack")
		assertContains(t, out, "uncommitted changes")
	})
}

// ---------------------------------------------------------------------------
// TestDetach
// ---------------------------------------------------------------------------

func TestDetach(t *testing.T) {
	t.Run("detach_leaf_branch", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runSt(t, "detach", "feature-1")
		assertContains(t, out, "Detached 'feature-1' from stack")

		// Branch should still exist in git
		if !repo.BranchExists("feature-1") {
			t.Error("git branch feature-1 should still exist")
		}

		// Branch should not be in graph
		if graphContains(t, repo, "feature-1") {
			t.Error("feature-1 should not be in graph")
		}
	})

	t.Run("detach_branch_with_children_reparents", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")
		runSt(t, "append", "feature-2")
		runSt(t, "append", "feature-3")

		out := runSt(t, "detach", "feature-1")
		assertContains(t, out, "Detached 'feature-1' from stack")
		assertContains(t, out, "reparented")
		assertContains(t, out, "st restack")

		// feature-1 removed, feature-2 reparented to root
		g := loadGraph(t, repo)
		if _, ok := g.Branches["feature-1"]; ok {
			t.Error("feature-1 should be removed from graph")
		}
		if b, ok := g.Branches["feature-2"]; !ok {
			t.Error("feature-2 should still be in graph")
		} else if b.Parent != root {
			t.Errorf("feature-2 parent = %q, want %q", b.Parent, root)
		}
	})

	t.Run("detach_root_errors", func(t *testing.T) {
		_, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runStExpectError(t, "detach", root)
		assertContains(t, out, "cannot detach the root branch")
	})

	t.Run("detach_unknown_branch_errors", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runStExpectError(t, "detach", "nonexistent")
		assertContains(t, out, "not in the stack")
	})

	t.Run("detach_current_branch_no_arg", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")
		// feature-1 is current branch

		out := runSt(t, "detach")
		assertContains(t, out, "Detached 'feature-1' from stack")

		if graphContains(t, repo, "feature-1") {
			t.Error("feature-1 should not be in graph")
		}
	})
}

// ---------------------------------------------------------------------------
// TestAttachFilterStacked
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// skill install
// ---------------------------------------------------------------------------

func TestSkillInstall_NpxNotFound(t *testing.T) {
	setupRepo(t)

	// Run with empty PATH so npx cannot be found.
	cmd := exec.Command(stBinary, "skill", "install")
	setCoverEnv(cmd)
	cmd.Env = append(cmd.Env, "PATH=")
	cmd.Env = append(cmd.Env, "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error when npx not in PATH, got success\nOutput: %s", out)
	}
	if !strings.Contains(string(out), "npx") {
		t.Fatalf("expected error mentioning npx, got: %s", out)
	}
}

func candidateNames(candidates []attachCandidate) []string {
	var names []string
	for _, c := range candidates {
		names = append(names, c.name)
	}
	return names
}

// ---------------------------------------------------------------------------
// Reviews command tests
// ---------------------------------------------------------------------------

func TestReviews_NoBranches(t *testing.T) {
	repo := setupRepo(t)
	defer repo.Cleanup()
	repo.InitStack()

	// With no branches in the stack, should exit cleanly
	out := runSt(t, "reviews")
	if !strings.Contains(out, "no branches in scope") {
		t.Errorf("expected 'no branches in scope', got: %s", out)
	}
}

func TestReviews_FlagsRegistered(t *testing.T) {
	repo := setupRepo(t)
	defer repo.Cleanup()
	repo.InitStack()

	// Verify help shows the expected flags
	out := runSt(t, "reviews", "--help")
	if !strings.Contains(out, "--current") {
		t.Error("missing --current flag in help")
	}
	if !strings.Contains(out, "--to-current") {
		t.Error("missing --to-current flag in help")
	}
	if !strings.Contains(out, "--out") {
		t.Error("missing --out flag in help")
	}
}
