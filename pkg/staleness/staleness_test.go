package staleness

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

func initRepo(t *testing.T) (string, *git.Runner) {
	t.Helper()
	tmpDir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test User")
	os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("init"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")
	return tmpDir, git.NewRunner(tmpDir)
}

func addBareRemote(t *testing.T, repoDir string) string {
	t.Helper()
	bareDir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, out)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("remote", "add", "origin", bareDir)
	run("push", "-u", "origin", "main")
	return bareDir
}

func TestCheck_TrunkBehindRemote(t *testing.T) {
	repoDir, gitRunner := initRepo(t)
	bareDir := addBareRemote(t, repoDir)

	g := graph.NewGraph("main")

	// Advance origin/main by committing directly into bare repo's ref
	// First create a commit in local, push, then reset local back
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create a new commit, push to origin, then reset local main back
	os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("new"), 0644)
	run("add", ".")
	run("commit", "-m", "advance")
	run("push", "origin", "main")
	// Reset local main back one commit so local is behind
	run("reset", "--hard", "HEAD~1")
	// Fetch to update tracking refs
	run("fetch", "origin")

	_ = bareDir

	report := Check(g, gitRunner)
	if !report.IsStale() {
		t.Fatal("expected stale report when trunk is behind remote")
	}

	found := false
	for _, s := range report.Signals {
		if s.Branch == "main" {
			found = true
		}
	}
	if !found {
		t.Error("expected signal for trunk branch 'main'")
	}
}

func TestCheck_BranchDeletedOnRemote(t *testing.T) {
	repoDir, gitRunner := initRepo(t)
	bareDir := addBareRemote(t, repoDir)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	bareRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = bareDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create and push a feature branch
	run("checkout", "-b", "feature-1")
	os.WriteFile(filepath.Join(repoDir, "feat.txt"), []byte("feat"), 0644)
	run("add", ".")
	run("commit", "-m", "feature commit")
	run("push", "origin", "feature-1")
	run("checkout", "main")

	// Add feature-1 to graph
	g := graph.NewGraph("main")
	g.AddBranch("feature-1", "main", "abc", "def")

	// Delete feature-1 on remote, then fetch --prune
	bareRun("branch", "-D", "feature-1")
	run("fetch", "origin", "--prune")

	report := Check(g, gitRunner)
	if !report.IsStale() {
		t.Fatal("expected stale report when branch deleted on remote")
	}

	found := false
	for _, s := range report.Signals {
		if s.Branch == "feature-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected signal for deleted branch 'feature-1'")
	}
}

func TestCheck_CleanState(t *testing.T) {
	repoDir, gitRunner := initRepo(t)
	addBareRemote(t, repoDir)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create and push feature branch
	run("checkout", "-b", "feature-1")
	os.WriteFile(filepath.Join(repoDir, "feat.txt"), []byte("feat"), 0644)
	run("add", ".")
	run("commit", "-m", "feature")
	run("push", "origin", "feature-1")
	run("checkout", "main")

	g := graph.NewGraph("main")
	g.AddBranch("feature-1", "main", "abc", "def")

	// Fetch to make sure tracking refs are current
	run("fetch", "origin")

	report := Check(g, gitRunner)
	if report.IsStale() {
		t.Errorf("expected clean state, got signals: %+v", report.Signals)
	}
}

func TestCheck_NoRemote(t *testing.T) {
	_, gitRunner := initRepo(t)

	g := graph.NewGraph("main")
	g.AddBranch("feature-1", "main", "abc", "def")

	// No remote configured — should return empty report
	report := Check(g, gitRunner)
	if report.IsStale() {
		t.Error("expected no staleness when no remote is configured")
	}
}
