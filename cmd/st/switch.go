package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/user/st/pkg/attach"
	"github.com/user/st/pkg/git"
	"github.com/user/st/pkg/graph"
)

// branchItem represents a branch in the interactive list
type branchItem struct {
	name       string
	parent     string
	depth      int
	current    bool
	matchScore int // For search highlighting
}

func (i branchItem) FilterValue() string { return i.name }

// switchTUI is the Bubble Tea model for the switch command
type switchTUI struct {
	list        list.Model
	branches    []branchItem // Store branches separately for index-based access
	git         *git.Runner
	graph       *graph.Graph
	current     string
	searchMode  bool
	searchQuery string
	matches     []int // Indices of items matching search
	matchIndex  int   // Current match position
	selected    string
	quitting    bool
	err         error
}

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	currentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")) // Green for current

	branchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	highlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")) // Gold for matches

	searchStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

func (s switchTUI) Init() tea.Cmd {
	return func() tea.Msg { return nil }
}

func (s switchTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle search mode - only process special keys, everything else is typed
		if s.searchMode {
			switch msg.String() {
			case "esc":
				s.searchMode = false
				s.searchQuery = ""
				s.updateMatches()
				return s, nil
			case "enter":
				// Exit search mode and jump to first match
				s.searchMode = false
				if len(s.matches) > 0 {
					s.matchIndex = 0
					idx := s.matches[s.matchIndex]
					s.list.Select(idx)
				}
				return s, nil
			case "backspace":
				if len(s.searchQuery) > 0 {
					s.searchQuery = s.searchQuery[:len(s.searchQuery)-1]
					s.updateMatches()
				}
				return s, nil
			default:
				// Add any other character to search query
				if len(msg.String()) == 1 {
					s.searchQuery += msg.String()
					s.updateMatches()
				}
				return s, nil
			}
		}

		// Normal mode
		switch msg.String() {
		case "q", "esc":
			s.quitting = true
			return s, tea.Quit
		case "/":
			s.searchMode = true
			s.searchQuery = ""
			s.updateMatches()
			return s, nil
		case "enter":
			// Use index instead of type assertion (Bubble Tea list doesn't preserve custom types)
			idx := s.list.Index()
			if idx >= 0 && idx < len(s.branches) {
				s.selected = s.branches[idx].name
				s.quitting = true
				return s, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		s.list.SetWidth(msg.Width)
		s.list.SetHeight(msg.Height - 3) // Leave room for search box
		return s, nil
	}

	// Pass other messages to the list
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s *switchTUI) updateMatches() {
	s.matches = []int{}
	if s.searchQuery == "" {
		return
	}

	query := strings.ToLower(s.searchQuery)
	for i, item := range s.list.Items() {
		if bi, ok := item.(branchItem); ok {
			if strings.Contains(strings.ToLower(bi.name), query) {
				s.matches = append(s.matches, i)
			}
		}
	}
}

func (s switchTUI) View() string {
	if s.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("  Switch Branch") + "\n")
	b.WriteString("\n")

	// List
	b.WriteString(s.renderList())

	// Search box
	if s.searchMode {
		b.WriteString("\n" + searchStyle.Render(fmt.Sprintf("  /%s", s.searchQuery)))
		if len(s.matches) > 0 {
			b.WriteString(fmt.Sprintf("  [%d/%d matches]", s.matchIndex+1, len(s.matches)))
		}
	} else {
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n" + helpStyle.Render("  ↑↓ navigate  / search  n/N next/prev match  enter select  q quit"))

	return b.String()
}

func (s switchTUI) renderList() string {
	items := s.list.Items()
	selectedIdx := s.list.Index()

	var lines []string
	for i, item := range items {
		bi := item.(branchItem)

		// Build indentation
		indent := ""
		for j := 0; j < bi.depth; j++ {
			indent += "  "
		}

		// Icon
		icon := "○"
		if bi.current {
			icon = "●"
		}

		// Style based on state
		line := fmt.Sprintf("%s%s %s", indent, icon, bi.name)

		// Check if this item matches search
		isMatch := false
		if s.searchQuery != "" {
			for _, matchIdx := range s.matches {
				if matchIdx == i {
					isMatch = true
					break
				}
			}
		}

		if i == selectedIdx {
			// Selected line
			if bi.current {
				line = "> " + currentStyle.Render(line)
			} else if isMatch {
				line = "> " + highlightStyle.Render(line)
			} else {
				line = "> " + branchStyle.Render(line)
			}
		} else {
			// Non-selected line
			if s.searchMode && !isMatch && s.searchQuery != "" {
				// Dim non-matches during search
				line = "  " + dimStyle.Render(line)
			} else if bi.current {
				line = "  " + currentStyle.Render(line)
			} else if isMatch {
				line = "  " + highlightStyle.Render(line)
			} else {
				line = "  " + branchStyle.Render(line)
			}
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func switchCmdFunc() *cobra.Command {
	return &cobra.Command{
		Use:   "switch",
		Short: "Interactively switch to a branch in the stack",
		Long: `Displays an interactive tree view of the stack with vim-like navigation.
Use arrow keys to navigate, / to search, n/N to jump between matches, enter to select.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, _, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := gitRunner.GetCurrentBranch()
			attacher := attach.NewAttacher(gitRunner, nil)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Build list items recursively
			var branchItems []branchItem
			var items []list.Item

			var addBranch func(branch string, depth int)
			addBranch = func(branch string, depth int) {
				isCurrent := branch == currentBranch

				// Get parent for display
				parent := ""
				if b, exists := g.GetBranch(branch); exists {
					parent = b.Parent
				}

				bi := branchItem{
					name:    branch,
					parent:  parent,
					depth:   depth,
					current: isCurrent,
				}
				branchItems = append(branchItems, bi)
				items = append(items, bi)

				// Add children
				children := g.GetChildren(branch)
				for _, child := range children {
					addBranch(child.Name, depth+1)
				}
			}

			addBranch(rootBranch, 0)

			if len(items) == 0 {
				return fmt.Errorf("no branches in stack")
			}

			// Create list model
			l := list.New(items, list.NewDefaultDelegate(), 0, 0)
			l.SetShowHelp(false)
			l.SetShowFilter(false)
			l.SetShowStatusBar(false)
			l.SetShowTitle(false)

			// Find current branch index
			for i, item := range items {
				if bi := item.(branchItem); bi.name == currentBranch {
					l.Select(i)
					break
				}
			}

			// Create model
			model := &switchTUI{
				list:     l,
				branches: branchItems,
				git:      gitRunner,
				graph:    g,
				current:  currentBranch,
			}

			// Run the TUI
			p := tea.NewProgram(model)
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("error running switch: %w", err)
			}

			// Handle result
			if m, ok := finalModel.(switchTUI); ok && m.selected != "" {
				if m.selected == currentBranch {
					fmt.Printf("Already on '%s'\n", m.selected)
				} else {
					err := gitRunner.CheckoutBranch(m.selected)
					if err != nil {
						return fmt.Errorf("failed to checkout %s: %w", m.selected, err)
					}
					fmt.Printf("Switched to '%s'\n", m.selected)
				}
			}

			return nil
		},
	}
}
