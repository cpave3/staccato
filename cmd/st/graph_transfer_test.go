package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runStIn runs st in dir with stdin, returning its standard output, its
// standard error and whether it exited 0.
func runStIn(t *testing.T, dir, stdin string, args ...string) (string, string, bool) {
	t.Helper()
	cmd := exec.Command(stBinary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	setCoverEnv(cmd)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("st %v: %v", args, err)
		}
	}
	return stdout.String(), stderr.String(), err == nil
}

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// twoClones gives a worker clone with a stack a → b pushed to a shared
// origin, and a second, fresh clone of that origin. Both are on root.
func twoClones(t *testing.T) (worker, mine, root string) {
	t.Helper()
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatal(err)
	}
	worker = repo.Dir
	gitAt(t, worker, "push", "-q", "-u", "origin", root)
	runSt(t, "append", "a")
	repo.CreateFile("a.txt", "a")
	repo.AddAndCommit("a")
	runSt(t, "append", "b")
	repo.CreateFile("b.txt", "b")
	repo.AddAndCommit("b")
	gitAt(t, worker, "push", "-q", "-u", "origin", "a", "b")
	gitAt(t, worker, "checkout", "-q", root)

	mine = filepath.Join(t.TempDir(), "mine")
	gitAt(t, t.TempDir(), "clone", "-q", "-b", root, repo.OriginDir(), mine)
	gitAt(t, mine, "config", "user.email", "me@example.com")
	gitAt(t, mine, "config", "user.name", "Me")
	return worker, mine, root
}

type importReport struct {
	Root     string `json:"root"`
	Branches []struct {
		Name    string `json:"name"`
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	} `json:"branches"`
}

func TestGraphExportThenImportIntoAnotherClone(t *testing.T) {
	worker, mine, root := twoClones(t)
	gitAt(t, worker, "checkout", "-q", "b")

	doc, stderr, ok := runStIn(t, worker, "", "graph", "export")
	if !ok {
		t.Fatalf("export failed: %s", stderr)
	}
	assertContains(t, doc, `"kind": "staccato.graph"`)

	file := filepath.Join(t.TempDir(), "stack.json")
	os.WriteFile(file, []byte(doc), 0o644)
	out, stderr, ok := runStIn(t, mine, "", "graph", "import", file)
	if !ok {
		t.Fatalf("import failed: %s\n%s", out, stderr)
	}

	assertContains(t, out, "a")
	assertContains(t, out, "added")
	g := loadGraphInDir(t, mine)
	if g.Root != root || g.Branches["b"] == nil || g.Branches["b"].Parent != "a" {
		t.Errorf("graph = %+v", g)
	}
	if gitAt(t, mine, "rev-parse", "b") != gitAt(t, worker, "rev-parse", "b") {
		t.Error("b was not created at the worker's head")
	}
	// The imported stack is one st can work on.
	gitAt(t, mine, "checkout", "-q", "b")
	log, _, ok := runStIn(t, mine, "", "log")
	if !ok || !strings.Contains(log, "a") || !strings.Contains(log, "b") {
		t.Errorf("st log after import: %s", log)
	}
}

func TestGraphExportToAFile(t *testing.T) {
	worker, _, _ := twoClones(t)
	file := filepath.Join(t.TempDir(), "out.json")

	stdout, stderr, ok := runStIn(t, worker, "", "graph", "export", "--branch", "a", "--output", file)

	if !ok || stdout != "" {
		t.Fatalf("export --output: ok=%v stdout=%q stderr=%s", ok, stdout, stderr)
	}
	data, _ := os.ReadFile(file)
	assertContains(t, string(data), `"name": "b"`)
}

func TestGraphExportRefusesAnUnpushedBranch(t *testing.T) {
	worker, _, _ := twoClones(t)
	gitAt(t, worker, "checkout", "-q", "b")
	os.WriteFile(filepath.Join(worker, "more.txt"), []byte("x"), 0o644)
	gitAt(t, worker, "add", "more.txt")
	gitAt(t, worker, "commit", "-q", "-m", "more")

	stdout, stderr, ok := runStIn(t, worker, "", "graph", "export")

	if ok || stdout != "" {
		t.Fatalf("exported an unpushed branch: %s", stdout)
	}
	assertContains(t, stderr, "b has commits origin/b lacks")

	stdout, _, ok = runStIn(t, worker, "", "graph", "export", "--allow-unpushed")
	if !ok || !strings.Contains(stdout, gitAt(t, worker, "rev-parse", "b")) {
		t.Errorf("--allow-unpushed did not export the local tip: %s", stdout)
	}
}

func TestGraphImportFromStandardInputAsJSON(t *testing.T) {
	worker, mine, root := twoClones(t)
	doc, _, _ := runStIn(t, worker, "", "graph", "export", "--all")

	out, stderr, ok := runStIn(t, mine, doc, "graph", "import", "--json", "-")

	if !ok {
		t.Fatalf("import failed: %s", stderr)
	}
	var report importReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if report.Root != root || len(report.Branches) != 2 || report.Branches[0].Name != "a" || report.Branches[0].Outcome != "added" {
		t.Errorf("report = %+v", report)
	}
}

func TestGraphImportDryRunChangesNothing(t *testing.T) {
	worker, mine, _ := twoClones(t)
	doc, _, _ := runStIn(t, worker, "", "graph", "export", "--all")

	out, stderr, ok := runStIn(t, mine, doc, "graph", "import", "--dry-run", "-")

	if !ok {
		t.Fatalf("dry run failed: %s", stderr)
	}
	assertContains(t, out, "added")
	assertContains(t, out, "nothing was changed")
	if branches := gitAt(t, mine, "branch", "--list", "a", "b"); branches != "" {
		t.Errorf("dry run created branches: %s", branches)
	}
	if g := loadGraphInDir(t, mine); len(g.Branches) != 0 {
		t.Errorf("dry run saved the graph: %+v", g.Branches)
	}
}

func TestGraphImportNoFetchNeedsTheCommits(t *testing.T) {
	worker, mine, _ := twoClones(t)
	doc, _, _ := runStIn(t, worker, "", "graph", "export", "--all")
	// The clone already has origin/a, but not origin/b's newest commit.
	gitAt(t, worker, "checkout", "-q", "b")
	os.WriteFile(filepath.Join(worker, "more.txt"), []byte("x"), 0o644)
	gitAt(t, worker, "add", "more.txt")
	gitAt(t, worker, "commit", "-q", "-m", "more")
	gitAt(t, worker, "push", "-q", "origin", "b")
	newer, _, _ := runStIn(t, worker, "", "graph", "export", "--all")

	_, stderr, ok := runStIn(t, mine, newer, "graph", "import", "--no-fetch", "-")
	if ok || !strings.Contains(stderr, "not in this repository") {
		t.Errorf("--no-fetch with a missing commit: ok=%v %s", ok, stderr)
	}

	_, stderr, ok = runStIn(t, mine, doc, "graph", "import", "--no-fetch", "-")
	if !ok {
		t.Errorf("--no-fetch with commits present failed: %s", stderr)
	}
}

func TestGraphImportExitsOneOnAConflictAndAppliesTheRest(t *testing.T) {
	worker, mine, root := twoClones(t)
	// mine has its own a, diverged from the worker's.
	gitAt(t, mine, "checkout", "-q", "-b", "a")
	os.WriteFile(filepath.Join(mine, "mine.txt"), []byte("mine"), 0o644)
	gitAt(t, mine, "add", "mine.txt")
	gitAt(t, mine, "commit", "-q", "-m", "mine")
	if _, e, ok := runStIn(t, mine, "", "attach", "a", "--parent", root); !ok {
		t.Fatalf("attach a: %s", e)
	}
	gitAt(t, worker, "checkout", "-q", "-b", "c")
	os.WriteFile(filepath.Join(worker, "c.txt"), []byte("c"), 0o644)
	gitAt(t, worker, "add", "c.txt")
	gitAt(t, worker, "commit", "-q", "-m", "c")
	if _, e, ok := runStIn(t, worker, "", "attach", "c", "--parent", root); !ok {
		t.Fatalf("attach c: %s", e)
	}
	gitAt(t, worker, "push", "-q", "origin", "c")
	doc, stderr, ok := runStIn(t, worker, "", "graph", "export", "--all")
	if !ok {
		t.Fatalf("export: %s", stderr)
	}

	out, stderr, ok := runStIn(t, mine, doc, "graph", "import", "-")

	if ok {
		t.Fatalf("exit 0 with a conflict:\n%s", out)
	}
	assertContains(t, out, "conflict")
	assertContains(t, out, "origin/a")
	assertContains(t, stderr, "not merged")
	if gitAt(t, mine, "branch", "--list", "c") == "" || loadGraphInDir(t, mine).Branches["c"] == nil {
		t.Error("c was not imported alongside the conflict")
	}
}

func TestGraphImportIntoASharedGraph(t *testing.T) {
	worker, mine, root := twoClones(t)
	doc, _, _ := runStIn(t, worker, "", "graph", "export", "--all")
	stack := filepath.Join(mine, ".git", "stack")
	os.MkdirAll(stack, 0o755)
	os.WriteFile(filepath.Join(stack, "graph.json"), []byte(`{"version":1,"root":"`+root+`","branches":{}}`), 0o644)
	if _, stderr, ok := runStIn(t, mine, "", "graph", "share"); !ok {
		t.Fatalf("share: %s", stderr)
	}

	if _, stderr, ok := runStIn(t, mine, doc, "graph", "import", "-"); !ok {
		t.Fatalf("import: %s", stderr)
	}

	if which, _, _ := runStIn(t, mine, "", "graph", "which"); !strings.HasPrefix(which, "Shared") {
		t.Errorf("the graph left shared mode: %s", which)
	}
	if _, err := os.Stat(filepath.Join(stack, "graph.json")); err == nil {
		t.Error("the import wrote a local graph file in shared mode")
	}
	if g := loadGraphInDir(t, mine); g.Branches["b"] == nil {
		t.Errorf("the shared graph lacks the imported branches: %+v", g.Branches)
	}
}
