// Package split divides one branch of a stack into several stacked branches.
package split

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/git"
)

// Description is the inventory of a branch that an agent reads to decide where
// to cut it: every commit, every changed file, and which commits touch which
// files.
type Description struct {
	Branch string `json:"branch"`
	Parent string `json:"parent"`
	// Worktree is the checkout the description was taken from.
	Worktree string `json:"worktree"`
	// Base is the commit the branch's changes are measured from.
	Base string `json:"base"`
	// Head is the branch's tip. A plan can carry it to make sure the branch
	// has not moved by the time the plan is applied.
	Head string `json:"head"`
	// NeedsRestack is set when the branch is behind its parent; it has to be
	// restacked before it can be split.
	NeedsRestack bool     `json:"needs_restack,omitempty"`
	Children     []string `json:"children"`
	// Totals is the net change between Base and Head.
	Totals Totals `json:"totals"`
	// Churn adds up what every commit changed. When it is much larger than
	// Totals the commits rework each other, and cutting at commit boundaries
	// would show reviewers code that a later slice removes: split by paths.
	Churn   Churn        `json:"churn"`
	Commits []CommitInfo `json:"commits"`
	Files   []FileInfo   `json:"files"`
}

// Totals summarises a set of changes.
type Totals struct {
	Files      int `json:"files"`
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

// Churn is the sum of the changes made by each commit.
type Churn struct {
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

// CommitInfo describes one commit on the branch.
type CommitInfo struct {
	// SHA is abbreviated; a plan can use it as it is.
	SHA        string `json:"sha"`
	Subject    string `json:"subject"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
	// Merge commits cannot be assigned to a slice by commit.
	Merge bool     `json:"merge,omitempty"`
	Files []string `json:"files"`
}

// FileInfo describes one file changed between the branch's base and its head.
type FileInfo struct {
	Path string `json:"path"`
	// Status is "added", "modified" or "deleted". A rename is reported as a
	// deletion plus an addition so each path can be placed on its own, with
	// RenamedFrom linking the two.
	Status      string `json:"status"`
	RenamedFrom string `json:"renamed_from,omitempty"`
	Insertions  int    `json:"insertions"`
	Deletions   int    `json:"deletions"`
	Binary      bool   `json:"binary,omitempty"`
	// Commits lists the branch commits that touch this file, oldest first.
	Commits []string `json:"commits"`
}

// abbrevLength is how many characters of a commit's SHA the description shows.
const abbrevLength = 12

func abbrev(sha string) string {
	if len(sha) > abbrevLength {
		return sha[:abbrevLength]
	}
	return sha
}

// target is the branch being split and its position in the stack.
type target struct {
	name     string
	parent   string
	base     string // commit the branch's changes are measured from
	head     string
	upToDate bool // parent's tip is an ancestor of the branch
	children []string
}

// resolveTarget looks up branch (or the current branch when empty) in the stack.
func resolveTarget(sc *stcontext.StaccatoContext, branch string) (*target, error) {
	if branch == "" {
		current, err := sc.Git.GetCurrentBranch()
		if err != nil || current == "HEAD" {
			return nil, fmt.Errorf("HEAD is detached — name the branch to split")
		}
		branch = current
	}
	if branch == sc.Graph.Root {
		return nil, fmt.Errorf("cannot split the root branch %q", branch)
	}
	b, ok := sc.Graph.GetBranch(branch)
	if !ok {
		return nil, fmt.Errorf("branch %q is not in the stack", branch)
	}

	// The ref that a split moves, so a tag of the same name cannot be meant.
	head, err := sc.Git.GetCommitSHA("refs/heads/" + branch)
	if err != nil {
		return nil, fmt.Errorf("resolve branch %q: %w", branch, err)
	}
	parentTip, err := sc.Git.GetCommitSHA(b.Parent)
	if err != nil {
		return nil, fmt.Errorf("resolve parent %q: %w", b.Parent, err)
	}
	upToDate, err := sc.Git.IsAncestor(parentTip, head)
	if err != nil {
		return nil, fmt.Errorf("compare %q with its parent: %w", branch, err)
	}
	base := parentTip
	if !upToDate {
		if base, err = sc.Git.GetMergeBase(parentTip, head); err != nil {
			return nil, fmt.Errorf("get merge base: %w", err)
		}
	}

	return &target{
		name:     branch,
		parent:   b.Parent,
		base:     base,
		head:     head,
		upToDate: upToDate,
		children: childNames(sc.Graph, branch),
	}, nil
}

// Describe returns the inventory of branch, or of the current branch when
// branch is empty.
func Describe(sc *stcontext.StaccatoContext, branch string) (*Description, error) {
	t, err := resolveTarget(sc, branch)
	if err != nil {
		return nil, err
	}

	changes, err := diffChanges(sc.Git, t.base, t.head)
	if err != nil {
		return nil, err
	}

	renames, err := renamedFrom(sc.Git, t.base, t.head)
	if err != nil {
		return nil, err
	}

	d := &Description{
		Branch:       t.name,
		Parent:       t.parent,
		Worktree:     sc.RepoPath,
		Base:         t.base,
		Head:         t.head,
		NeedsRestack: !t.upToDate,
		Children:     append([]string{}, t.children...),
		Commits:      []CommitInfo{},
		Files:        []FileInfo{},
	}

	commits, err := listCommits(sc.Git, t.base, t.head)
	if err != nil {
		return nil, err
	}
	touchedBy := map[string][]string{}
	for _, c := range commits {
		info := CommitInfo{SHA: abbrev(c.sha), Subject: c.subject, Merge: len(c.parents) != 1, Files: []string{}}
		if !info.Merge {
			touched, err := diffChanges(sc.Git, c.parents[0], c.sha)
			if err != nil {
				return nil, err
			}
			for _, ch := range touched {
				info.Files = append(info.Files, ch.path)
				info.Insertions += ch.insertions
				info.Deletions += ch.deletions
				touchedBy[ch.path] = append(touchedBy[ch.path], info.SHA)
			}
		}
		d.Churn.Insertions += info.Insertions
		d.Churn.Deletions += info.Deletions
		d.Commits = append(d.Commits, info)
	}

	for _, ch := range changes {
		by := touchedBy[ch.path]
		if by == nil {
			by = []string{}
		}
		d.Files = append(d.Files, FileInfo{
			Path:        ch.path,
			Status:      ch.status,
			RenamedFrom: renames[ch.path],
			Insertions:  ch.insertions,
			Deletions:   ch.deletions,
			Binary:      ch.binary,
			Commits:     by,
		})
		d.Totals.Files++
		d.Totals.Insertions += ch.insertions
		d.Totals.Deletions += ch.deletions
	}
	return d, nil
}

// renamedFrom maps each path that git detects as a rename between two commits
// to the path it was renamed from.
func renamedFrom(g *git.Runner, from, to string) (map[string]string, error) {
	out, err := g.Output(nil, "", "diff-tree", "-r", "-M", "--diff-filter=R", "--name-status", "-z", from, to)
	if err != nil {
		return nil, fmt.Errorf("detect renames: %w", err)
	}
	// R<score> NUL <old path> NUL <new path> NUL
	fields := splitNUL(out)
	renames := map[string]string{}
	for i := 0; i+2 < len(fields); i += 3 {
		renames[fields[i+2]] = fields[i+1]
	}
	return renames, nil
}

// change is one path that differs between two commits.
type change struct {
	path       string
	status     string
	srcMode    string
	mode       string // mode at the newer commit; unset when deleted
	sha        string // blob at the newer commit
	insertions int
	deletions  int
	binary     bool
}

// diffChanges lists the paths that differ between two commits, sorted by path.
// Rename detection is off so that every path stands on its own.
func diffChanges(g *git.Runner, from, to string) ([]change, error) {
	raw, err := g.Output(nil, "", "diff-tree", "-r", "--no-renames", "--raw", "-z", "--no-abbrev", from, to)
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", from, to, err)
	}
	byPath := map[string]*change{}
	var changes []*change
	fields := splitNUL(raw)
	for i := 0; i+1 < len(fields); i += 2 {
		// :<src mode> <dst mode> <src sha> <dst sha> <status>
		meta := strings.Fields(strings.TrimPrefix(fields[i], ":"))
		if len(meta) != 5 {
			return nil, fmt.Errorf("unexpected diff-tree output %q", fields[i])
		}
		ch := &change{path: fields[i+1], srcMode: meta[0], mode: meta[1], sha: meta[3], status: "modified"}
		switch meta[4] {
		case "A":
			ch.status = "added"
		case "D":
			ch.status = "deleted"
		}
		byPath[ch.path] = ch
		changes = append(changes, ch)
	}

	numstat, err := g.Output(nil, "", "diff-tree", "-r", "--no-renames", "--numstat", "-z", from, to)
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", from, to, err)
	}
	for _, line := range splitNUL(numstat) {
		// <insertions>\t<deletions>\t<path>, with "-" counts for binary files
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		ch, ok := byPath[parts[2]]
		if !ok {
			continue
		}
		if parts[0] == "-" {
			ch.binary = true
			continue
		}
		ch.insertions, _ = strconv.Atoi(parts[0])
		ch.deletions, _ = strconv.Atoi(parts[1])
	}

	out := make([]change, 0, len(changes))
	for _, ch := range changes {
		out = append(out, *ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

// commit is one commit on the branch being split.
type commit struct {
	sha     string
	parents []string
	subject string
}

// listCommits returns the commits in base..head, oldest first.
func listCommits(g *git.Runner, base, head string) ([]commit, error) {
	out, err := g.RunWith(nil, "", "log", "--reverse", "--topo-order", "--format=%H%x1f%P%x1f%s", base+".."+head)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}
	var commits []commit
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, commit{sha: parts[0], parents: strings.Fields(parts[1]), subject: parts[2]})
	}
	return commits, nil
}

// splitNUL splits NUL-terminated git output into its records.
func splitNUL(s string) []string {
	s = strings.TrimRight(s, "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}
