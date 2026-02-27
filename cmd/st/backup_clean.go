package main

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
)

type backupCleanItem struct {
	info       backup.BackupInfo
	isHeader   bool
	headerText string
}

type backupCleanTUI struct {
	items      []backupCleanItem
	cursor     int
	selected   map[int]bool
	confirming bool
	quitting   bool
	cancelled  bool
}

func (m backupCleanTUI) Init() tea.Cmd { return nil }

func (m backupCleanTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.quitting = true
				return m, tea.Quit
			default:
				m.confirming = false
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "esc":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case " ":
			if !m.items[m.cursor].isHeader {
				if m.selected[m.cursor] {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = true
				}
			}
		case "a":
			for i, item := range m.items {
				if !item.isHeader {
					m.selected[i] = true
				}
			}
		case "n":
			m.selected = make(map[int]bool)
		case "enter":
			if len(m.selected) > 0 {
				m.confirming = true
			}
		}
	}
	return m, nil
}

func (m *backupCleanTUI) moveCursor(delta int) {
	next := m.cursor + delta
	for next >= 0 && next < len(m.items) {
		if !m.items[next].isHeader {
			m.cursor = next
			return
		}
		next += delta
	}
}

var (
	cleanTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	cleanHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	cleanItemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	cleanDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	cleanHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	cleanWarnStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6347"))
)

func (m backupCleanTUI) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(cleanTitleStyle.Render("  Clean Backups") + "\n\n")

	for i, item := range m.items {
		if item.isHeader {
			b.WriteString("  " + cleanHeaderStyle.Render(item.headerText) + "\n")
			continue
		}

		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		check := "[ ]"
		if m.selected[i] {
			check = "[x]"
		}

		kind := fmt.Sprintf("[%-6s]", item.info.Kind)
		ts := item.info.Timestamp.Format("2006-01-02 15:04:05")

		line := fmt.Sprintf("%s  %s %s %s", cursor, check, kind, ts)
		if i == m.cursor {
			b.WriteString(cleanItemStyle.Render(line) + "\n")
		} else {
			b.WriteString(cleanDimStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")

	if m.confirming {
		b.WriteString(cleanWarnStyle.Render(fmt.Sprintf("  Delete %d backup(s)? (y/n)", len(m.selected))) + "\n")
	} else {
		b.WriteString(cleanHelpStyle.Render("  space toggle  a all  n none  enter delete  q quit") + "\n")
	}

	return b.String()
}

func (m backupCleanTUI) selectedBackups() []backup.BackupInfo {
	var result []backup.BackupInfo
	for i := range m.selected {
		result = append(result, m.items[i].info)
	}
	return result
}

func backupCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Interactively select and delete old backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			backupMgr := backup.NewManager(gitRunner, repoPath)
			all, err := backupMgr.ListAllBackups()
			if err != nil {
				return err
			}

			if len(all) == 0 {
				printer.Println("No backups to clean")
				return nil
			}

			// Group by source branch
			grouped := make(map[string][]backup.BackupInfo)
			for _, b := range all {
				grouped[b.SourceBranch] = append(grouped[b.SourceBranch], b)
			}

			// Sort branch names
			var branchNames []string
			for name := range grouped {
				branchNames = append(branchNames, name)
			}
			sort.Strings(branchNames)

			// Build item list with headers
			var items []backupCleanItem
			firstDataIdx := -1
			for _, branch := range branchNames {
				items = append(items, backupCleanItem{isHeader: true, headerText: branch})
				for _, b := range grouped[branch] {
					if firstDataIdx < 0 {
						firstDataIdx = len(items)
					}
					items = append(items, backupCleanItem{info: b})
				}
			}

			if firstDataIdx < 0 {
				printer.Println("No backups to clean")
				return nil
			}

			model := &backupCleanTUI{
				items:    items,
				cursor:   firstDataIdx,
				selected: make(map[int]bool),
			}

			p := tea.NewProgram(model)
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("error running backup clean: %w", err)
			}

			m, ok := finalModel.(*backupCleanTUI)
			if !ok {
				// Value receiver fallback
				if mv, ok2 := finalModel.(backupCleanTUI); ok2 {
					m = &mv
				} else {
					return nil
				}
			}

			if m.cancelled || len(m.selected) == 0 {
				return nil
			}

			toDelete := m.selectedBackups()
			deleted, err := backupMgr.DeleteBackups(toDelete)
			if err != nil {
				return fmt.Errorf("cleanup failed after deleting %d: %w", deleted, err)
			}

			printer.Println("Deleted %d backup(s)", deleted)
			return nil
		},
	}
}
