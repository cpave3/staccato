package transfer

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

// Outcome is what an import does with one branch of a document.
type Outcome string

const (
	// Added: no local branch; it is created at the document's head.
	Added Outcome = "added"
	// Updated: the local branch is at the document's head; the entry is the document's.
	Updated Outcome = "updated"
	// Unchanged: as Updated, and the local entry already matched.
	Unchanged Outcome = "unchanged"
	// FastForwarded: the local branch is behind; it is moved to the document's head.
	FastForwarded Outcome = "fast-forwarded"
	// KeptLocal: the local branch already has the document's head and more.
	KeptLocal Outcome = "kept-local"
	// Conflict: each side has commits the other lacks; nothing is changed.
	Conflict Outcome = "conflict"
	// Skipped: the branch could not be merged now; Reason says why.
	Skipped Outcome = "skipped"
)

// ImportOptions controls Plan.
type ImportOptions struct {
	// Remote to fetch the document's branches from; "" for origin.
	Remote string
	// NoFetch uses the commits already in the repository.
	NoFetch bool
}

// BranchResult is the outcome of one document branch.
type BranchResult struct {
	Name    string  `json:"name"`
	Outcome Outcome `json:"outcome"`
	Reason  string  `json:"reason"`
}

// ImportPlan is every decision of an import, made before anything changes.
type ImportPlan struct {
	// Root is the root of the merged graph.
	Root string `json:"root"`
	// Branches are the document's branches, in document order.
	Branches []BranchResult `json:"branches"`

	remote string
	doc    *Document
	local  *graph.Graph
	facts  map[string]branchFacts
	// dirty: this worktree has uncommitted changes to tracked files.
	dirty bool
	// refused holds the reason git gave for not fast-forwarding a checkout.
	refused map[string]string

	merged *graph.Graph
	moves  map[string]move
}

// Unmerged is the number of branches that were a conflict or skipped.
func (p *ImportPlan) Unmerged() int {
	n := 0
	for _, b := range p.Branches {
		if b.Outcome == Conflict || b.Outcome == Skipped {
			n++
		}
	}
	return n
}

// branchFacts is what git says about one document branch in this clone.
type branchFacts struct {
	local       string // the local branch's tip; "" when there is none
	tracking    string // <remote>/<name>'s commit; "" when there is none
	localBehind bool   // the local tip is an ancestor of the document's head
	localAhead  bool   // the document's head is an ancestor of the local tip
	here        bool   // checked out in this worktree
	worktree    string // the other worktree it is checked out in, if any
}

// move is a change to one branch: created at to (from == ""), or moved from
// from to to. here moves the branch checked out in this worktree.
type move struct {
	from, to string
	here     bool
}

// Plan decides what importing doc into sc does, branch by branch, and checks
// the merged graph, without changing any branch or the graph. Unless
// opts.NoFetch, it fetches the document's branches into their
// remote-tracking branches first; that is the only thing it changes.
func Plan(sc *stcontext.StaccatoContext, doc *Document, opts ImportOptions) (*ImportPlan, error) {
	r := sc.Git
	remote := opts.Remote
	if remote == "" {
		remote = DefaultRemote
	}

	// The document comes from another machine: its names reach git only
	// once git agrees they are branch names.
	for _, name := range documentNames(doc) {
		if err := checkBranchName(r, name); err != nil {
			return nil, err
		}
	}

	local := sc.Graph
	root := local.Root
	if doc.Root != root {
		if len(local.Branches) > 0 {
			return nil, fmt.Errorf("the document's stack is on %s, but this clone's graph is on %s", doc.Root, root)
		}
		root = doc.Root
	}

	if !opts.NoFetch && len(doc.Branches) > 0 {
		if err := fetch(r, remote, doc); err != nil {
			return nil, err
		}
	}
	for _, b := range doc.Branches {
		if _, err := r.Run("cat-file", "-e", b.HeadSHA+"^{commit}"); err != nil {
			hint := ""
			if opts.NoFetch {
				hint = " (it was not fetched: try without --no-fetch)"
			}
			return nil, fmt.Errorf("branch %s: commit %s is not in this repository%s", b.Name, b.HeadSHA, hint)
		}
	}

	facts, err := gatherFacts(r, remote, doc)
	if err != nil {
		return nil, err
	}
	// Untracked files are left to git: a fast-forward refuses only one in
	// its way, and Apply then skips the branch with git's reason.
	status, err := r.Run("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return nil, err
	}

	p := &ImportPlan{Root: root, remote: remote, doc: doc, local: local, facts: facts, dirty: status != "", refused: map[string]string{}}
	if err := p.decide(); err != nil {
		return nil, err
	}
	return p, nil
}

// documentNames are the root and every branch and parent the document names.
func documentNames(doc *Document) []string {
	names := []string{doc.Root}
	for _, b := range doc.Branches {
		names = append(names, b.Name, b.Parent)
	}
	return names
}

// checkBranchName refuses a name git would not accept as a branch, or would
// read as something else.
func checkBranchName(r *git.Runner, name string) error {
	bad := strings.HasPrefix(name, "-") || name == "HEAD" || strings.ContainsAny(name, " \t\r\n")
	if !bad {
		_, err := r.Run("check-ref-format", "refs/heads/"+name)
		bad = err != nil
	}
	if bad {
		return fmt.Errorf("the document names %q, which is not a valid branch name", name)
	}
	return nil
}

var missingRemoteRef = regexp.MustCompile(`couldn't find remote ref refs/heads/(\S+)`)

// fetch brings every document branch into <remote>/<name> in one call.
func fetch(r *git.Runner, remote string, doc *Document) error {
	if _, err := r.Run("remote", "get-url", remote); err != nil {
		return fmt.Errorf("there is no remote named %s: name another with --remote, or use --no-fetch", remote)
	}
	args := []string{"fetch", "--quiet", "--no-tags", remote}
	for _, b := range doc.Branches {
		args = append(args, "+refs/heads/"+b.Name+":refs/remotes/"+remote+"/"+b.Name)
	}
	if _, err := r.Run(args...); err != nil {
		if m := missingRemoteRef.FindStringSubmatch(err.Error()); m != nil {
			return fmt.Errorf("fetching the document's branches: %s is not on %s (deleted after the export?); nothing was changed", m[1], remote)
		}
		return fmt.Errorf("fetching the document's branches from %s: %w", remote, err)
	}
	return nil
}

// gatherFacts asks git about each document branch.
func gatherFacts(r *git.Runner, remote string, doc *Document) (map[string]branchFacts, error) {
	current, _ := r.Run("symbolic-ref", "-q", "--short", "HEAD")
	worktrees, err := branchWorktrees(r)
	if err != nil {
		return nil, err
	}
	facts := map[string]branchFacts{}
	for _, b := range doc.Branches {
		f := branchFacts{here: b.Name == current}
		if !f.here {
			f.worktree = worktrees[b.Name]
		}
		f.tracking, _ = r.Run("rev-parse", "--verify", "-q", "refs/remotes/"+remote+"/"+b.Name)
		if tip, err := r.Run("rev-parse", "--verify", "-q", "refs/heads/"+b.Name); err == nil {
			f.local = tip
			if f.localBehind, err = r.IsAncestor(tip, b.HeadSHA); err != nil {
				return nil, err
			}
			if f.localAhead, err = r.IsAncestor(b.HeadSHA, tip); err != nil {
				return nil, err
			}
		}
		facts[b.Name] = f
	}
	return facts, nil
}

// branchWorktrees maps each checked-out branch to its worktree.
func branchWorktrees(r *git.Runner) (map[string]string, error) {
	out, err := r.Run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	var path string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			m[strings.TrimPrefix(line, "branch refs/heads/")] = path
		}
	}
	return m, nil
}

// decide works out every branch's outcome and the merged graph from the
// facts, then checks the merged graph.
func (p *ImportPlan) decide() error {
	merged := copyGraph(p.local)
	merged.Root = p.Root
	results := map[string]*BranchResult{}
	moves := map[string]move{}

	for _, b := range p.doc.Branches {
		f := p.facts[b.Name]
		incoming := &graph.Branch{Name: b.Name, Parent: b.Parent, BaseSHA: b.BaseSHA, HeadSHA: b.HeadSHA}
		localEntry, inGraph := p.local.Branches[b.Name]
		res := &BranchResult{Name: b.Name}
		results[b.Name] = res

		switch {
		case f.local == "":
			res.Outcome = Added
			merged.Branches[b.Name] = incoming
			moves[b.Name] = move{to: b.HeadSHA}
		case f.local == b.HeadSHA:
			res.Outcome = Updated
			if inGraph && *localEntry == *incoming {
				res.Outcome = Unchanged
			}
			merged.Branches[b.Name] = incoming
		case f.localBehind:
			switch {
			case f.worktree != "":
				res.Outcome, res.Reason = Skipped, fmt.Sprintf("checked out in the worktree at %s: fast-forward it there, or check out another branch there and import again", f.worktree)
			case f.here && p.dirty:
				res.Outcome, res.Reason = Skipped, "checked out here with uncommitted changes: commit or stash them and import again"
			case p.refused[b.Name] != "":
				res.Outcome, res.Reason = Skipped, p.refused[b.Name]
			default:
				res.Outcome = FastForwarded
				merged.Branches[b.Name] = incoming
				moves[b.Name] = move{from: f.local, to: b.HeadSHA, here: f.here}
			}
		case f.localAhead:
			res.Outcome = KeptLocal
			if !inGraph {
				merged.Branches[b.Name] = &graph.Branch{Name: b.Name, Parent: b.Parent, BaseSHA: b.BaseSHA, HeadSHA: f.local}
			}
		default:
			res.Outcome = Conflict
			res.Reason = fmt.Sprintf("%s and the document's head have each got commits the other lacks: compare them with 'git log --oneline %s...%s' and reconcile by hand", b.Name, b.Name, p.compareWith(b))
		}
	}

	// A branch whose parent will not be in the graph cannot be merged, and
	// neither can the branches above it.
	for changed := true; changed; {
		changed = false
		for _, b := range p.doc.Branches {
			res := results[b.Name]
			if res.Outcome == Conflict || res.Outcome == Skipped {
				continue
			}
			e, ok := merged.Branches[b.Name]
			if !ok || e.Parent == merged.Root {
				continue
			}
			if _, ok := merged.Branches[e.Parent]; ok {
				continue
			}
			res.Outcome, res.Reason = Skipped, "parent "+e.Parent+" was not merged"
			if localEntry, ok := p.local.Branches[b.Name]; ok {
				merged.Branches[b.Name] = copyBranch(localEntry)
			} else {
				delete(merged.Branches, b.Name)
			}
			delete(moves, b.Name)
			changed = true
		}
	}

	if cycle := findCycle(merged); len(cycle) > 0 {
		return fmt.Errorf("the merged graph would have a cycle through %s; nothing was changed", strings.Join(cycle, ", "))
	}

	p.Branches = p.Branches[:0]
	for _, b := range p.doc.Branches {
		p.Branches = append(p.Branches, *results[b.Name])
	}
	p.merged, p.moves = merged, moves
	return nil
}

// compareWith is what to compare a diverged branch with: its
// remote-tracking branch when that is the document's head, else the head.
func (p *ImportPlan) compareWith(b Branch) string {
	if p.facts[b.Name].tracking == b.HeadSHA {
		return p.remote + "/" + b.Name
	}
	return b.HeadSHA[:12]
}

// findCycle returns the branches of a cycle in g's parents, sorted, or nil.
func findCycle(g *graph.Graph) []string {
	done := map[string]bool{}
	names := make([]string, 0, len(g.Branches))
	for name := range g.Branches {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, start := range names {
		path := map[string]int{}
		var order []string
		for name := start; ; {
			if done[name] {
				break
			}
			if i, seen := path[name]; seen {
				cycle := append([]string(nil), order[i:]...)
				sort.Strings(cycle)
				return cycle
			}
			b, ok := g.Branches[name]
			if !ok {
				break
			}
			path[name] = len(order)
			order = append(order, name)
			name = b.Parent
		}
		for _, name := range order {
			done[name] = true
		}
	}
	return nil
}

func copyBranch(b *graph.Branch) *graph.Branch {
	c := *b
	return &c
}

func copyGraph(g *graph.Graph) *graph.Graph {
	c := &graph.Graph{Version: g.Version, Root: g.Root, Branches: make(map[string]*graph.Branch, len(g.Branches))}
	for name, b := range g.Branches {
		c.Branches[name] = copyBranch(b)
	}
	return c
}

// Apply carries out a plan: it fast-forwards the branch checked out here
// (if the plan moves it), moves and creates the other branches in one
// git update-ref transaction, sets up tracking for the created ones, and
// saves the merged graph once. If git refuses to fast-forward the checkout,
// that branch is skipped with git's reason and the plan decided again.
func Apply(sc *stcontext.StaccatoContext, p *ImportPlan) error {
	r := sc.Git
	for name, m := range p.moves {
		if !m.here {
			continue
		}
		if _, err := r.Run("merge", "--ff-only", "--quiet", m.to); err != nil {
			p.refused[name] = "git refused to fast-forward the checkout: " + gitMessage(err)
			if err := p.decide(); err != nil {
				return err
			}
		}
		break // only one branch is checked out here
	}

	var stdin strings.Builder
	var created []string
	for _, b := range p.doc.Branches {
		m, ok := p.moves[b.Name]
		if !ok || m.here {
			continue
		}
		if m.from == "" {
			fmt.Fprintf(&stdin, "create refs/heads/%s %s\n", b.Name, m.to)
			created = append(created, b.Name)
		} else {
			fmt.Fprintf(&stdin, "update refs/heads/%s %s %s\n", b.Name, m.to, m.from)
		}
	}
	if stdin.Len() > 0 {
		if _, err := r.Output(nil, stdin.String(), "update-ref", "--stdin"); err != nil {
			return fmt.Errorf("moving the branches (none was moved): %w", err)
		}
	}
	for _, name := range created {
		tracking := "refs/remotes/" + p.remote + "/" + name
		if _, err := r.Run("rev-parse", "--verify", "-q", tracking); err == nil {
			// Tracking is a convenience; the branch is right without it.
			_, _ = r.Run("branch", "--set-upstream-to="+p.remote+"/"+name, name)
		}
	}

	if reflect.DeepEqual(p.merged, p.local) {
		return nil
	}
	sc.Graph.Root = p.merged.Root
	sc.Graph.Branches = p.merged.Branches
	return sc.Save()
}

// gitMessage is what git printed for a failed command run through
// git.Runner.Run, on one line, without "Aborting".
func gitMessage(err error) string {
	msg := err.Error()
	if _, out, ok := strings.Cut(msg, "\nOutput: "); ok {
		msg = out
	}
	var parts []string
	for _, line := range strings.Split(msg, "\n") {
		if line = strings.TrimSpace(line); line != "" && line != "Aborting" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ")
}
