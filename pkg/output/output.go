package output

import (
	"fmt"
	"strings"

	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/graph"
)

// Symbols for CLI output
const (
	SuccessIcon  = "✔"
	WarningIcon  = "⚠"
	ErrorIcon    = "✘"
	InfoIcon     = "ℹ"
	BranchIcon   = "○"
	CurrentIcon  = "●"
	ArrowIcon    = "→"
	ConflictIcon = "✖"
	RerereIcon   = "↻"
)

// Printer handles CLI output formatting
type Printer struct {
	verbose bool
}

// NewPrinter creates a new output printer
func NewPrinter(verbose bool) *Printer {
	return &Printer{verbose: verbose}
}

// SetVerbose sets the verbose mode
func (p *Printer) SetVerbose(verbose bool) {
	p.verbose = verbose
}

// Success prints a success message
func (p *Printer) Success(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", SuccessIcon, fmt.Sprintf(format, args...))
}

// Warning prints a warning message
func (p *Printer) Warning(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", WarningIcon, fmt.Sprintf(format, args...))
}

// Error prints an error message
func (p *Printer) Error(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", ErrorIcon, fmt.Sprintf(format, args...))
}

// Info prints an info message
func (p *Printer) Info(format string, args ...interface{}) {
	if p.verbose {
		fmt.Printf("%s %s\n", InfoIcon, fmt.Sprintf(format, args...))
	}
}

// Print prints a plain message
func (p *Printer) Print(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Println prints a plain message with newline
func (p *Printer) Println(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// BranchCreated prints a branch creation message
func (p *Printer) BranchCreated(name, parent string) {
	p.Success("Created branch '%s' on top of '%s'", name, parent)
}

// BranchInserted prints a branch insertion message
func (p *Printer) BranchInserted(name, before string) {
	p.Success("Inserted branch '%s' before '%s'", name, before)
}

// RestackStart prints the start of a restack operation
func (p *Printer) RestackStart(branch string) {
	p.Println("Restacking from '%s'...", branch)
}

// RestackBranch prints progress for a branch being restacked
func (p *Printer) RestackBranch(branch string) {
	p.Info("Restacking %s...", branch)
}

// RestackComplete prints completion message
func (p *Printer) RestackComplete(count int) {
	p.Success("Restacked %d branch(es)", count)
}

// ConflictDetected prints a conflict message
func (p *Printer) ConflictDetected(branch string) {
	p.Warning("Conflict while rebasing '%s'", branch)
	p.Println("  Please resolve the conflicts and run 'st continue'")
	p.Println("  Or run 'st restore' to abort and restore from backup")
}

// RerereApplied prints a rerere auto-resolution message
func (p *Printer) RerereApplied(count int) {
	p.Info("Applied %d previous conflict resolution(s) via rerere", count)
}

// BackupCreated prints a backup creation message
func (p *Printer) BackupCreated(branch, backupName string) {
	p.Info("Created backup: %s", backupName)
}

// BackupRestored prints a backup restoration message
func (p *Printer) BackupRestored(branch string) {
	p.Success("Restored '%s' from backup", branch)
}

// StackLog prints the stack hierarchy
func (p *Printer) StackLog(g *graph.Graph, currentBranch string) {
	p.Println("")
	p.Println("Stack:")

	var printBranch func(branch string, depth int)
	printBranch = func(branch string, depth int) {
		indent := strings.Repeat("  ", depth)
		icon := BranchIcon
		if branch == currentBranch {
			icon = CurrentIcon
		}
		p.Println("%s%s %s", indent, icon, branch)

		children := g.GetChildren(branch)
		for _, child := range children {
			printBranch(child.Name, depth+1)
		}
	}

	printBranch(g.Root, 0)
	p.Println("")
}

// StackStatus prints the stack hierarchy with PR status annotations
func (p *Printer) StackStatus(g *graph.Graph, currentBranch string, prStatus map[string]*forge.PRStatusInfo) {
	p.Println("")
	p.Println("Stack status:")

	var printBranch func(branch string, depth int, isRoot bool)
	printBranch = func(branch string, depth int, isRoot bool) {
		indent := strings.Repeat("  ", depth)
		icon := BranchIcon
		if branch == currentBranch {
			icon = CurrentIcon
		}

		suffix := ""
		if !isRoot {
			if info, ok := prStatus[branch]; ok && info.HasPR {
				suffix = " — " + formatPRStatus(info)
			} else {
				suffix = " — No PR"
			}
		}

		p.Println("%s%s %s%s", indent, icon, branch, suffix)

		children := g.GetChildren(branch)
		for _, child := range children {
			printBranch(child.Name, depth+1, false)
		}
	}

	printBranch(g.Root, 0, true)
	p.Println("")
}

func formatPRStatus(info *forge.PRStatusInfo) string {
	var parts []string

	switch info.State {
	case "MERGED":
		parts = append(parts, fmt.Sprintf("#%d %s Merged", info.Number, SuccessIcon))
	case "CLOSED":
		parts = append(parts, fmt.Sprintf("#%d %s Closed", info.Number, ErrorIcon))
	case "OPEN":
		if info.IsDraft {
			parts = append(parts, fmt.Sprintf("#%d Draft", info.Number))
		} else {
			switch info.ReviewStatus {
			case "APPROVED":
				parts = append(parts, fmt.Sprintf("#%d %s Approved", info.Number, SuccessIcon))
			case "CHANGES_REQUESTED":
				parts = append(parts, fmt.Sprintf("#%d %s Changes requested", info.Number, WarningIcon))
			default:
				parts = append(parts, fmt.Sprintf("#%d Review pending", info.Number))
			}
		}

		switch info.CheckStatus {
		case "fail":
			parts = append(parts, "CI "+ErrorIcon)
		case "pending":
			parts = append(parts, "CI pending")
		}
	}

	return strings.Join(parts, " | ")
}

// AttachPrompt prints the attachment prompt
func (p *Printer) AttachPrompt(branch string, candidates []string) {
	p.Warning("Branch '%s' is not in the stack graph", branch)
	p.Println("  Select a parent branch for '%s':", branch)

	for i, candidate := range candidates {
		p.Println("    [%d] %s", i+1, candidate)
	}
	p.Println("    [0] Other (specify manually)")
}

// SyncFetching prints fetching status
func (p *Printer) SyncFetching() {
	p.Println("Fetching from remote...")
}

// SyncTrunkUpdated prints trunk update message
func (p *Printer) SyncTrunkUpdated(trunk string) {
	p.Success("Updated '%s' to match remote", trunk)
}

// SyncMergedDetected prints that a merged branch was detected
func (p *Printer) SyncMergedDetected(branch string) {
	p.Println("  %s Merged: '%s'", SuccessIcon, branch)
}

// SyncBranchRemoved prints that a branch was removed from the stack
func (p *Printer) SyncBranchRemoved(branch string) {
	p.Info("Removed '%s' from stack and deleted local branch", branch)
}

// SyncNoMergedBranches prints when no merged branches are found
func (p *Printer) SyncNoMergedBranches() {
	p.Println("No merged branches detected.")
}

// SyncComplete prints sync completion summary
func (p *Printer) SyncComplete(pushed int, dryRun bool) {
	if dryRun {
		p.Info("Dry run: would have pushed %d branch(es)", pushed)
	} else {
		p.Success("Pushed %d branch(es) to remote", pushed)
	}
}

// SyncSummary prints the full sync summary
func (p *Printer) SyncSummary(merged, restacked, pushed int) {
	p.Println("")
	p.Success("Sync complete: %d merged, %d restacked, %d pushed", merged, restacked, pushed)
}

// Help prints general help
func (p *Printer) Help() {
	p.Println("st - A deterministic, offline-first Git stack management CLI")
	p.Println("")
	p.Println("Commands:")
	p.Println("  new <branch>       Create a new branch from current root/trunk")
	p.Println("  append <branch>    Create a child branch from the current branch")
	p.Println("  insert <branch>    Insert a branch before the current branch")
	p.Println("  restack            Restack the entire stack")
	p.Println("  continue           Resume restack after conflict resolution")
	p.Println("  attach             Adopt an unknown branch into the stack")
	p.Println("  restore [branch]   Restore branch(es) from backup")
	p.Println("  sync [--dry-run]   Fetch, detect merged branches, restack & push")
	p.Println("  log                Display stack hierarchy")
	p.Println("")
	p.Println("Options:")
	p.Println("  -v, --verbose      Enable verbose output")
	p.Println("  -h, --help         Show this help message")
}

// DryRunNotice prints a dry-run notice
func (p *Printer) DryRunNotice() {
	p.Warning("Running in dry-run mode (no changes will be made)")
}
