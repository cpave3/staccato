package transfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpave3/staccato/pkg/graph"
)

// exported is the document of c's current stack, every branch pushed.
func exported(t *testing.T, c *clone, branch string) *Document {
	t.Helper()
	doc, err := Export(c.context(), ExportOptions{Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// importInto plans and applies doc in c, returning the outcome of each branch
// as "name:outcome".
func importInto(t *testing.T, c *clone, doc *Document, opts ImportOptions) (*ImportPlan, string) {
	t.Helper()
	sc := c.context()
	plan, err := Plan(sc, doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(sc, plan); err != nil {
		t.Fatal(err)
	}
	return plan, outcomes(plan)
}

func outcomes(plan *ImportPlan) string {
	var out []string
	for _, b := range plan.Branches {
		out = append(out, b.Name+":"+string(b.Outcome))
	}
	return strings.Join(out, ",")
}

func reasonOf(plan *ImportPlan, name string) string {
	for _, b := range plan.Branches {
		if b.Name == name {
			return b.Reason
		}
	}
	return ""
}

func entry(g *graph.Graph, name string) graph.Branch {
	if b, ok := g.Branches[name]; ok {
		return *b
	}
	return graph.Branch{}
}

func docEntry(doc *Document, name string) graph.Branch {
	for _, b := range doc.Branches {
		if b.Name == name {
			return graph.Branch{Name: b.Name, Parent: b.Parent, BaseSHA: b.BaseSHA, HeadSHA: b.HeadSHA}
		}
	}
	return graph.Branch{}
}

// workerStack is a worker clone with a pushed stack a → b on main, and the
// person's clone of the same origin.
func workerStack(t *testing.T) (worker, mine *clone, doc *Document) {
	t.Helper()
	origin, worker := newOrigin(t)
	worker.stack("a=main", "b=a")
	worker.push("a", "b")
	return worker, cloneOf(t, origin), exported(t, worker, "b")
}

func TestImportAStackNewToThisClone(t *testing.T) {
	worker, mine, doc := workerStack(t)

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:added,b:added" {
		t.Fatalf("outcomes = %s", got)
	}
	g := mine.graph()
	for _, name := range []string{"a", "b"} {
		if mine.sha(name) != worker.sha(name) {
			t.Errorf("%s = %s, want %s", name, mine.sha(name), worker.sha(name))
		}
		if entry(g, name) != docEntry(doc, name) {
			t.Errorf("entry %s = %+v, want the document's %+v", name, entry(g, name), docEntry(doc, name))
		}
		if upstream := mine.git("rev-parse", "--abbrev-ref", name+"@{upstream}"); upstream != "origin/"+name {
			t.Errorf("%s tracks %q, want origin/%s", name, upstream, name)
		}
	}
	if mine.git("branch", "--show-current") != "main" {
		t.Error("the import changed the checkout")
	}
}

func TestImportTwiceIsUnchanged(t *testing.T) {
	_, mine, doc := workerStack(t)
	importInto(t, mine, doc, ImportOptions{})

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:unchanged,b:unchanged" {
		t.Errorf("outcomes = %s", got)
	}
}

func TestImportUpdatesAnEntryForABranchAlreadyAtTheHead(t *testing.T) {
	_, mine, doc := workerStack(t)
	mine.git("fetch", "-q", "origin")
	mine.git("branch", "a", "origin/a") // same commit, not in the graph

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:updated,b:added" || entry(mine.graph(), "a") != docEntry(doc, "a") {
		t.Errorf("outcomes = %s, a = %+v", got, entry(mine.graph(), "a"))
	}
}

func TestImportFastForwardsAStackTheWorkerBuiltOn(t *testing.T) {
	origin, mine := newOrigin(t)
	mine.stack("a=main")
	mine.push("a")
	worker := cloneOf(t, origin)
	importInto(t, worker, exported(t, mine, "a"), ImportOptions{})
	worker.git("checkout", "-q", "a")
	worker.commit("more.txt", "more")
	worker.stack("b=a")
	sc := worker.context()
	sc.Graph.UpdateBranch("a", sc.Graph.Branches["a"].BaseSHA, worker.sha("a"))
	sc.Save()
	worker.push("a", "b")
	doc := exported(t, worker, "b")

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:fast-forwarded,b:added" {
		t.Fatalf("outcomes = %s", got)
	}
	if mine.sha("a") != worker.sha("a") || entry(mine.graph(), "a") != docEntry(doc, "a") {
		t.Errorf("a = %s %+v, want the worker's", mine.sha("a"), entry(mine.graph(), "a"))
	}
}

func TestImportFastForwardsTheCleanCheckout(t *testing.T) {
	origin, mine := newOrigin(t)
	mine.stack("a=main")
	mine.push("a")
	worker := cloneOf(t, origin)
	importInto(t, worker, exported(t, mine, "a"), ImportOptions{})
	worker.git("checkout", "-q", "a")
	worker.commit("more.txt", "more")
	worker.push("a")
	mine.git("checkout", "-q", "a")

	_, got := importInto(t, mine, exported(t, worker, "a"), ImportOptions{})

	if got != "a:fast-forwarded" || mine.sha("HEAD") != worker.sha("a") {
		t.Fatalf("outcomes = %s, HEAD = %s", got, mine.sha("HEAD"))
	}
	if _, err := os.Stat(filepath.Join(mine.dir, "more.txt")); err != nil {
		t.Errorf("the working tree was not moved with the branch: %v", err)
	}
	if status := mine.git("status", "--porcelain"); status != "" {
		t.Errorf("working tree not clean after fast-forward:\n%s", status)
	}
}

// aheadOnWorker sets up a stack a on main in mine, and the worker's a one
// commit further, pushed; it returns the worker's document.
func aheadOnWorker(t *testing.T) (mine *clone, doc *Document) {
	t.Helper()
	origin, mine := newOrigin(t)
	mine.stack("a=main")
	mine.push("a")
	worker := cloneOf(t, origin)
	importInto(t, worker, exported(t, mine, "a"), ImportOptions{})
	worker.git("checkout", "-q", "a")
	worker.commit("more.txt", "more")
	worker.push("a")
	return mine, exported(t, worker, "a")
}

func TestImportSkipsACheckoutWithUncommittedChanges(t *testing.T) {
	mine, doc := aheadOnWorker(t)
	mine.git("checkout", "-q", "a")
	before := mine.sha("a")
	os.WriteFile(filepath.Join(mine.dir, "README"), []byte("edited"), 0o644)

	plan, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:skipped" || !strings.Contains(reasonOf(plan, "a"), "uncommitted changes") {
		t.Errorf("outcomes = %s (%s)", got, reasonOf(plan, "a"))
	}
	if mine.sha("a") != before || entry(mine.graph(), "a").HeadSHA != before {
		t.Error("a was moved")
	}
}

func TestImportIgnoresUntrackedFilesInTheCheckout(t *testing.T) {
	mine, doc := aheadOnWorker(t)
	mine.git("checkout", "-q", "a")
	os.WriteFile(filepath.Join(mine.dir, "scratch.txt"), []byte("mine"), 0o644)

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:fast-forwarded" {
		t.Errorf("outcomes = %s", got)
	}
}

func TestImportSkipsWhenGitRefusesToFastForwardTheCheckout(t *testing.T) {
	mine, doc := aheadOnWorker(t)
	mine.git("checkout", "-q", "a")
	before := mine.sha("a")
	// An untracked file where the incoming commit adds one.
	os.WriteFile(filepath.Join(mine.dir, "more.txt"), []byte("mine"), 0o644)

	plan, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:skipped" || !strings.Contains(reasonOf(plan, "a"), "more.txt") {
		t.Errorf("outcomes = %s (%s)", got, reasonOf(plan, "a"))
	}
	if mine.sha("a") != before {
		t.Error("a was moved")
	}
}

func TestImportSkipsABranchCheckedOutInAnotherWorktree(t *testing.T) {
	mine, doc := aheadOnWorker(t)
	before := mine.sha("a")
	wt := filepath.Join(t.TempDir(), "wt")
	mine.git("worktree", "add", "-q", wt, "a")

	plan, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:skipped" || !strings.Contains(reasonOf(plan, "a"), "worktree") {
		t.Errorf("outcomes = %s (%s)", got, reasonOf(plan, "a"))
	}
	if mine.sha("a") != before {
		t.Error("a was moved under another worktree")
	}
}

func TestImportKeepsLocalWorkThatIsAhead(t *testing.T) {
	_, mine, doc := workerStack(t)
	importInto(t, mine, doc, ImportOptions{})
	mine.git("checkout", "-q", "a")
	ahead := mine.commit("local.txt", "local")
	mine.git("checkout", "-q", "main")
	local := entry(mine.graph(), "a")

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:kept-local,b:unchanged" || mine.sha("a") != ahead || entry(mine.graph(), "a") != local {
		t.Errorf("outcomes = %s, a = %s %+v", got, mine.sha("a"), entry(mine.graph(), "a"))
	}
}

func TestImportKeepsLocalWorkOnABranchNotInTheGraph(t *testing.T) {
	_, mine, doc := workerStack(t)
	mine.git("fetch", "-q", "origin")
	mine.git("checkout", "-q", "-b", "a", "origin/a")
	ahead := mine.commit("local.txt", "local")
	mine.git("checkout", "-q", "main")

	_, got := importInto(t, mine, doc, ImportOptions{})

	want := docEntry(doc, "a")
	want.HeadSHA = ahead
	if got != "a:kept-local,b:added" || entry(mine.graph(), "a") != want {
		t.Errorf("outcomes = %s, a = %+v, want %+v", got, entry(mine.graph(), "a"), want)
	}
}

func TestImportLeavesADivergedBranchAlone(t *testing.T) {
	worker, mine, doc := workerStack(t)
	importInto(t, mine, doc, ImportOptions{})
	mine.git("checkout", "-q", "a")
	mine.commit("local.txt", "local")
	mine.git("checkout", "-q", "main")
	localA, localB := mine.sha("a"), entry(mine.graph(), "a")
	worker.git("checkout", "-q", "a")
	worker.commit("theirs.txt", "theirs")
	worker.stack("c=main")
	sc := worker.context()
	sc.Graph.UpdateBranch("a", sc.Graph.Branches["a"].BaseSHA, worker.sha("a"))
	sc.Save()
	worker.push("a", "c")
	doc, err := Export(worker.context(), ExportOptions{All: true, AllowUnpushed: true})
	if err != nil {
		t.Fatal(err)
	}

	plan, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:conflict,b:unchanged,c:added" {
		t.Fatalf("outcomes = %s", got)
	}
	if !strings.Contains(reasonOf(plan, "a"), "origin/a") {
		t.Errorf("reason = %q, want it to point at origin/a", reasonOf(plan, "a"))
	}
	if mine.sha("a") != localA || entry(mine.graph(), "a") != localB {
		t.Error("the diverged branch or its entry was changed")
	}
}

func TestImportOfADivergedBranchNotInTheGraphAddsNoEntry(t *testing.T) {
	_, mine, doc := workerStack(t)
	mine.git("checkout", "-q", "-b", "a")
	mine.commit("local.txt", "local")
	mine.git("checkout", "-q", "main")

	plan, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:conflict,b:skipped" || reasonOf(plan, "b") != "parent a was not merged" {
		t.Errorf("outcomes = %s (b: %s)", got, reasonOf(plan, "b"))
	}
	if _, ok := mine.graph().Branches["a"]; ok {
		t.Error("an entry was added for the diverged branch")
	}
	if mine.git("branch", "--list", "b") != "" {
		t.Error("b was created on top of a branch that was not merged")
	}
}

func TestImportKeepsAChildOfAConflictWithALocalEntry(t *testing.T) {
	worker, mine, doc := workerStack(t)
	importInto(t, mine, doc, ImportOptions{})
	mine.git("checkout", "-q", "a")
	mine.commit("local.txt", "local")
	mine.git("checkout", "-q", "main")
	worker.git("checkout", "-q", "a")
	worker.commit("theirs.txt", "theirs")
	worker.stack("b2=a")
	worker.push("a", "b2")
	doc, err := Export(worker.context(), ExportOptions{Branch: "b2", AllowUnpushed: true})
	if err != nil {
		t.Fatal(err)
	}

	_, got := importInto(t, mine, doc, ImportOptions{})

	if !strings.Contains(got, "a:conflict") || !strings.Contains(got, "b2:added") || entry(mine.graph(), "b2").Parent != "a" {
		t.Errorf("outcomes = %s, b2 = %+v", got, entry(mine.graph(), "b2"))
	}
}

func TestImportLeavesBranchesTheDocumentDoesNotName(t *testing.T) {
	_, mine, doc := workerStack(t)
	mine.stack("mine=main")
	before := entry(mine.graph(), "mine")

	importInto(t, mine, doc, ImportOptions{})

	if entry(mine.graph(), "mine") != before || mine.git("branch", "--list", "mine") == "" {
		t.Error("a branch the document does not name was changed")
	}
}

func TestImportRefusesACycle(t *testing.T) {
	_, mine := newOrigin(t)
	mine.stack("y=main", "x=y")
	doc := &Document{Format: Format, Kind: Kind, Root: "main", Branches: []Branch{
		{Name: "y", Parent: "x", BaseSHA: mine.sha("x"), HeadSHA: mine.sha("y")},
	}}
	before := mine.graph()

	_, err := Plan(mine.context(), doc, ImportOptions{NoFetch: true})

	if err == nil || !strings.Contains(err.Error(), "x") || !strings.Contains(err.Error(), "y") || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("err = %v, want a cycle through x and y", err)
	}
	if entry(mine.graph(), "y") != entry(before, "y") {
		t.Error("the graph was changed")
	}
}

func TestImportRefusesAnotherRoot(t *testing.T) {
	_, mine, doc := workerStack(t)
	mine.stack("mine=main")
	sc := mine.context()
	sc.Graph.Root = "develop"
	sc.Save()

	_, err := Plan(mine.context(), doc, ImportOptions{})

	if err == nil || !strings.Contains(err.Error(), "main") || !strings.Contains(err.Error(), "develop") {
		t.Errorf("err = %v, want it to name both roots", err)
	}
}

func TestImportIntoAnEmptyGraphAdoptsTheRoot(t *testing.T) {
	_, mine, doc := workerStack(t)
	sc := mine.context()
	sc.Graph.Root = "elsewhere"
	sc.Save()

	_, got := importInto(t, mine, doc, ImportOptions{})

	if got != "a:added,b:added" || mine.graph().Root != "main" {
		t.Errorf("outcomes = %s, root = %s", got, mine.graph().Root)
	}
}

func TestImportRefusesAMissingCommit(t *testing.T) {
	_, mine, doc := workerStack(t)
	doc.Branches[1].HeadSHA = strings.Repeat("ab", 20)

	_, err := Plan(mine.context(), doc, ImportOptions{NoFetch: true})

	if err == nil || !strings.Contains(err.Error(), "b") || !strings.Contains(err.Error(), "abababab") {
		t.Errorf("err = %v, want it to name b and its commit", err)
	}
	if mine.git("branch", "--list", "a") != "" {
		t.Error("a branch was created")
	}
}

func TestImportFailsWhenTheRemoteBranchIsGone(t *testing.T) {
	worker, mine, doc := workerStack(t)
	worker.git("push", "-q", "origin", "--delete", "b")

	_, err := Plan(mine.context(), doc, ImportOptions{})

	if err == nil || !strings.Contains(err.Error(), "b is not on origin") {
		t.Errorf("err = %v, want it to say b is not on origin", err)
	}
}

func TestImportRefusesABadBranchNameBeforeFetching(t *testing.T) {
	for _, name := range []string{"a b", "a\ndelete refs/heads/main", "a..b", "-a", "@{-1}"} {
		_, mine, doc := workerStack(t)
		doc.Branches[0].Name = name
		doc.Branches[1].Parent = name

		_, err := Plan(mine.context(), doc, ImportOptions{Remote: "nowhere"})

		if err == nil || !strings.Contains(err.Error(), "not a valid branch name") {
			t.Errorf("%q: err = %v, want it refused as a branch name", name, err)
		}
	}
}
