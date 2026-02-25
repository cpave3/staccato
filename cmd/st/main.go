package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/user/st/pkg/attach"
	"github.com/user/st/pkg/backup"
	"github.com/user/st/pkg/git"
	"github.com/user/st/pkg/graph"
	"github.com/user/st/pkg/output"
	"github.com/user/st/pkg/restack"
)

var (
	verbose bool
	rootCmd *cobra.Command
)

// TUI Types for interactive commands

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
	searchMode     bool
	searchQuery    string
	matches        []int
	matchIndex     int
	quitting       bool
	err            error
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
			b.WriteString(fmt.Sprintf("  [%d/%d matches]", a.matchIndex+1, len(a.matches)))
		}
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  ↑↓ navigate  / search  enter select  q quit"))

	return b.String()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	bannerLines := []string{
		` ███████╗ ████████╗  █████╗   ██████╗  ██████╗  █████╗  ████████╗  ██████╗`,
		` ██╔════╝ ╚══██╔══╝ ██╔══██╗ ██╔════╝ ██╔════╝ ██╔══██╗ ╚══██╔══╝ ██╔═══██╗`,
		` ███████╗    ██║    ███████║ ██║      ██║      ███████║    ██║    ██║   ██║`,
		` ╚════██║    ██║    ██╔══██║ ██║      ██║      ██╔══██║    ██║    ██║   ██║`,
		` ███████║    ██║    ██║  ██║ ╚██████╗ ╚██████╗ ██║  ██║    ██║    ╚██████╔╝`,
		` ╚══════╝    ╚═╝    ╚═╝  ╚═╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═╝    ╚═╝     ╚═════╝`,
	}
	// Find the longest line (in runes) for gradient calculation
	maxLen := 0
	for _, l := range bannerLines {
		if n := len([]rune(l)); n > maxLen {
			maxLen = n
		}
	}
	// Horizontal gradient from #524180 to #ad8499
	r0, g0, b0 := 0x52, 0x41, 0x80
	r1, g1, b1 := 0xad, 0x84, 0x99
	var banner string
	for _, line := range bannerLines {
		runes := []rune(line)
		for i, ch := range runes {
			t := float64(i) / float64(maxLen-1)
			r := int(float64(r0) + t*float64(r1-r0))
			g := int(float64(g0) + t*float64(g1-g0))
			b := int(float64(b0) + t*float64(b1-b0))
			color := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			banner += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(string(ch))
		}
		banner += "\n"
	}

	rootCmd = &cobra.Command{
		Use:   "st",
		Short: "A deterministic, offline-first Git stack management CLI",
		Long: "\n" + banner + "\n\n" +
			"Staccato provides branch-level stacking with deterministic restacking, automatic backups,\n" +
			"and lazy attachment for retrofitting existing branches.",
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(appendCmd())
	rootCmd.AddCommand(insertCmd())
	rootCmd.AddCommand(restackCmd())
	rootCmd.AddCommand(continueCmd())
	rootCmd.AddCommand(attachCmd())
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(syncCmd())
	rootCmd.AddCommand(logCmd())
	rootCmd.AddCommand(switchCmdFunc())
	rootCmd.AddCommand(backupCmd())
}

// getContext loads the graph and git runner for commands
func getContext() (*graph.Graph, *git.Runner, *output.Printer, string, error) {
	printer := output.NewPrinter(verbose)

	// Find git repository root
	gitRunner := git.NewRunner("")
	repoPath, err := gitRunner.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("not a git repository")
	}

	gitRunner = git.NewRunner(repoPath)

	// Load or create graph
	graphPath := filepath.Join(repoPath, graph.GetDefaultGraphPath())
	g, err := graph.LoadGraph(graphPath)
	if err != nil {
		// Graph doesn't exist yet, try to create with current branch as root
		currentBranch, err := gitRunner.GetCurrentBranch()
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("failed to get current branch: %w", err)
		}
		g = graph.NewGraph(currentBranch)
	}

	return g, gitRunner, printer, repoPath, nil
}

// saveContext saves the graph
func saveContext(g *graph.Graph, repoPath string) error {
	graphPath := filepath.Join(repoPath, graph.GetDefaultGraphPath())
	return g.Save(graphPath)
}

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <branch-name>",
		Short: "Create a new branch from the current root/trunk",
		Long:  "Creates a new branch from the current root branch and adds it to the stack graph.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Create branch from root
			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(g.Root)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add to graph
			g.AddBranch(branchName, g.Root, baseSHA, headSHA)

			// Save graph
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchCreated(branchName, g.Root)

			return nil
		},
	}
}

func appendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "append <branch-name>",
		Short: "Create a child branch from the current branch",
		Long:  "Creates a new branch from the current branch and adds it as a child in the stack.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			parentBranch, _ := git.GetCurrentBranch()

			// If current branch is not in graph and not root, we need to attach it first
			if parentBranch != g.Root {
				if _, exists := g.GetBranch(parentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack. Run 'st attach' first", parentBranch)
				}
			}

			// Create branch
			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(parentBranch)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add to graph
			g.AddBranch(branchName, parentBranch, baseSHA, headSHA)

			// Save graph
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchCreated(branchName, parentBranch)

			return nil
		},
	}
}

func insertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "insert <branch-name>",
		Short: "Insert a branch before the current branch",
		Long: `Inserts a new branch before the current branch in the stack.
The current branch and all downstream branches will be reparented and restacked.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()

			// Get current branch's parent
			currentBranchInfo, exists := g.GetBranch(currentBranch)
			if !exists {
				return fmt.Errorf("current branch '%s' is not in the stack", currentBranch)
			}

			oldParent := currentBranchInfo.Parent

			// Create backup manager
			backupMgr := backup.NewManager(git, repoPath)

			// Create backups of all affected branches
			downstreamBranches := restack.GetDownstreamBranches(g, currentBranch)
			affectedBranches := append([]string{currentBranch}, downstreamBranches...)

			backups, err := backupMgr.CreateBackupsForStack(affectedBranches)
			if err != nil {
				return fmt.Errorf("failed to create backups: %w", err)
			}

			// Create new branch from old parent
			err = git.CheckoutBranch(oldParent)
			if err != nil {
				return fmt.Errorf("failed to checkout parent: %w", err)
			}

			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(oldParent)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add new branch to graph
			g.AddBranch(branchName, oldParent, baseSHA, headSHA)

			// Reparent current branch to new branch
			g.Branches[currentBranch].Parent = branchName

			// Save graph first
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchInserted(branchName, currentBranch)

			// Now restack downstream branches
			printer.Println("Restacking downstream branches...")

			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.Restack(g, branchName)
			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack")
				}

				// Restore backups on error
				printer.Error("Restack failed, restoring from backups...")
				backupMgr.RestoreStack(backups)
				return err
			}

			// Cleanup backups
			backupMgr.CleanupStackBackups(affectedBranches)

			printer.RestackComplete(len(result.Completed))

			// Checkout the newly inserted branch
			git.CheckoutBranch(branchName)

			return nil
		},
	}
}

func restackCmd() *cobra.Command {
	var toCurrent bool
	cmd := &cobra.Command{
		Use:   "restack",
		Short: "Restack the entire stack",
		Long: `Rebases all branches in the stack onto their parents in topological order.
Creates backups before any destructive operations. Stops on first conflict.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()

			// Check if current branch is in the stack
			if currentBranch != g.Root {
				if _, exists := g.GetBranch(currentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack", currentBranch)
				}
			}

			// Find root of current stack
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Get only the current lineage (not all branches under root)
			lineageBranches := restack.GetLineage(g, currentBranch)

			// Check if we're at the tip
			if !restack.IsBranchAtTip(g, currentBranch) {
				if !toCurrent {
					printer.Warning("You are not at the tip of your stack lineage")
					printer.Println("  Use --to-current to restack only up to '%s'", currentBranch)
					printer.Println("  Or switch to the tip branch and run 'st restack'")
					return fmt.Errorf("specify --to-current or switch to the tip branch")
				}
				lineageBranches = restack.GetAncestors(g, currentBranch)
			}

			printer.RestackStart(currentBranch)

			// Create backup manager
			backupMgr := backup.NewManager(git, repoPath)

			// Perform restack for this lineage only
			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.RestackLineage(g, currentBranch, lineageBranches)

			// Save graph state (even if there was an error)
			saveContext(g, repoPath)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack - resolve and run 'st continue'")
				}

				// Check if we should restore
				if len(result.Backups) > 0 {
					printer.Error("Restack failed, run 'st restore' to recover")
				}
				return err
			}

			// Cleanup backups on success
			if len(result.Backups) > 0 {
				backupMgr.CleanupStackBackups(lineageBranches)
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
	cmd.Flags().BoolVar(&toCurrent, "to-current", false, "Restack only up to the current branch")
	return cmd
}

func continueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Resume restack after conflict resolution",
		Long:  "Continues a restack operation that was paused due to conflicts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Check if rebase is in progress
			inProgress, err := git.IsRebaseInProgress()
			if err != nil {
				return fmt.Errorf("failed to check rebase status: %w", err)
			}

			if !inProgress {
				return fmt.Errorf("no rebase in progress - nothing to continue")
			}

			backupMgr := backup.NewManager(git, repoPath)
			engine := restack.NewEngine(git, backupMgr)

			currentBranch, _ := git.GetCurrentBranch()
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Continue the restack
			result, err := engine.Continue(g, rootBranch, nil)

			// Save graph state
			saveContext(g, repoPath)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("still have conflicts to resolve")
				}
				return err
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
}

// attachRecursively attaches a branch and continues up the chain until reaching root or tracked branch
func attachRecursively(g *graph.Graph, gitRunner *git.Runner, repoPath string, attacher *attach.Attacher, branchToAttach string) error {
	// Check if already in graph
	if attacher.IsBranchInGraph(g, branchToAttach) {
		return nil // Already attached, stop recursion
	}

	currentBranch, _ := gitRunner.GetCurrentBranch()
	var candidates []attachCandidate

	// Get ALL branches from git (not just those in graph)
	allBranches, err := gitRunner.GetAllBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	// Add all branches as candidates
	seen := make(map[string]bool)
	for _, name := range allBranches {
		if name == branchToAttach {
			continue // Don't include the branch being attached
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
		// RECURSIVE STEP: If the selected parent isn't in the graph yet, attach it first
		if !attacher.IsBranchInGraph(g, m.selected) && m.selected != g.Root {
			fmt.Printf("\nParent '%s' is not yet in the stack. Attaching it first...\n", m.selected)
			if err := attachRecursively(g, gitRunner, repoPath, attacher, m.selected); err != nil {
				return fmt.Errorf("failed to attach parent '%s': %w", m.selected, err)
			}
		}

		// Attach the branch (parent is now guaranteed to be in the graph)
		err := attacher.AttachBranch(g, branchToAttach, m.selected)
		if err != nil {
			return fmt.Errorf("failed to attach branch: %w", err)
		}

		if err := saveContext(g, repoPath); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}

		fmt.Printf("✔ Attached '%s' as child of '%s'\n", branchToAttach, m.selected)
	}

	return nil
}

func attachCmd() *cobra.Command {
	var autoSelect bool

	cmd := &cobra.Command{
		Use:   "attach [branch-name]",
		Short: "Adopt an unknown branch into the stack",
		Long: `Attaches a branch that was created outside of st to the stack graph.
If no branch is specified, uses the current branch.
Opens an interactive TUI to select the parent branch (use --auto to skip TUI).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, repoPath, err := getContext()
			if err != nil {
				return err
			}

			var branchToAttach string
			if len(args) > 0 {
				branchToAttach = args[0]
			} else {
				branchToAttach, _ = gitRunner.GetCurrentBranch()
			}

			attacher := attach.NewAttacher(gitRunner, nil)

			// If --auto flag is set, use auto-attach mode
			if autoSelect {
				err = attacher.AutoAttach(g, branchToAttach, true)
				if err != nil {
					return fmt.Errorf("failed to auto-attach: %w", err)
				}
				if err := saveContext(g, repoPath); err != nil {
					return fmt.Errorf("failed to save graph: %w", err)
				}
				return nil
			}

			// Interactive TUI mode with recursive attachment
			return attachRecursively(g, gitRunner, repoPath, attacher, branchToAttach)
		},
	}

	cmd.Flags().BoolVar(&autoSelect, "auto", false, "Automatically select the best parent candidate (skip TUI)")

	return cmd
}

func restoreCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "restore [branch-name]",
		Short: "Restore branch(es) from backup",
		Long: `Restores a branch or all branches from their backups.
Use this to recover from failed restack operations.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			backupMgr := backup.NewManager(git, repoPath)

			if all {
				// Restore all branches in stack
				currentBranch, _ := git.GetCurrentBranch()
				attacher := attach.NewAttacher(git, printer)
				rootBranch := attacher.FindRoot(g, currentBranch)
				if rootBranch == "" {
					rootBranch = g.Root
				}

				branches := restack.GetStackBranches(g, rootBranch)
				for _, branch := range branches {
					backups, _ := backupMgr.ListBackups(branch)
					if len(backups) > 0 {
						err := backupMgr.RestoreBackup(branch, backups[0])
						if err != nil {
							printer.Error("Failed to restore %s: %v", branch, err)
						} else {
							printer.BackupRestored(branch)
						}
					}
				}
			} else {
				// Restore specific branch
				var branchName string
				if len(args) > 0 {
					branchName = args[0]
				} else {
					branchName, _ = git.GetCurrentBranch()
				}

				backups, err := backupMgr.ListBackups(branchName)
				if err != nil || len(backups) == 0 {
					return fmt.Errorf("no backups found for branch '%s'", branchName)
				}

				err = backupMgr.RestoreBackup(branchName, backups[0])
				if err != nil {
					return fmt.Errorf("failed to restore backup: %w", err)
				}

				printer.BackupRestored(branchName)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Restore all branches in the stack")

	return cmd
}

func syncCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Push branches to remote",
		Long: `Explicitly pushes all branches in the stack to the remote.
This is an offline-first tool, so push is always explicit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			if dryRun {
				printer.DryRunNotice()
			}

			// Check if we have a remote
			hasRemote, _ := git.HasRemote()
			if !hasRemote {
				return fmt.Errorf("no remote configured")
			}

			currentBranch, _ := git.GetCurrentBranch()
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Get all branches in stack (excluding root)
			branches := restack.GetStackBranches(g, rootBranch)

			// Push each branch
			pushedCount := 0
			for _, branch := range branches {
				if branch == g.Root {
					continue
				}

				if dryRun {
					printer.Info("Would push: %s", branch)
				} else {
					err := git.Push(branch, false)
					if err != nil {
						printer.Error("Failed to push %s: %v", branch, err)
					} else {
						printer.Info("Pushed: %s", branch)
						pushedCount++
					}
				}
			}

			printer.SyncComplete(pushedCount, dryRun)

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pushed without pushing")

	return cmd
}

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create a manual backup of all stack branches",
		Long: `Creates a manual snapshot of every branch in the stack (excluding the root).
Unlike automatic backups created during restack/insert, manual backups persist
until you delete them. Backup branches are named backups/<timestamp>/<branch>.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Collect branch names excluding root
			var branches []string
			for name := range g.Branches {
				if name != g.Root {
					branches = append(branches, name)
				}
			}

			if len(branches) == 0 {
				return fmt.Errorf("no branches in the stack to backup")
			}

			backupMgr := backup.NewManager(gitRunner, repoPath)
			timestamp, err := backupMgr.CreateManualBackup(branches)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			printer.Info("Backup created: %s (%d branches)", timestamp, len(branches))
			return nil
		},
	}
}

func logCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Display stack hierarchy",
		Long:  "Shows the current stack structure as a tree.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()

			printer.StackLog(g, currentBranch)

			return nil
		},
	}
}
