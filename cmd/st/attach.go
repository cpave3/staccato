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
	candidates     []attachCandidate
	selected       string
	setAsRoot      bool
	searchMode     bool
	searchQuery    string
	matches        []int
	matchIndex     int
	quitting       bool
}

func (a attachTUI) Init() tea.Cmd {
	return nil
}

func (a attachTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle search mode
		if a.searchMode {
			switch msg.String() {
			case "esc":
				a.searchMode = false
				a.searchQuery = ""
				a.updateMatches()
				return a, nil
			case "enter":
				a.searchMode = false
				if len(a.matches) > 0 {
					a.matchIndex = 0
					idx := a.matches[a.matchIndex]
					a.list.Select(idx)
				}
				return a, nil
			case "backspace":
				if len(a.searchQuery) > 0 {
					a.searchQuery = a.searchQuery[:len(a.searchQuery)-1]
					a.updateMatches()
				}
				return a, nil
			default:
				if len(msg.String()) == 1 {
					a.searchQuery += msg.String()
					a.updateMatches()
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
			a.updateMatches()
			return a, nil
		case "r":
			// Set selected branch as root (stop recursion here)
			idx := a.list.Index()
			if idx >= 0 && idx < len(a.candidates) {
				a.selected = a.candidates[idx].name
				a.setAsRoot = true
				a.quitting = true
				return a, tea.Quit
			}
		case "enter":
			// Use index instead of type assertion (Bubble Tea list doesn't preserve custom types)
			idx := a.list.Index()
			if idx >= 0 && idx < len(a.candidates) {
				a.selected = a.candidates[idx].name
				a.quitting = true
				return a, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		a.list.SetWidth(msg.Width)
		a.list.SetHeight(msg.Height - 5)
		return a, nil
	}

	var cmd tea.Cmd
	a.list, cmd = a.list.Update(msg)
	return a, cmd
}

func (a *attachTUI) updateMatches() {
	a.matches = []int{}
	if a.searchQuery == "" {
		return
	}
	query := strings.ToLower(a.searchQuery)
	for i, item := range a.list.Items() {
		if c, ok := item.(attachCandidate); ok {
			if strings.Contains(strings.ToLower(c.name), query) {
				a.matches = append(a.matches, i)
			}
		}
	}
}

func (a attachTUI) View() string {
	if a.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render(fmt.Sprintf("  Attach '%s'", a.branchToAttach)) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  Select a parent branch from the stack") + "\n\n")

	items := a.list.Items()
	selectedIdx := a.list.Index()

	for i, item := range items {
		c := item.(attachCandidate)
		icon := "○"
		if c.isCurrent {
			icon = "●"
		}
		line := fmt.Sprintf("%s %s", icon, c.name)

		isMatch := false
		if a.searchQuery != "" {
			for _, matchIdx := range a.matches {
				if matchIdx == i {
					isMatch = true
					break
				}
			}
		}

		if i == selectedIdx {
			if c.isCurrent {
				line = "> " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575")).Render(line)
			} else if isMatch {
				line = "> " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(line)
			} else {
				line = "> " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(line)
			}
		} else {
			if a.searchMode && !isMatch && a.searchQuery != "" {
				line = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(line)
			} else if c.isCurrent {
				line = "  " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575")).Render(line)
			} else if isMatch {
				line = "  " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(line)
			} else {
				line = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(line)
			}
		}
		b.WriteString(line + "\n")
	}

	if a.searchMode {
		b.WriteString("\n" + lipgloss.NewStyle().Background(lipgloss.Color("#333333")).Foreground(lipgloss.Color("#FFFFFF")).Render(fmt.Sprintf("  /%s", a.searchQuery)))
		if len(a.matches) > 0 {
			fmt.Fprintf(&b, "  [%d/%d matches]", a.matchIndex+1, len(a.matches))
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
