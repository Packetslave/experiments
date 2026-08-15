package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
)

// drive runs a msg through Update and synchronously executes any returned
// command (including batches), feeding resulting msgs back in — enough to
// exercise the full action round trip against the DemoClient.
func drive(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	model := next.(Model)
	if cmd == nil {
		return model
	}
	out := cmd()
	if out == nil {
		return model
	}
	// Skip spinner ticks: while busy they schedule the next tick forever.
	if _, isTick := out.(spinner.TickMsg); isTick {
		return model
	}
	if batch, ok := out.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if inner := c(); inner != nil {
				model = drive(t, model, inner)
			}
		}
		return model
	}
	return drive(t, model, out)
}

func loaded(t *testing.T, client omnifocus.Client) Model {
	t.Helper()
	m := New(client)
	msg := loadCmd(client)()
	lm, ok := msg.(loadedMsg)
	if !ok || lm.err != nil {
		t.Fatalf("loadCmd: %#v", msg)
	}
	next, _ := m.Update(lm)
	return next.(Model)
}

func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	}
	t := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	return t
}

func TestCompleteRemovesTask(t *testing.T) {
	client := omnifocus.NewDemoClient()
	m := loaded(t, client)
	initial := len(m.tasks)
	if initial == 0 {
		t.Fatal("demo client has no tasks")
	}

	m = drive(t, m, key("c"))
	if len(m.tasks) != initial-1 {
		t.Fatalf("after complete: %d tasks, want %d", len(m.tasks), initial-1)
	}
	if m.processed != 1 {
		t.Fatalf("processed = %d, want 1", m.processed)
	}
	if m.busy {
		t.Fatal("model still busy after action completed")
	}
}

func TestNavigationBounds(t *testing.T) {
	m := loaded(t, omnifocus.NewDemoClient())
	m = drive(t, m, key("k"))
	if m.index != 0 {
		t.Fatalf("k at start: index = %d, want 0", m.index)
	}
	for range m.tasks {
		m = drive(t, m, key("j"))
	}
	if m.index != len(m.tasks)-1 {
		t.Fatalf("j past end: index = %d, want %d", m.index, len(m.tasks)-1)
	}
}

func TestMultiRuneKeysReplayIndividually(t *testing.T) {
	m := loaded(t, omnifocus.NewDemoClient())
	// Coalesced input (fast key repeat / paste) arrives as one multi-rune
	// KeyMsg; each rune should act as its own keypress in browse mode.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjj")})
	if m.index != 3 {
		t.Fatalf("after coalesced jjj: index = %d, want 3", m.index)
	}
}

func TestJumpKeys(t *testing.T) {
	m := loaded(t, omnifocus.NewDemoClient())
	m = drive(t, m, key("G"))
	if m.index != len(m.tasks)-1 {
		t.Fatalf("G: index = %d, want %d", m.index, len(m.tasks)-1)
	}
	m = drive(t, m, key("g"))
	if m.index != 0 {
		t.Fatalf("g: index = %d, want 0", m.index)
	}
}

func TestOneline(t *testing.T) {
	got := oneline("\n\n  https://example.com  \nrest\n")
	if got != "https://example.com rest" {
		t.Fatalf("oneline = %q", got)
	}
}

func TestFileToProject(t *testing.T) {
	m := loaded(t, omnifocus.NewDemoClient())
	initial := len(m.tasks)

	m = drive(t, m, key("f"))
	if m.mode != modePickProject {
		t.Fatalf("mode = %d, want modePickProject", m.mode)
	}
	if len(m.picker.filtered) == 0 {
		t.Fatal("picker has no items")
	}
	m = drive(t, m, key("enter"))
	if m.mode != modeBrowse {
		t.Fatalf("mode after pick = %d, want modeBrowse", m.mode)
	}
	if len(m.tasks) != initial-1 {
		t.Fatalf("after filing: %d tasks, want %d", len(m.tasks), initial-1)
	}
	if m.processed != 1 {
		t.Fatalf("processed = %d, want 1", m.processed)
	}
}

func TestAddTagUpdatesTask(t *testing.T) {
	m := loaded(t, omnifocus.NewDemoClient())
	before := len(m.tasks[m.index].Tags)

	m = drive(t, m, key("t"))
	if m.mode != modePickTag {
		t.Fatalf("mode = %d, want modePickTag", m.mode)
	}
	m = drive(t, m, key("enter"))
	if got := len(m.tasks[m.index].Tags); got != before+1 {
		t.Fatalf("tags after tagging = %d, want %d", got, before+1)
	}
	// Task stays in the inbox — tagging is not a terminal action.
	if m.processed != 0 {
		t.Fatalf("processed = %d, want 0", m.processed)
	}
}

func TestPickerFilters(t *testing.T) {
	p := newPicker("Pick:", "", []pickItem{
		{id: "1", label: "Home maintenance", desc: "Personal"},
		{id: "2", label: "Experiments repo", desc: "Work"},
		{id: "3", label: "Reading list"},
	})
	if len(p.filtered) != 3 {
		t.Fatalf("unfiltered: %d, want 3", len(p.filtered))
	}
	p.input.SetValue("work")
	p.refilter()
	if len(p.filtered) != 1 {
		t.Fatalf("filtered by desc: %d, want 1", len(p.filtered))
	}
	it, ok := p.selected()
	if !ok || it.id != "2" {
		t.Fatalf("selected = %+v, want id 2", it)
	}
	p.input.SetValue("zzz")
	p.refilter()
	if _, ok := p.selected(); ok {
		t.Fatal("selected on empty filter should report !ok")
	}
}

func TestEscCancelsPicker(t *testing.T) {
	m := loaded(t, omnifocus.NewDemoClient())
	initial := len(m.tasks)
	m = drive(t, m, key("f"))
	m = drive(t, m, key("esc"))
	if m.mode != modeBrowse {
		t.Fatalf("mode after esc = %d, want modeBrowse", m.mode)
	}
	if len(m.tasks) != initial {
		t.Fatal("esc should not change tasks")
	}
}
