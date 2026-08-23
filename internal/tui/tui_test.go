package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gstash/internal/git"
)

func sampleModel(t *testing.T) Model {
	t.Helper()
	m := New("/tmp/nonexistent")
	m.curBranch = "main"
	m.width, m.height = 120, 40
	m.resize()
	m.loaded = true
	m.entries = []git.Entry{
		{Index: 0, Ref: "stash@{0}", Message: "wip feature", Branch: "feature", Source: git.SourceExact, Age: "2 days ago"},
		{Index: 1, Ref: "stash@{1}", Message: "wip main", Branch: "main", Source: git.SourceExact},
	}
	m.refilter()
	return m
}

func send(m Model, keys ...string) Model {
	for _, k := range keys {
		var next tea.Model
		switch k {
		case "up":
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
		case "down":
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		default:
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		}
		m = next.(Model)
	}
	return m
}

func TestFilterDefaultsToCurrentBranch(t *testing.T) {
	m := sampleModel(t)
	if len(m.filtered) != 1 || m.entries[m.filtered[0]].Branch != "main" {
		t.Fatalf("expected only main stash visible: %+v", m.filtered)
	}
	m2 := send(m, "tab")
	if len(m2.filtered) != 2 {
		t.Fatalf("tab should show all: %+v", m2.filtered)
	}
}

func TestNavigationAndRender(t *testing.T) {
	m := sampleModel(t)
	if v := m.View(); !strings.Contains(v, "#1") || !strings.Contains(v, "wip main") {
		t.Fatalf("view missing entries: %q", v)
	}
	m2 := send(m, "tab")
	if m2.cursor != 0 || len(m2.filtered) != 2 {
		t.Fatalf("tab should show all: %+v", m2.filtered)
	}
	if v := m2.View(); !strings.Contains(v, "AGE") || !strings.Contains(v, "2 days ago") || !strings.Contains(v, "BRANCH") {
		t.Fatalf("view missing table header/age column: %q", v)
	}
	m3 := send(m2, "down", "down", "down")
	if m3.cursor != len(m3.filtered)-1 {
		t.Fatalf("cursor should clamp to last: %d", m3.cursor)
	}
}

func TestDropConfirmationFlow(t *testing.T) {
	m := sampleModel(t)
	m = send(m, "d")
	if m.mode != modeConfirmDrop {
		t.Fatalf("expected confirm mode, got %v", m.mode)
	}
	m = send(m, "y")
	if m.mode != modeList {
		t.Fatal("confirm should close after y")
	}
}

func TestQuitKey(t *testing.T) {
	m := sampleModel(t)
	final, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should quit")
	}
	_ = final
}

func TestListLoadedRefilter(t *testing.T) {
	m := sampleModel(t)
	msg := listLoadedMsg{entries: []git.Entry{
		{Index: 0, Ref: "stash@{0}", Message: "x", Branch: "main", Source: git.SourceExact},
	}}
	m2, _ := m.Update(msg)
	tm := m2.(Model)
	if len(tm.entries) != 1 || tm.entries[0].Message != "x" {
		t.Fatalf("entries not loaded: %+v", tm.entries)
	}
}
