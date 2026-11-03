package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/graph"
)

// rejectPushesTo installs a pre-receive hook in bareDir that refuses any push
// to ref, as a credential proxy holding a push for approval would.
func rejectPushesTo(t *testing.T, bareDir, ref string) {
	t.Helper()
	script := "#!/bin/sh\nwhile read old new ref; do\n  if [ \"$ref\" = \"" + ref + "\" ]; then\n    echo \"held for approval\" >&2\n    exit 1\n  fi\ndone\n"
	if err := os.WriteFile(filepath.Join(bareDir, "hooks", "pre-receive"), []byte(script), 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
}

// stackOfTwo makes main -> a -> b in repoDir, with b checked out, and returns
// its graph.
func stackOfTwo(t *testing.T, repoDir string) *graph.Graph {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	mainSHA := getSHA(t, repoDir, "main")
	run("checkout", "-b", "a")
	os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("a"), 0644)
	run("add", ".")
	run("commit", "-m", "a")
	run("checkout", "-b", "b")
	os.WriteFile(filepath.Join(repoDir, "b.txt"), []byte("b"), 0644)
	run("add", ".")
	run("commit", "-m", "b")

	g := graph.NewGraph("main")
	g.AddBranch("a", "main", mainSHA, getSHA(t, repoDir, "a"))
	g.AddBranch("b", "a", getSHA(t, repoDir, "a"), getSHA(t, repoDir, "b"))
	return g
}

func TestSyncRecordsARefusedPushAndPushesTheRest(t *testing.T) {
	repoDir, bareDir, gitRunner := initRepoWithRemote(t)
	g := stackOfTwo(t, repoDir)
	rejectPushesTo(t, bareDir, "refs/heads/a")

	result, err := Run(stcontext.NewContext(g, gitRunner, repoDir), Options{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(result.PushedBranches) != 1 || result.PushedBranches[0] != "b" {
		t.Errorf("PushedBranches = %v, want [b]", result.PushedBranches)
	}
	if len(result.FailedPushes) != 1 {
		t.Fatalf("FailedPushes = %+v, want one for a", result.FailedPushes)
	}
	failed := result.FailedPushes[0]
	if failed.Branch != "a" || !strings.HasPrefix(failed.Reason, "! [remote rejected] a -> a") {
		t.Errorf("FailedPushes[0] = %+v, want a with git's [remote rejected] line", failed)
	}
}

func TestSyncRecordsARefusedSharedGraphPushUnderTheRef(t *testing.T) {
	repoDir, bareDir, gitRunner := initRepoWithRemote(t)
	g := stackOfTwo(t, repoDir)
	ref := graph.UserGraphRef("test@test.com")
	if err := gitRunner.WriteBlobRef(ref, []byte(`{"version":1,"root":"main","branches":{}}`)); err != nil {
		t.Fatalf("write shared ref: %v", err)
	}
	rejectPushesTo(t, bareDir, ref)

	sc := stcontext.NewContext(g, gitRunner, repoDir)
	if !sc.IsShared() {
		t.Fatal("context is not in shared mode")
	}
	result, err := Run(sc, Options{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(result.PushedBranches) != 2 {
		t.Errorf("PushedBranches = %v, want both branches", result.PushedBranches)
	}
	if len(result.FailedPushes) != 1 || result.FailedPushes[0].Branch != ref {
		t.Fatalf("FailedPushes = %+v, want one under %s", result.FailedPushes, ref)
	}
	if !strings.Contains(result.FailedPushes[0].Reason, "remote rejected") {
		t.Errorf("reason = %q, want git's [remote rejected] line", result.FailedPushes[0].Reason)
	}
}
