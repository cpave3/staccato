package sync

import (
	"encoding/json"
	"fmt"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/restack"
)

// Options controls sync behavior.
type Options struct {
	DryRun  bool
	DownOnly bool
}

// Result reports what happened during sync.
type Result struct {
	Fetched          bool
	TrunkUpdated     bool
	MergedBranches   []string
	PushedBranches   []string
	RestackedCount   int
	Conflicts        bool
	ConflictsAt      string
	StashedFromBranch string // non-empty if we stashed uncommitted changes from a merged branch
}

// Run performs the full sync operation: fetch, detect merged, restack, push.
func Run(sc *stcontext.StaccatoContext, opts Options) (*Result, error) {
	g := sc.Graph
	gitRunner := sc.Git

	result := &Result{}

	hasRemote, _ := gitRunner.HasRemote()
	if !hasRemote {
		return nil, fmt.Errorf("no remote configured")
	}

	originalBranch, _ := gitRunner.GetCurrentBranch()

	// 1. Fetch with prune
	if err := gitRunner.FetchPrune(); err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	result.Fetched = true

	// Reconcile shared graph after fetch
	if gitRunner.RefExists(graph.SharedGraphRef) {
		reconcileSharedGraph(g, gitRunner)
	}

	trunk := g.Root

	// Dry-run: report what would happen without making changes
	if opts.DryRun {
		if gitRunner.RemoteBranchExists(trunk) {
			isAnc, err := gitRunner.IsAncestor(trunk, "origin/"+trunk)
			if err == nil && isAnc {
				behindTrunk, _ := gitRunner.IsAncestor("origin/"+trunk, trunk)
				if !behindTrunk {
					result.TrunkUpdated = true
				}
			}
		}

		merged, err := DetectMergedBranches(g, gitRunner, trunk)
		if err != nil {
			return nil, fmt.Errorf("failed to detect merged branches: %w", err)
		}
		result.MergedBranches = merged

		if !opts.DownOnly {
			remaining := restack.GetLineage(g, originalBranch)
			for _, branch := range remaining {
				if branch == trunk {
					continue
				}
				result.PushedBranches = append(result.PushedBranches, branch)
			}
		}

		return result, nil
	}

	// 2. Fast-forward trunk
	if gitRunner.RemoteBranchExists(trunk) {
		if originalBranch == trunk {
			if err := gitRunner.MergeFFOnly("origin/" + trunk); err == nil {
				result.TrunkUpdated = true
			}
		} else {
			if err := gitRunner.FastForwardBranch(trunk, "origin/"+trunk); err == nil {
				result.TrunkUpdated = true
			}
		}
	}

	// 3. Detect merged branches
	merged, err := DetectMergedBranches(g, gitRunner, trunk)
	if err != nil {
		return nil, fmt.Errorf("failed to detect merged branches: %w", err)
	}
	result.MergedBranches = merged

	// 4. Remove merged branches
	for _, branch := range merged {
		b, exists := g.GetBranch(branch)
		if !exists {
			continue
		}
		parent := b.Parent
		g.ReparentChildren(branch, parent)
		g.RemoveBranch(branch)

		cur, _ := gitRunner.GetCurrentBranch()
		if cur == branch {
			if hasChanges, _ := gitRunner.HasUncommittedChanges(); hasChanges {
				gitRunner.StashPush(fmt.Sprintf("st-sync: changes from merged branch %s", branch))
				result.StashedFromBranch = branch
			}
			gitRunner.CheckoutBranch(trunk)
			originalBranch = trunk
		}
		gitRunner.DeleteBranch(branch, true)
	}

	// 5. Save graph if branches were removed
	if len(merged) > 0 {
		if err := sc.Save(); err != nil {
			return nil, fmt.Errorf("failed to save graph: %w", err)
		}
	}

	// 6. Restack remaining branches
	if len(g.Branches) > 0 {
		backupMgr := backup.NewManager(gitRunner, sc.RepoPath)
		engine := restack.NewEngine(gitRunner, backupMgr)
		restackResult, err := engine.Restack(g, trunk)
		if err != nil {
			if restackResult != nil && restackResult.Conflicts {
				result.Conflicts = true
				result.ConflictsAt = restackResult.ConflictsAt
				sc.Save()
				return result, fmt.Errorf("conflict during restack - resolve and run 'st continue'")
			}
			return result, fmt.Errorf("restack failed: %w", err)
		}
		result.RestackedCount = len(restackResult.Completed)
		sc.Save()
	}

	// 7. Push remaining branches
	remaining := restack.GetLineage(g, originalBranch)
	if !opts.DownOnly {
		for _, branch := range remaining {
			if branch == trunk {
				continue
			}
			if err := gitRunner.Push(branch, true); err == nil {
				result.PushedBranches = append(result.PushedBranches, branch)
			}
		}
	}

	// 8. Push shared graph ref if in shared mode
	if !opts.DownOnly && gitRunner.RefExists(graph.SharedGraphRef) {
		gitRunner.PushRef(graph.SharedGraphRef)
	}

	// 9. Restore original branch if it still exists
	if originalBranch != "" {
		exists, _ := gitRunner.BranchExists(originalBranch)
		if exists {
			gitRunner.CheckoutBranch(originalBranch)
		}
	}

	return result, nil
}

// DetectMergedBranches returns branch names that have been merged into trunk.
func DetectMergedBranches(g *graph.Graph, gitRunner *git.Runner, trunk string) ([]string, error) {
	sorted, err := restack.TopologicalSort(g, trunk)
	if err != nil {
		return nil, err
	}

	var merged []string
	for _, branch := range sorted {
		b, exists := g.GetBranch(branch)
		if !exists {
			continue
		}

		actualHead, err := gitRunner.GetCommitSHA(branch)
		if err == nil && actualHead == b.BaseSHA {
			continue
		}

		isMerged := false

		isAnc, err := gitRunner.IsAncestor(branch, "origin/"+trunk)
		if err == nil && isAnc {
			isMerged = true
		}

		if !isMerged && !gitRunner.RemoteBranchExists(branch) {
			diffEmpty, err := gitRunner.DiffIsEmpty("origin/"+trunk, branch)
			if err == nil && diffEmpty {
				isMerged = true
			}
		}

		if isMerged {
			merged = append(merged, branch)
		}
	}
	return merged, nil
}

// reconcileSharedGraph merges the fetched remote graph with the local graph.
func reconcileSharedGraph(g *graph.Graph, gitRunner *git.Runner) {
	remoteData, err := gitRunner.ReadBlobRef(graph.SharedGraphRef)
	if err != nil {
		return
	}
	var remoteGraph graph.Graph
	if json.Unmarshal(remoteData, &remoteGraph) != nil {
		return
	}
	if remoteGraph.Branches == nil {
		remoteGraph.Branches = make(map[string]*graph.Branch)
	}

	reconciled := ReconcileGraphs(g, &remoteGraph, gitRunner)

	g.Root = reconciled.Root
	g.Branches = reconciled.Branches
	g.Version = reconciled.Version
}

// ReconcileGraphs performs the union merge: start with remote, add local-only branches,
// and for shared branches keep local HeadSHA if local is ahead.
func ReconcileGraphs(local *graph.Graph, remote *graph.Graph, gitRunner *git.Runner) *graph.Graph {
	result := &graph.Graph{
		Version:  remote.Version,
		Root:     remote.Root,
		Branches: make(map[string]*graph.Branch),
	}

	for name, branch := range remote.Branches {
		if exists, _ := gitRunner.BranchExists(name); exists {
			result.Branches[name] = &graph.Branch{
				Name:    branch.Name,
				Parent:  branch.Parent,
				BaseSHA: branch.BaseSHA,
				HeadSHA: branch.HeadSHA,
			}
		}
	}

	for name, localBranch := range local.Branches {
		if _, inRemote := remote.Branches[name]; !inRemote {
			if exists, _ := gitRunner.BranchExists(name); exists {
				result.Branches[name] = &graph.Branch{
					Name:    localBranch.Name,
					Parent:  localBranch.Parent,
					BaseSHA: localBranch.BaseSHA,
					HeadSHA: localBranch.HeadSHA,
				}
			}
		} else {
			remoteBranch := remote.Branches[name]
			if localBranch.HeadSHA != remoteBranch.HeadSHA {
				isAnc, err := gitRunner.IsAncestor(remoteBranch.HeadSHA, localBranch.HeadSHA)
				if err == nil && isAnc {
					result.Branches[name].HeadSHA = localBranch.HeadSHA
				}
			}
		}
	}

	return result
}
