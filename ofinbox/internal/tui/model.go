// Package tui implements the inbox-processing terminal UI on top of
// bubbletea. It presents one inbox item at a time with single-key actions.
package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
)

type mode int

const (
	modeLoading mode = iota
	modeBrowse
	modePickProject
	modePickTag
	modeEditTitle
	modeEditDefer
	modeEditDue
)

const (
	deferDefaultHour = 8  // "tomorrow" as a defer means tomorrow morning
	dueDefaultHour   = 17 // "friday" as a due date means Friday 5pm
	actionTimeout    = 30 * time.Second
)

type Model struct {
	client omnifocus.Client

	mode   mode
	width  int
	height int

	spin     spinner.Model
	tasks    []omnifocus.Task
	projects []omnifocus.Project
	tags     []omnifocus.Tag

	index     int // current task in tasks
	processed int // items completed/dropped/filed this session

	picker   picker
	input    textinput.Model
	inputErr string

	busy        bool
	status      string
	statusIsErr bool
	loadErr     error
}

func New(client omnifocus.Client) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = accentStyle
	return Model{client: client, mode: modeLoading, spin: sp}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, loadCmd(m.client))
}

// --- messages ---

type loadedMsg struct {
	tasks    []omnifocus.Task
	projects []omnifocus.Project
	tags     []omnifocus.Tag
	err      error
}

// actionDoneMsg reports a finished backend mutation. On success the model
// either removes the task (complete/drop/file) or applies `apply` to it.
type actionDoneMsg struct {
	taskID string
	status string
	remove bool
	apply  func(*omnifocus.Task)
	err    error
}

func loadCmd(client omnifocus.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		tasks, err := client.InboxTasks(ctx)
		if err != nil {
			return loadedMsg{err: err}
		}
		projects, err := client.Projects(ctx)
		if err != nil {
			return loadedMsg{err: err}
		}
		tags, err := client.Tags(ctx)
		if err != nil {
			return loadedMsg{err: err}
		}
		return loadedMsg{tasks: tasks, projects: projects, tags: tags}
	}
}

func actionCmd(taskID string, call func(context.Context) error, done actionDoneMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		if err := call(ctx); err != nil {
			return actionDoneMsg{taskID: taskID, err: err}
		}
		done.taskID = taskID
		return done
	}
}

// --- update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.mode == modeLoading || m.busy {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case loadedMsg:
		if msg.err != nil {
			m.loadErr = msg.err
			m.mode = modeBrowse
			return m, nil
		}
		m.loadErr = nil
		m.tasks = msg.tasks
		m.projects = msg.projects
		m.tags = msg.tags
		if m.index >= len(m.tasks) {
			m.index = max(0, len(m.tasks)-1)
		}
		m.mode = modeBrowse
		return m, nil

	case actionDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatus("✗ "+msg.err.Error(), true)
			return m, nil
		}
		if i, ok := m.taskIndex(msg.taskID); ok {
			if msg.remove {
				m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
				m.processed++
				if m.index > i {
					m.index--
				}
				if m.index >= len(m.tasks) {
					m.index = max(0, len(m.tasks)-1)
				}
			} else if msg.apply != nil {
				msg.apply(&m.tasks[i])
			}
		}
		m.setStatus(msg.status, false)
		return m, nil

	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// Terminals can coalesce fast keystrokes (key repeat, paste) into one
	// multi-rune message. In browse mode, replay each rune as its own
	// keypress; in input/picker modes the text field handles it as a paste.
	if m.mode == modeBrowse && msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
		var model tea.Model = m
		var cmds []tea.Cmd
		for _, r := range msg.Runes {
			var cmd tea.Cmd
			model, cmd = model.(Model).updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return model, tea.Batch(cmds...)
	}

	switch m.mode {
	case modeLoading:
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil

	case modeBrowse:
		return m.updateBrowse(msg)

	case modePickProject, modePickTag:
		switch msg.String() {
		case "esc":
			m.mode = modeBrowse
			return m, nil
		case "enter":
			it, ok := m.picker.selected()
			if !ok {
				return m, nil
			}
			return m.applyPick(it)
		default:
			cmd := m.picker.update(msg)
			return m, cmd
		}

	case modeEditTitle, modeEditDefer, modeEditDue:
		switch msg.String() {
		case "esc":
			m.mode = modeBrowse
			m.inputErr = ""
			return m, nil
		case "enter":
			return m.applyInput()
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.inputErr = ""
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q":
		return m, tea.Quit
	case "r":
		m.mode = modeLoading
		m.setStatus("", false)
		return m, tea.Batch(m.spin.Tick, loadCmd(m.client))
	case "j", "down", "right":
		if m.index < len(m.tasks)-1 {
			m.index++
		}
		return m, nil
	case "k", "up", "left":
		if m.index > 0 {
			m.index--
		}
		return m, nil
	case "g", "home":
		m.index = 0
		return m, nil
	case "G", "end":
		m.index = max(0, len(m.tasks)-1)
		return m, nil
	}

	// Everything below acts on the current task.
	task, ok := m.current()
	if !ok {
		return m, nil
	}
	if m.busy {
		m.setStatus("still working on the previous action…", true)
		return m, nil
	}

	switch key {
	case "c":
		m.busy = true
		id := task.ID
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.Complete(ctx, id) },
			actionDoneMsg{status: "✓ completed: " + displayName(task.Name), remove: true}))

	case "d":
		m.busy = true
		id := task.ID
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.Drop(ctx, id) },
			actionDoneMsg{status: "⌫ dropped: " + displayName(task.Name), remove: true}))

	case "!":
		m.busy = true
		id := task.ID
		newVal := !task.Flagged
		verb := "flagged"
		if !newVal {
			verb = "unflagged"
		}
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.SetFlagged(ctx, id, newVal) },
			actionDoneMsg{status: "⚑ " + verb + ": " + displayName(task.Name),
				apply: func(t *omnifocus.Task) { t.Flagged = newVal }}))

	case "f":
		items := make([]pickItem, 0, len(m.projects))
		for _, p := range m.projects {
			desc := p.Folder
			if p.Status == "on hold" {
				if desc != "" {
					desc += " · "
				}
				desc += "on hold"
			}
			items = append(items, pickItem{id: p.ID, label: p.Name, desc: desc})
		}
		m.picker = newPicker("File \""+displayName(task.Name)+"\" to project:", "type to filter projects", items)
		m.mode = modePickProject
		return m, textinput.Blink

	case "t":
		items := make([]pickItem, 0, len(m.tags))
		for _, g := range m.tags {
			items = append(items, pickItem{id: g.ID, label: g.Name})
		}
		m.picker = newPicker("Add tag to \""+displayName(task.Name)+"\":", "type to filter tags", items)
		m.mode = modePickTag
		return m, textinput.Blink

	case "e":
		m.input = newInput("new title", task.Name)
		m.mode = modeEditTitle
		return m, textinput.Blink

	case "s":
		m.input = newInput("tomorrow · fri · 3d · 2026-08-20 · empty clears", "")
		m.mode = modeEditDefer
		return m, textinput.Blink

	case "u":
		m.input = newInput("tomorrow · fri · 3d · 2026-08-20 · empty clears", "")
		m.mode = modeEditDue
		return m, textinput.Blink
	}
	return m, nil
}

func newInput(placeholder, value string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Prompt = "› "
	in.SetValue(value)
	in.CursorEnd()
	in.Focus()
	return in
}

func (m Model) applyPick(it pickItem) (tea.Model, tea.Cmd) {
	task, ok := m.current()
	if !ok {
		m.mode = modeBrowse
		return m, nil
	}
	id := task.ID
	wasProjectPick := m.mode == modePickProject
	m.mode = modeBrowse
	m.busy = true

	switch {
	case wasProjectPick:
		pickID := it.id
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.MoveToProject(ctx, id, pickID) },
			actionDoneMsg{status: "→ filed to " + it.label + ": " + displayName(task.Name), remove: true}))
	default: // Add tag
		pickID := it.id
		tagName := it.label
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.AddTag(ctx, id, pickID) },
			actionDoneMsg{status: "# tagged " + tagName + ": " + displayName(task.Name),
				apply: func(t *omnifocus.Task) {
					for _, existing := range t.Tags {
						if existing == tagName {
							return
						}
					}
					t.Tags = append(t.Tags, tagName)
				}}))
	}
}

func (m Model) applyInput() (tea.Model, tea.Cmd) {
	task, ok := m.current()
	if !ok {
		m.mode = modeBrowse
		return m, nil
	}
	id := task.ID
	value := m.input.Value()

	switch m.mode {
	case modeEditTitle:
		if value == "" {
			m.inputErr = "title can't be empty"
			return m, nil
		}
		m.mode = modeBrowse
		m.busy = true
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.Rename(ctx, id, value) },
			actionDoneMsg{status: "✎ renamed to: " + value,
				apply: func(t *omnifocus.Task) { t.Name = value }}))

	case modeEditDefer, modeEditDue:
		isDefer := m.mode == modeEditDefer
		hour := dueDefaultHour
		if isDefer {
			hour = deferDefaultHour
		}
		when, err := parseWhen(value, time.Now(), hour)
		if err != nil {
			m.inputErr = err.Error()
			return m, nil
		}
		m.mode = modeBrowse
		m.busy = true

		label, verb := "due", "◷ due"
		call := m.client.SetDueDate
		if isDefer {
			label, verb = "defer", "⏥ deferred"
			call = m.client.SetDeferDate
		}
		status := verb + " " + fmtWhen(when) + ": " + displayName(task.Name)
		if when == nil {
			status = "cleared " + label + " date: " + displayName(task.Name)
		}
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return call(ctx, id, when) },
			actionDoneMsg{status: status,
				apply: func(t *omnifocus.Task) {
					if isDefer {
						t.DeferDate = when
					} else {
						t.DueDate = when
					}
				}}))
	}
	return m, nil
}

// --- helpers ---

func (m *Model) current() (omnifocus.Task, bool) {
	if m.index < 0 || m.index >= len(m.tasks) {
		return omnifocus.Task{}, false
	}
	return m.tasks[m.index], true
}

func (m *Model) taskIndex(id string) (int, bool) {
	for i, t := range m.tasks {
		if t.ID == id {
			return i, true
		}
	}
	return 0, false
}

func (m *Model) setStatus(s string, isErr bool) {
	m.status = s
	m.statusIsErr = isErr
}

