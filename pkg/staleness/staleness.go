package staleness

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

// DetectedSignal represents a single staleness indicator.
type DetectedSignal struct {
	Branch  string
	Message string
}

// Report holds the results of a staleness check.
type Report struct {
	Signals []DetectedSignal
}

// IsStale returns true if any staleness signals were detected.
func (r *Report) IsStale() bool {
	return len(r.Signals) > 0
}

// Check performs a fast, offline staleness check using existing remote tracking refs.
// No network calls — it only compares local refs against whatever was last fetched.
func Check(g *graph.Graph, gitRunner *git.Runner) *Report {
	report := &Report{}

	// Check if trunk is behind origin/<trunk>
	trunk := g.Root
	trunkSHA, err := gitRunner.GetCommitSHA(trunk)
	if err != nil {
		return report
	}
	remoteTrunk := "origin/" + trunk
	remoteTrunkSHA, err := gitRunner.GetCommitSHA(remoteTrunk)
	if err != nil {
		// No remote tracking ref for trunk — skip
		return report
	}

	if trunkSHA != remoteTrunkSHA {
		isAnc, err := gitRunner.IsAncestor(trunk, remoteTrunk)
		if err == nil && isAnc {
			report.Signals = append(report.Signals, DetectedSignal{
				Branch:  trunk,
				Message: fmt.Sprintf("'%s' is behind 'origin/%s'", trunk, trunk),
			})
		}
	}

	// Check for graph branches whose remote tracking ref is gone
	for name := range g.Branches {
		if !gitRunner.RemoteBranchExists(name) {
			report.Signals = append(report.Signals, DetectedSignal{
				Branch:  name,
				Message: fmt.Sprintf("'%s' has been deleted on remote", name),
			})
		}
	}

	return report
}
