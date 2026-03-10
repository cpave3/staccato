package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/attach"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/output"
	"github.com/cpave3/staccato/pkg/restack"
)

// attachCandidate represents a potential parent branch for attach TUI
type attachCandidate struct {
	name      string
	isCurrent bool
}

func (c attachCandidate) FilterValue() string { return c.name }

// attachTUI is the Bubble Tea model for interactive attachment
type attachTUI struct {
	list           list.Model
	git            *git.Runner
	graph          *graph.Graph
	branchToAttach string
	allCandidates  []attachCandidate // full unfiltered list
	candidates     []attachCandidate // currently visible (filtered) list
	selected       string
	setAsRoot      bool
	searchMode     bool
	searchQuery    string
	quitting       bool
	viewHeight     int // terminal height for viewport
}

func (a attachTUI) Init() tea.Cmd {
	return nil
}

func (a attachTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle search mode
		if a.searchMode {
			switch msg.Type {
			case tea.KeyEsc:
				a.searchMode = false
				a.searchQuery = ""
				a.applyFilter()
				return a, nil
			case tea.KeyEnter:
				a.searchMode = false
				idx := a.list.Index()
				if idx >= 0 && idx < len(a.candidates) {
					a.selected = a.candidates[idx].name
					a.quitting = true
					return a, tea.Quit
				}
				return a, nil
			case tea.KeyBackspace:
				if len(a.searchQuery) > 0 {
					a.searchQuery = a.searchQuery[:len(a.searchQuery)-1]
					a.applyFilter()
				}
				return a, nil
			case tea.KeyUp, tea.KeyDown:
				// Pass arrow keys to the list for navigation
				var cmd tea.Cmd
				a.list, cmd = a.list.Update(msg)
				return a, cmd
			default:
				if len(msg.String()) == 1 {
					a.searchQuery += msg.String()
					a.applyFilter()
				}
				return a, nil
			}
		}

		switch msg.String() {
		case "q", "esc":
			a.quitting = true
			return a, tea.Quit
		case "/":
			a.searchMode = true
			a.searchQuery = ""
			a.applyFilter()
			return a, nil
		case "r":
			idx := a.list.Index()
			if idx >= 0 && idx < len(a.candidates) {
				a.selected = a.candidates[idx].name
				a.setAsRoot = true
				a.quitting = true
				return a, tea.Quit
			}
		case "enter":
			idx := a.list.Index()
			if idx >= 0 && idx < len(a.candidates) {
				a.selected = a.candidates[idx].name
				a.quitting = true
				return a, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		a.list.SetWidth(msg.Width)
		a.viewHeight = msg.Height - 5 // header + footer
		a.list.SetHeight(a.viewHeight)
		return a, nil
	}

	var cmd tea.Cmd
	a.list, cmd = a.list.Update(msg)
	return a, cmd
}

// applyFilter rebuilds candidates and list items based on search query.
// When query is empty, restores all candidates.
func (a *attachTUI) applyFilter() {
	source := a.allCandidates
	if len(source) == 0 {
		source = a.candidates
	}

	if a.searchQuery == "" {
		// Restore full list
		a.candidates = source
	} else {
		query := strings.ToLower(a.searchQuery)
		var filtered []attachCandidate
		for _, c := range source {
			if strings.Contains(strings.ToLower(c.name), query) {
				filtered = append(filtered, c)
			}
		}
		a.candidates = filtered
	}

	// Rebuild list items
	var items []list.Item
	for _, c := range a.candidates {
		items = append(items, c)
	}
	a.list.SetItems(items)
	a.list.Select(0)
}

func (a attachTUI) View() string {
	if a.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render(fmt.Sprintf("  Attach '%s'", a.branchToAttach)) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  Select a parent branch from the stack") + "\n\n")

	selectedIdx := a.list.Index()

	// Viewport: determine which items to render
	maxVisible := a.viewHeight
	if maxVisible <= 0 {
		maxVisible = 15 // default if no window size received
	}
	total := len(a.candidates)

	startIdx := 0
	if total > maxVisible {
		// Center the selected item in the viewport
		startIdx = selectedIdx - maxVisible/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx+maxVisible > total {
			startIdx = total - maxVisible
		}
	}
	endIdx := startIdx + maxVisible
	if endIdx > total {
		endIdx = total
	}

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(fmt.Sprintf("  ↑ %d more", startIdx)) + "\n")
	}

	for i := startIdx; i < endIdx; i++ {
		c := a.candidates[i]
		icon := "○"
		if c.isCurrent {
			icon = "●"
		}
		line := fmt.Sprintf("%s %s", icon, c.name)

		if i == selectedIdx {
			if c.isCurrent {
				line = "> " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575")).Render(line)
			} else {
				line = "> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(line)
			}
		} else {
			if c.isCurrent {
				line = "  " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575")).Render(line)
			} else {
				line = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(line)
			}
		}
		b.WriteString(line + "\n")
	}

	// Show scroll indicator at bottom
	if endIdx < total {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(fmt.Sprintf("  ↓ %d more", total-endIdx)) + "\n")
	}

	if a.searchMode {
		b.WriteString("\n" + lipgloss.NewStyle().Background(lipgloss.Color("#333333")).Foreground(lipgloss.Color("#FFFFFF")).Render(fmt.Sprintf("  /%s", a.searchQuery)))
		if len(a.candidates) > 0 {
			fmt.Fprintf(&b, "  [%d matches]", len(a.candidates))
		}
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  ↑↓ navigate  / search  enter select  r set-root  q quit"))

	return b.String()
}

// attachInteractive launches the TUI to attach a branch, recursing up the chain as needed.
// Unlike attachRecursively, this always shows the TUI even if the branch is already tracked.
func attachInteractive(g *graph.Graph, gitRunner *git.Runner, repoPath string, attacher *attach.Attacher, branchToAttach string, printer *output.Printer) error {
	return doAttachRecursively(g, gitRunner, repoPath, attacher, branchToAttach, false, printer)
}

// attachRecursively attaches a branch, stopping if already tracked (used for recursive parent attachment).
func attachRecursively(g *graph.Graph, gitRunner *git.Runner, repoPath string, attacher *attach.Attacher, branchToAttach string, printer *output.Printer) error {
	return doAttachRecursively(g, gitRunner, repoPath, attacher, branchToAttach, true, printer)
}

func doAttachRecursively(g *graph.Graph, gitRunner *git.Runner, repoPath string, attacher *attach.Attacher, branchToAttach string, stopIfTracked bool, printer *output.Printer) error {
	if stopIfTracked && attacher.IsBranchInGraph(g, branchToAttach) {
		return nil // Already attached, stop recursion
	}

	currentBranch, _ := gitRunner.GetCurrentBranch()
	var candidates []attachCandidate

	// Get ALL branches from git (not just those in graph)
	allBranches, err := gitRunner.GetAllBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	// Add branches as candidates, filtering out already-stacked branches during recursive attach
	seen := make(map[string]bool)
	for _, name := range allBranches {
		if name == branchToAttach {
			continue // Don't include the branch being attached
		}
		// During recursive attach, hide branches already in the graph (except root)
		if stopIfTracked && name != g.Root {
			if _, inGraph := g.Branches[name]; inGraph {
				continue
			}
		}
		if !seen[name] {
			seen[name] = true
			candidates = append(candidates, attachCandidate{
				name:      name,
				isCurrent: name == currentBranch,
			})
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no existing branches to use as parent for '%s'", branchToAttach)
	}

	// Create list items
	var items []list.Item
	for _, c := range candidates {
		items = append(items, c)
	}

	// Create list
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowHelp(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)

	// Create model
	model := &attachTUI{
		list:           l,
		git:            gitRunner,
		graph:          g,
		branchToAttach: branchToAttach,
		allCandidates:  candidates,
		candidates:     candidates,
	}

	// Run TUI
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running attach: %w", err)
	}

	// Handle result
	if m, ok := finalModel.(attachTUI); ok && m.selected != "" {
		// Set as root if: user pressed 'r' or branch is a trunk name
		if m.setAsRoot || isTrunkBranch(m.selected) {
			g.Root = m.selected
			if err := saveContext(g, repoPath, gitRunner); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}
			printer.Success("Set '%s' as stack root", m.selected)

			// Now attach the branch under the new root
			err := attacher.AttachBranch(g, branchToAttach, m.selected)
			if err != nil {
				return fmt.Errorf("failed to attach branch: %w", err)
			}
			if err := saveContext(g, repoPath, gitRunner); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}
			printer.Success("Attached '%s' as child of '%s'", branchToAttach, m.selected)
			return nil
		}

		// RECURSIVE STEP: If the selected parent isn't in the graph yet, attach it first
		if !attacher.IsBranchInGraph(g, m.selected) && m.selected != g.Root {
			printer.Println("\nParent '%s' is not yet in the stack. Attaching it first...", m.selected)
			if err := attachRecursively(g, gitRunner, repoPath, attacher, m.selected, printer); err != nil {
				return fmt.Errorf("failed to attach parent '%s': %w", m.selected, err)
			}
		}

		// Attach the branch (parent is now guaranteed to be in the graph)
		err := attacher.AttachBranch(g, branchToAttach, m.selected)
		if err != nil {
			return fmt.Errorf("failed to attach branch: %w", err)
		}

		if err := saveContext(g, repoPath, gitRunner); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}

		printer.Success("Attached '%s' as child of '%s'", branchToAttach, m.selected)
	}

	return nil
}

func attachCmd() *cobra.Command {
	var autoSelect bool
	var parentFlag string

	cmd := &cobra.Command{
		Use:   "attach [branch-name]",
		Short: "Adopt an unknown branch into the stack",
		Long: `Attaches a branch that was created outside of st to the stack graph.
If no branch is specified, uses the current branch.
Opens an interactive TUI to select the parent branch (use --auto to skip TUI).
Use --parent to specify the parent directly. Works for both new and already-tracked branches.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			checkStaleness(g, gitRunner, printer)

			var branchToAttach string
			if len(args) > 0 {
				branchToAttach = args[0]
			} else {
				branchToAttach, _ = gitRunner.GetCurrentBranch()
			}

			attacher := attach.NewAttacher(gitRunner, nil)

			// If --parent flag is set, use direct parent specification
			if parentFlag != "" {
				return attachWithParent(g, gitRunner, repoPath, attacher, branchToAttach, parentFlag, printer)
			}

			// If --auto flag is set, use auto-attach mode
			if autoSelect {
				err = attacher.AutoAttach(g, branchToAttach, true)
				if err != nil {
					return fmt.Errorf("failed to auto-attach: %w", err)
				}
				if err := saveContext(g, repoPath, gitRunner); err != nil {
					return fmt.Errorf("failed to save graph: %w", err)
				}
				return nil
			}

			// Interactive TUI mode with recursive attachment
			// Always launch TUI — even for tracked branches (allows building/modifying stack interactively)
			return attachInteractive(g, gitRunner, repoPath, attacher, branchToAttach, printer)
		},
	}

	cmd.Flags().BoolVar(&autoSelect, "auto", false, "Automatically select the best parent candidate (skip TUI)")
	cmd.Flags().StringVar(&parentFlag, "parent", "", "Specify parent branch directly (skip TUI)")

	return cmd
}

func attachWithParent(g *graph.Graph, gitRunner *git.Runner, repoPath string, attacher *attach.Attacher, branchToAttach, parent string, printer *output.Printer) error {
	// Validate parent exists in graph or is root
	if parent != g.Root {
		if _, exists := g.GetBranch(parent); !exists {
			// If the parent is a trunk branch and exists in git, auto-set as root
			if isTrunkBranch(parent) {
				exists, err := gitRunner.BranchExists(parent)
				if err == nil && exists {
					g.Root = parent
					printer.Success("Set '%s' as stack root", parent)
				} else {
					return fmt.Errorf("parent '%s' is not in the stack", parent)
				}
			} else {
				return fmt.Errorf("parent '%s' is not in the stack", parent)
			}
		}
	}

	isRelocate := attacher.IsBranchInGraph(g, branchToAttach)

	if isRelocate {
		currentParent := g.Branches[branchToAttach].Parent
		if currentParent == parent {
			printer.Println("'%s' already has parent '%s'", branchToAttach, parent)
			return nil
		}

		warnDirtyTree(gitRunner, printer)

		// Create backups before any destructive operations
		backupMgr := backup.NewManager(gitRunner, repoPath)
		downstreamBranches := restack.GetDownstreamBranches(g, branchToAttach)
		affectedBranches := append([]string{branchToAttach}, downstreamBranches...)

		backups, err := backupMgr.CreateBackupsForStack(affectedBranches)
		if err != nil {
			return fmt.Errorf("failed to create backups: %w", err)
		}

		// Update parent in graph
		g.Branches[branchToAttach].Parent = parent

		// Rebase the branch itself onto the new parent
		if err := gitRunner.CheckoutBranch(branchToAttach); err != nil {
			backupMgr.RestoreStack(backups)
			return fmt.Errorf("failed to checkout %s: %w", branchToAttach, err)
		}
		if err := gitRunner.Rebase(parent); err != nil {
			backupMgr.RestoreStack(backups)
			return fmt.Errorf("failed to rebase %s onto %s: %w", branchToAttach, parent, err)
		}

		// Update branch metadata
		newBaseSHA, _ := gitRunner.GetCommitSHA(parent)
		newHeadSHA, _ := gitRunner.GetCommitSHA(branchToAttach)
		g.UpdateBranch(branchToAttach, newBaseSHA, newHeadSHA)

		if err := saveContext(g, repoPath, gitRunner); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}

		// Restack downstream branches if any
		if len(downstreamBranches) > 0 {
			engine := restack.NewEngine(gitRunner, backupMgr)
			result, err := engine.Restack(g, branchToAttach)
			if err != nil {
				if result.Conflicts {
					return fmt.Errorf("conflict during restack at '%s'", result.ConflictsAt)
				}
				backupMgr.RestoreStack(backups)
				return fmt.Errorf("restack failed: %w", err)
			}
		}

		backupMgr.CleanupStackBackups(affectedBranches)

		printer.Success("Relocated '%s' under '%s'", branchToAttach, parent)
		return nil
	}

	// New attachment
	if err := attacher.AttachBranch(g, branchToAttach, parent); err != nil {
		return fmt.Errorf("failed to attach branch: %w", err)
	}

	if err := saveContext(g, repoPath, gitRunner); err != nil {
		return fmt.Errorf("failed to save graph: %w", err)
	}

	printer.Success("Attached '%s' as child of '%s'", branchToAttach, parent)
	return nil
}
