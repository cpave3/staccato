package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

func initRepoWithCommit(t *testing.T) (string, *git.Runner) {
	t.Helper()
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()
	return tmpDir, git.NewRunner(tmpDir)
}

func TestReconcileGraphs(t *testing.T) {
	t.Run("remote_only_branch_local_git_exists", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		// Create a branch in git
		exec.Command("git", "-C", tmpDir, "branch", "remote-feat").Run()

		local := graph.NewGraph("main")
		remote := graph.NewGraph("main")
		sha, _ := gitRunner.GetBranchSHA("remote-feat")
		remote.AddBranch("remote-feat", "main", "abc", sha)

		result := ReconcileGraphs(local, remote, gitRunner)
		if _, ok := result.Branches["remote-feat"]; !ok {
			t.Error("remote-only branch with local git should be included")
		}
	})

	t.Run("remote_only_branch_local_git_missing", func(t *testing.T) {
		_, gitRunner := initRepoWithCommit(t)

		local := graph.NewGraph("main")
		remote := graph.NewGraph("main")
		remote.AddBranch("no-local", "main", "abc", "def")

		result := ReconcileGraphs(local, remote, gitRunner)
		if _, ok := result.Branches["no-local"]; ok {
			t.Error("remote-only branch without local git should be excluded")
		}
	})

	t.Run("local_only_branch_git_exists", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		exec.Command("git", "-C", tmpDir, "branch", "local-feat").Run()

		local := graph.NewGraph("main")
		sha, _ := gitRunner.GetBranchSHA("local-feat")
		local.AddBranch("local-feat", "main", "abc", sha)
		remote := graph.NewGraph("main")

		result := ReconcileGraphs(local, remote, gitRunner)
		if _, ok := result.Branches["local-feat"]; !ok {
			t.Error("local-only branch with git should be included")
		}
	})

	t.Run("local_only_branch_git_missing", func(t *testing.T) {
		_, gitRunner := initRepoWithCommit(t)

		local := graph.NewGraph("main")
		local.AddBranch("ghost", "main", "abc", "def")
		remote := graph.NewGraph("main")

		result := ReconcileGraphs(local, remote, gitRunner)
		if _, ok := result.Branches["ghost"]; ok {
			t.Error("local-only branch without git should be excluded")
		}
	})

	t.Run("shared_branch_same_sha", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		exec.Command("git", "-C", tmpDir, "branch", "shared").Run()
		sha, _ := gitRunner.GetBranchSHA("shared")

		local := graph.NewGraph("main")
		local.AddBranch("shared", "main", "abc", sha)
		remote := graph.NewGraph("main")
		remote.AddBranch("shared", "main", "abc", sha)

		result := ReconcileGraphs(local, remote, gitRunner)
		b, ok := result.Branches["shared"]
		if !ok {
			t.Fatal("shared branch should be present")
		}
		if b.HeadSHA != sha {
			t.Errorf("expected HeadSHA %s, got %s", sha, b.HeadSHA)
		}
	})

	t.Run("shared_branch_local_ahead", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		// Create branch and add a commit
		exec.Command("git", "-C", tmpDir, "checkout", "-b", "shared").Run()
		baseSHA, _ := gitRunner.GetBranchSHA("main")
		os.WriteFile(filepath.Join(tmpDir, "extra.txt"), []byte("extra"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "extra").Run()
		localSHA, _ := gitRunner.GetBranchSHA("shared")
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		local := graph.NewGraph("main")
		local.AddBranch("shared", "main", "abc", localSHA)
		remote := graph.NewGraph("main")
		remote.AddBranch("shared", "main", "abc", baseSHA)

		result := ReconcileGraphs(local, remote, gitRunner)
		b, ok := result.Branches["shared"]
		if !ok {
			t.Fatal("shared branch should be present")
		}
		if b.HeadSHA != localSHA {
			t.Errorf("expected local HeadSHA %s (local ahead), got %s", localSHA, b.HeadSHA)
		}
	})

	t.Run("higher_version_wins", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		exec.Command("git", "-C", tmpDir, "checkout", "-b", "shared").Run()
		os.WriteFile(filepath.Join(tmpDir, "extra.txt"), []byte("extra"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "extra").Run()
		remoteSHA, _ := gitRunner.GetBranchSHA("shared")
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		baseSHA, _ := gitRunner.GetBranchSHA("main")

		// Local version 1, remote version 2 — remote wins as base
		local := graph.NewGraph("main")
		local.Version = 1
		local.AddBranch("shared", "main", "abc", baseSHA)
		remote := graph.NewGraph("main")
		remote.Version = 2
		remote.AddBranch("shared", "main", "abc", remoteSHA)

		result := ReconcileGraphs(local, remote, gitRunner)
		b, ok := result.Branches["shared"]
		if !ok {
			t.Fatal("shared branch should be present")
		}
		// Remote has higher version, so its metadata wins
		if b.HeadSHA != remoteSHA {
			t.Errorf("expected remote HeadSHA %s (higher version), got %s", remoteSHA, b.HeadSHA)
		}
	})

	t.Run("shared_branch_local_parent_preferred", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		// Create a branch in git
		exec.Command("git", "-C", tmpDir, "checkout", "-b", "feat").Run()
		os.WriteFile(filepath.Join(tmpDir, "feat.txt"), []byte("feat"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "feat").Run()
		sha, _ := gitRunner.GetBranchSHA("feat")
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		// Local: feat reparented to main (after merge removal of old parent)
		local := graph.NewGraph("main")
		local.AddBranch("feat", "main", "local-base", sha)

		// Remote: stale graph still has feat with old parent
		remote := graph.NewGraph("main")
		remote.AddBranch("feat", "old-parent", "remote-base", sha)

		result := ReconcileGraphs(local, remote, gitRunner)
		b, ok := result.Branches["feat"]
		if !ok {
			t.Fatal("feat should be present")
		}
		// Local parent should win (reflects reparenting from merge removal)
		if b.Parent != "main" {
			t.Errorf("expected local parent %q, got %q", "main", b.Parent)
		}
		if b.BaseSHA != "local-base" {
			t.Errorf("expected local BaseSHA %q, got %q", "local-base", b.BaseSHA)
		}
	})
}

func initRepoWithRemote(t *testing.T) (string, string, *git.Runner) {
	t.Helper()
	// Create the "remote" bare repo
	bareDir := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run(bareDir, "init", "--bare", "-b", "main")

	// Clone it to get a working repo with origin set up
	tmpDir := t.TempDir()
	run(tmpDir, "clone", bareDir, "repo")
	repoDir := filepath.Join(tmpDir, "repo")
	run(repoDir, "config", "user.email", "test@test.com")
	run(repoDir, "config", "user.name", "Test User")

	// Initial commit
	os.WriteFile(filepath.Join(repoDir, "init.txt"), []byte("init"), 0644)
	run(repoDir, "add", ".")
	run(repoDir, "commit", "-m", "initial")
	run(repoDir, "push", "origin", "main")

	return repoDir, bareDir, git.NewRunner(repoDir)
}

func TestSyncRestacksOnlyCurrentLineage(t *testing.T) {
	repoDir, bareDir, gitRunner := initRepoWithRemote(t)
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Create lineage A: main -> feat-a1 -> feat-a2
	run(repoDir, "checkout", "-b", "feat-a1")
	os.WriteFile(filepath.Join(repoDir, "a1.txt"), []byte("a1"), 0644)
	run(repoDir, "add", ".")
	run(repoDir, "commit", "-m", "a1")
	a1SHA := getSHA(t, repoDir, "feat-a1")

	run(repoDir, "checkout", "-b", "feat-a2")
	os.WriteFile(filepath.Join(repoDir, "a2.txt"), []byte("a2"), 0644)
	run(repoDir, "add", ".")
	run(repoDir, "commit", "-m", "a2")

	// Create lineage B: main -> feat-b1
	run(repoDir, "checkout", "main")
	run(repoDir, "checkout", "-b", "feat-b1")
	os.WriteFile(filepath.Join(repoDir, "b1.txt"), []byte("b1"), 0644)
	run(repoDir, "add", ".")
	run(repoDir, "commit", "-m", "b1")
	b1SHABefore := getSHA(t, repoDir, "feat-b1")

	// Push all branches
	run(repoDir, "push", "origin", "feat-a1")
	run(repoDir, "push", "origin", "feat-a2")
	run(repoDir, "push", "origin", "feat-b1")

	// Build the graph
	mainSHA := getSHA(t, repoDir, "main")
	g := graph.NewGraph("main")
	g.AddBranch("feat-a1", "main", mainSHA, a1SHA)
	a2SHA := getSHA(t, repoDir, "feat-a2")
	g.AddBranch("feat-a2", "feat-a1", a1SHA, a2SHA)
	g.AddBranch("feat-b1", "main", mainSHA, b1SHABefore)

	// Advance trunk on origin (simulate someone merging a PR)
	// Clone fresh to make the commit on origin
	advanceDir := t.TempDir()
	run(advanceDir, "clone", bareDir, "adv")
	advDir := filepath.Join(advanceDir, "adv")
	run(advDir, "config", "user.email", "test@test.com")
	run(advDir, "config", "user.name", "Test User")
	os.WriteFile(filepath.Join(advDir, "trunk-advance.txt"), []byte("new"), 0644)
	run(advDir, "add", ".")
	run(advDir, "commit", "-m", "trunk advance")
	run(advDir, "push", "origin", "main")

	// Checkout lineage A branch and run sync
	run(repoDir, "checkout", "feat-a2")

	sc := stcontext.NewContext(g, gitRunner, repoDir)
	result, err := Run(sc, Options{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// feat-b1 should NOT have been restacked (its SHA should be unchanged)
	b1SHAAfter := getSHA(t, repoDir, "feat-b1")
	if b1SHAAfter != b1SHABefore {
		t.Errorf("feat-b1 was restacked but shouldn't have been (outside current lineage)\nbefore: %s\nafter:  %s", b1SHABefore, b1SHAAfter)
	}

	// feat-a1 and feat-a2 SHOULD have been restacked (they're in the current lineage)
	if result.RestackedCount == 0 {
		t.Error("expected some branches to be restacked in the current lineage")
	}
}

func getSHA(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get SHA for %s: %v", ref, err)
	}
	return string(out[:len(out)-1]) // trim newline
}
