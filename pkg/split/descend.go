package split

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cpave3/staccato/pkg/graph"
)

// movedBranch is a descendant re-parented onto the split.
type movedBranch struct {
	name   string
	oldTip string
	newTip string
	base   string // new tip of its parent
}

// moveDescendants re-creates the commits of every descendant of the split
// branch on top of newTop.
//
// newTop holds the same content as the branch's old tip, so each descendant
// commit keeps its exact tree and only its parent changes. That needs no
// checkout and cannot conflict, which is why this does not go through the
// restack engine.
//
// A descendant that does not contain its parent's old tip was already waiting
// for a restack. It is left untouched, along with everything above it, and
// returned in stale.
func (b *builder) moveDescendants(g *graph.Graph, t *target, newTop string) (moved []movedBranch, stale []string, err error) {
	rewritten := map[string]string{t.head: newTop} // old commit → its replacement
	oldTips := map[string]string{t.name: t.head}

	queue := childNames(g, t.name)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		parentOldTip := oldTips[g.Branches[name].Parent]
		// A graph entry can outlive its branch. There is nothing to move then,
		// and it should not stop the split.
		if !b.git.RefExists("refs/heads/" + name) {
			b.warnings = append(b.warnings, fmt.Sprintf("%q is in the stack graph but has no branch, so it was skipped", name))
			continue
		}
		tip, err := b.git.GetCommitSHA("refs/heads/" + name)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve descendant %q: %w", name, err)
		}
		upToDate, err := b.git.IsAncestor(parentOldTip, tip)
		if err != nil {
			return nil, nil, fmt.Errorf("compare %q with its parent: %w", name, err)
		}
		if !upToDate {
			stale = append(stale, name)
			continue
		}

		out, err := b.git.RunWith(nil, "", "log", "--reverse", "--topo-order", "--format=%H%x1f%P%x1f%T", parentOldTip+".."+tip)
		if err != nil {
			return nil, nil, fmt.Errorf("list commits of %q: %w", name, err)
		}
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Split(line, "\x1f")
			if len(fields) != 3 {
				continue
			}
			sha, tree := fields[0], fields[2]
			parents := strings.Fields(fields[1])
			changed := false
			for i, p := range parents {
				if replacement, ok := rewritten[p]; ok {
					parents[i] = replacement
					changed = true
				}
			}
			// A commit merged in from outside the stack has no rewritten
			// parent and stays as it is.
			if !changed {
				continue
			}
			replacement, err := b.copyCommit(sha, tree, parents...)
			if err != nil {
				return nil, nil, fmt.Errorf("move %q: %w", name, err)
			}
			rewritten[sha] = replacement
		}

		newTip := rewritten[tip]
		if same, err := b.sameTree(tip, newTip); err != nil {
			return nil, nil, err
		} else if !same {
			return nil, nil, fmt.Errorf("moving %q would change its content — this is a bug in st split; nothing was changed", name)
		}
		moved = append(moved, movedBranch{name: name, oldTip: tip, newTip: newTip, base: rewritten[parentOldTip]})
		oldTips[name] = tip
		queue = append(queue, childNames(g, name)...)
	}
	return moved, stale, nil
}

// childNames returns the names of a branch's children in a stable order.
func childNames(g *graph.Graph, parent string) []string {
	var names []string
	for _, child := range g.GetChildren(parent) {
		names = append(names, child.Name)
	}
	sort.Strings(names)
	return names
}
