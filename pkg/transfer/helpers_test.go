package transfer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/graph"
)

// clone is a repository on disk for a test: a working clone of a bare
// origin shared with any other clone made from the same origin.
type clone struct {
	t    *testing.T
	dir  string
	root string
}

// gitIn runs git in dir with the user's configuration kept out.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newOrigin makes a bare origin with one commit on main, and a first clone.
func newOrigin(t *testing.T) (origin string, first *clone) {
	t.Helper()
	origin = filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	first = cloneOf(t, origin)
	first.commit("README", "hello")
	gitIn(t, first.dir, "push", "-q", "-u", "origin", "main")
	return origin, first
}

// cloneOf clones origin into a new directory, on main.
func cloneOf(t *testing.T, origin string) *clone {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	gitIn(t, filepath.Dir(dir), "clone", "-q", origin, dir)
	gitIn(t, dir, "config", "user.email", "t@example.com")
	gitIn(t, dir, "config", "user.name", "t")
	if gitIn(t, dir, "branch", "--show-current") == "" {
		gitIn(t, dir, "checkout", "-q", "-b", "main")
	}
	return &clone{t: t, dir: dir, root: "main"}
}

func (c *clone) git(args ...string) string {
	c.t.Helper()
	return gitIn(c.t, c.dir, args...)
}

// commit writes file and commits it on the current branch, returning the commit.
func (c *clone) commit(file, content string) string {
	c.t.Helper()
	if err := os.WriteFile(filepath.Join(c.dir, file), []byte(content), 0o644); err != nil {
		c.t.Fatal(err)
	}
	c.git("add", file)
	c.git("commit", "-q", "-m", "change "+file)
	return c.git("rev-parse", "HEAD")
}

func (c *clone) sha(ref string) string {
	c.t.Helper()
	return c.git("rev-parse", ref)
}

// stack creates branches, each with one commit, stacked in the order given
// as name=parent pairs, records them in the graph, and returns to main.
func (c *clone) stack(pairs ...string) {
	c.t.Helper()
	sc := c.context()
	for _, p := range pairs {
		name, parent, _ := strings.Cut(p, "=")
		c.git("checkout", "-q", parent)
		base := c.sha("HEAD")
		c.git("checkout", "-q", "-b", name)
		head := c.commit(strings.ReplaceAll(name, "/", "_")+".txt", name)
		sc.Graph.AddBranch(name, parent, base, head)
	}
	if err := sc.Save(); err != nil {
		c.t.Fatal(err)
	}
	c.git("checkout", "-q", "main")
}

func (c *clone) push(branches ...string) {
	c.t.Helper()
	c.git(append([]string{"push", "-q", "-f", "origin"}, branches...)...)
}

func (c *clone) context() *stcontext.StaccatoContext {
	c.t.Helper()
	sc, err := stcontext.Load(c.dir)
	if err != nil {
		c.t.Fatal(err)
	}
	return sc
}

func (c *clone) graph() *graph.Graph {
	c.t.Helper()
	return c.context().Graph
}

func names(doc *Document) string {
	var out []string
	for _, b := range doc.Branches {
		out = append(out, b.Name)
	}
	return strings.Join(out, ",")
}
