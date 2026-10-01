package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpave3/staccato/internal/testutil"
	stcontext "github.com/cpave3/staccato/pkg/context"
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
