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
