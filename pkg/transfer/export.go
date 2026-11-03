package transfer

import (
	"fmt"
	"strings"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/restack"
)

// DefaultRemote is the remote export compares with and import fetches from.
const DefaultRemote = "origin"

// ExportOptions chooses what Export writes.
type ExportOptions struct {
	// Branch picks the stack containing this branch; "" for the current one.
	Branch string
	// All exports every branch in the graph.
	All bool
	// Remote is the remote whose tracking branches each tip must equal;
	// "" for origin.
	Remote string
	// AllowUnpushed skips that check.
	AllowUnpushed bool
	// STVersion is recorded in the document for information.
	STVersion string
}

// Export writes the document for a stack of sc's graph: the stack the
// current branch (or opts.Branch) belongs to, from its bottom branch up, or
// every branch with opts.All. Each branch's head is its git tip, which must
// equal <remote>/<name> unless opts.AllowUnpushed. It changes nothing.
func Export(sc *stcontext.StaccatoContext, opts ExportOptions) (*Document, error) {
	g := sc.Graph
	remote := opts.Remote
	if remote == "" {
		remote = DefaultRemote
	}

	var selected []string
	if opts.All {
		for name := range g.Branches {
			selected = append(selected, name)
		}
	} else {
		start := opts.Branch
		if start == "" {
			current, err := sc.Git.GetCurrentBranch()
			if err != nil || current == "HEAD" {
				return nil, fmt.Errorf("HEAD is detached: check out a branch of the stack, or use --branch <name> or --all")
			}
			start = current
		}
		if _, ok := g.GetBranch(start); !ok {
			what := "is not in a stack"
			if start == g.Root {
				what = "is the root, not a branch of a stack"
			}
			return nil, fmt.Errorf("%s %s: check out a branch of the stack to export, or use --branch <name> or --all", start, what)
		}
		bottom := start
		for _, name := range restack.GetAncestors(g, start) {
			if name != g.Root {
				bottom = name
				break
			}
		}
		selected = restack.GetStackBranches(g, bottom)
	}

	doc := &Document{Format: Format, Kind: Kind, STVersion: opts.STVersion, Root: g.Root, Branches: []Branch{}}
	var missing, mismatched []string
	for _, name := range Order(g, selected) {
		b := g.Branches[name]
		tip, err := sc.Git.Run("rev-parse", "--verify", "-q", "refs/heads/"+name)
		if err != nil {
			missing = append(missing, name)
			continue
		}
		if !opts.AllowUnpushed {
			if problem := comparePushed(sc.Git, remote, name, tip); problem != "" {
				mismatched = append(mismatched, problem)
			}
		}
		doc.Branches = append(doc.Branches, Branch{Name: name, Parent: b.Parent, BaseSHA: b.BaseSHA, HeadSHA: tip})
	}

	if len(missing) > 0 {
		var advice []string
		for _, name := range missing {
			advice = append(advice, "st detach "+name)
		}
		return nil, fmt.Errorf("the graph has %s, but git has no such branch: remove it from the graph with '%s'", strings.Join(missing, ", "), strings.Join(advice, "', '"))
	}
	if len(mismatched) > 0 {
		return nil, fmt.Errorf("the document would not match %s:\n  %s\npush them first ('st sync'), or export the local tips with --allow-unpushed", remote, strings.Join(mismatched, "\n  "))
	}
	return doc, nil
}

// comparePushed says how branch's tip differs from <remote>/<branch>, or ""
// when they are the same commit. It reads the remote-tracking branch, not
// the remote.
func comparePushed(r *git.Runner, remote, branch, tip string) string {
	tracking := remote + "/" + branch
	pushed, err := r.Run("rev-parse", "--verify", "-q", "refs/remotes/"+tracking)
	switch {
	case err != nil:
		return fmt.Sprintf("%s is not on %s", branch, remote)
	case pushed == tip:
		return ""
	}
	if behind, _ := r.IsAncestor(tip, pushed); behind {
		return fmt.Sprintf("%s is behind %s", branch, tracking)
	}
	if ahead, _ := r.IsAncestor(pushed, tip); ahead {
		return fmt.Sprintf("%s has commits %s lacks", branch, tracking)
	}
	return fmt.Sprintf("%s has diverged from %s", branch, tracking)
}
