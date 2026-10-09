package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

// ---------------------------------------------------------------------------
// TestSwitchTUI — model-level tests, no binary
// ---------------------------------------------------------------------------

func TestSwitchTUI(t *testing.T) {
	makeSwitchModel := func(branches []branchItem) switchTUI {
		var items []list.Item
		for _, b := range branches {
			items = append(items, b)
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		l.SetShowHelp(false)
		l.SetShowFilter(false)
		l.SetShowStatusBar(false)
		l.SetShowTitle(false)
		return switchTUI{
			list:     l,
			branches: branches,
			current:  "main",
		}
	}

	t.Run("enter_selects_branch", func(t *testing.T) {
		branches := []branchItem{
			{name: "main", current: true, depth: 0},
			{name: "feature-1", current: false, depth: 1},
		}
		model := makeSwitchModel(branches)
		// Select index 1
		model.list.Select(1)

		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m := updated.(switchTUI)
		if m.selected != "feature-1" {
			t.Errorf("selected = %q, want feature-1", m.selected)
		}
	})

	t.Run("search_mode", func(t *testing.T) {
		branches := []branchItem{
			{name: "main", current: true, depth: 0},
			{name: "api-endpoint", current: false, depth: 1},
			{name: "feature-1", current: false, depth: 1},
		}
		model := makeSwitchModel(branches)

		// Enter search mode
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		m := updated.(switchTUI)
		if !m.searchMode {
			t.Fatal("should be in search mode")
		}

		// Type "api"
		for _, ch := range "api" {
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
			m = updated.(switchTUI)
		}

		if len(m.matches) != 1 {
			t.Fatalf("matches = %d, want 1", len(m.matches))
		}
		if m.matches[0] != 1 {
			t.Errorf("match index = %d, want 1", m.matches[0])
		}
	})

	t.Run("n_and_N_step_through_the_search_matches", func(t *testing.T) {
		model := makeSwitchModel([]branchItem{
			{name: "main", current: true, depth: 0},
			{name: "feature-api", depth: 1},
			{name: "docs", depth: 1},
			{name: "feature-ui", depth: 2},
			{name: "feature-tests", depth: 3},
		})
		press := func(m switchTUI, msg tea.KeyMsg) switchTUI {
			updated, _ := m.Update(msg)
			return updated.(switchTUI)
		}
		key := func(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

		// Search for "feature" and confirm it: the first match is selected.
		m := press(model, key('/'))
		for _, ch := range "feature" {
			m = press(m, key(ch))
		}
		m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
		if m.list.Index() != 1 {
			t.Fatalf("after the search, selected index = %d, want the first match at 1", m.list.Index())
		}

		// n goes to the next match and wraps after the last one; N goes back.
		for _, step := range []struct {
			key  rune
			want int
		}{{'n', 3}, {'n', 4}, {'n', 1}, {'N', 4}, {'N', 3}} {
			m = press(m, key(step.key))
			if m.list.Index() != step.want {
				t.Fatalf("after %c, selected index = %d, want %d", step.key, m.list.Index(), step.want)
			}
		}
		if m.selected != "" || m.quitting {
			t.Error("stepping through matches selected a branch or quit")
		}
	})

	t.Run("q_quits", func(t *testing.T) {
		branches := []branchItem{
			{name: "main", current: true, depth: 0},
		}
		model := makeSwitchModel(branches)

		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		m := updated.(switchTUI)
		if !m.quitting {
			t.Error("should be quitting")
		}
		if m.selected != "" {
			t.Errorf("selected = %q, want empty", m.selected)
		}
	})
}
