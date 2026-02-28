package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

	t.Run("shared_branch_local_not_ahead", func(t *testing.T) {
		tmpDir, gitRunner := initRepoWithCommit(t)

		// Create branch with extra commit as "remote" state
		exec.Command("git", "-C", tmpDir, "checkout", "-b", "shared").Run()
		os.WriteFile(filepath.Join(tmpDir, "extra.txt"), []byte("extra"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "extra").Run()
		remoteSHA, _ := gitRunner.GetBranchSHA("shared")
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		// Local has the base SHA (behind remote)
		baseSHA, _ := gitRunner.GetBranchSHA("main")

		local := graph.NewGraph("main")
		local.AddBranch("shared", "main", "abc", baseSHA)
		remote := graph.NewGraph("main")
		remote.AddBranch("shared", "main", "abc", remoteSHA)

		result := ReconcileGraphs(local, remote, gitRunner)
		b, ok := result.Branches["shared"]
		if !ok {
			t.Fatal("shared branch should be present")
		}
		// Local is not ahead of remote, so remote HeadSHA should be kept
		if b.HeadSHA != remoteSHA {
			t.Errorf("expected remote HeadSHA %s (local not ahead), got %s", remoteSHA, b.HeadSHA)
		}
	})
}
