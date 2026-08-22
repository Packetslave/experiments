// Package tui implements the inbox-processing terminal UI on top of
// bubbletea. It presents one inbox item at a time with single-key actions.
package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
	"github.com/packetslave/experiments/ofinbox/internal/suggest"
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

// Where the "l" shortcut files link items. Hardcoded for now; preference later.
const (
	linksProjectName   = "Links to Review"
	linksProjectFolder = "Personal"
	linkTagName        = "NoAction"
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

	// rec suggests projects/tags in the f/t pickers. nil until the filing
	// history loads (a separate async fetch); pickers degrade to plain lists.
	rec suggest.Recommender

	index     int  // current task in tasks
	processed int  // items completed/dropped/filed this session
	linksOnly bool // when set, navigation and actions see only link items

	picker   picker
	input    textinput.Model
	inputErr string

	// pending destructive-action confirmation for an action group: the key
	// must be pressed twice on the same task before it runs.
	confirmKey string
	confirmID  string

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
	return tea.Batch(m.spin.Tick, loadCmd(m.client), historyCmd(m.client))
}

// maxSuggestions caps the pinned rows at the top of the f/t pickers.
const maxSuggestions = 3

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

	// set when the action created a project/tag, so the model's lists pick
	// it up without a full reload.
	newProject *omnifocus.Project
	newTag     *omnifocus.Tag

	// set when the action was a filing decision worth teaching the
	// recommender mid-session.
	learn *omnifocus.HistoryEntry
}

// historyMsg carries the filing history fetched for suggestion scoring.
type historyMsg struct {
	entries []omnifocus.HistoryEntry
	err     error
}

func historyCmd(client omnifocus.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		entries, err := client.History(ctx)
		return historyMsg{entries: entries, err: err}
	}
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

// childrenMsg carries the lazily fetched subtasks of one action group.
type childrenMsg struct {
	parentID string
	tasks    []omnifocus.Task
	err      error
}

func childrenCmd(client omnifocus.Client, parentID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		kids, err := client.Children(ctx, parentID)
		return childrenMsg{parentID: parentID, tasks: kids, err: err}
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
		m.snapToVisible()
		m.mode = modeBrowse
		return m, nil

	case historyMsg:
		if msg.err != nil {
			m.setStatus("no suggestions (history fetch failed: "+msg.err.Error()+")", false)
			return m, nil
		}
		m.rec = suggest.NewLexical(msg.entries)
		return m, nil

	case actionDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatus("✗ "+msg.err.Error(), true)
			return m, nil
		}
		if msg.learn != nil && m.rec != nil {
			m.rec.Learn(*msg.learn)
		}
		if msg.newProject != nil {
			m.projects = append(m.projects, *msg.newProject)
		}
		if msg.newTag != nil {
			m.tags = append(m.tags, *msg.newTag)
		}
		if i, ok := m.taskIndex(msg.taskID); ok {
			if msg.remove {
				m.removeFromQueue(msg.taskID)
				m.processed++
			} else if msg.apply != nil {
				msg.apply(&m.tasks[i])
			}
		}
		m.snapToVisible()
		m.setStatus(msg.status, false)
		return m, nil

	case openedMsg:
		if msg.err != nil {
			m.setStatus("✗ open failed: "+msg.err.Error(), true)
		} else {
			m.setStatus("🌐 opened: "+msg.url, false)
		}
		return m, nil

	case childrenMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatus("✗ "+msg.err.Error(), true)
			return m, nil
		}
		i, ok := m.taskIndex(msg.parentID)
		if !ok {
			return m, nil
		}
		if len(msg.tasks) == 0 {
			m.tasks[i].ChildCount = 0
			m.setStatus("no subtasks", false)
			return m, nil
		}
		// Splice children before the parent so the parent is processed last;
		// once its subtasks are dispatched, it comes up as a normal item.
		out := make([]omnifocus.Task, 0, len(m.tasks)+len(msg.tasks))
		out = append(out, m.tasks[:i]...)
		out = append(out, msg.tasks...)
		out = append(out, m.tasks[i:]...)
		parentName := displayName(m.tasks[i].Name)
		m.tasks = out
		m.tasks[i+len(msg.tasks)].ChildCount = len(msg.tasks)
		m.index = i
		m.snapToVisible()
		m.setStatus(fmt.Sprintf("▾ %d subtasks of %s", len(msg.tasks), parentName), false)
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

	// Any key other than the awaited one abandons a pending confirmation.
	if m.confirmKey != "" && key != m.confirmKey {
		m.confirmKey, m.confirmID = "", ""
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "r":
		m.mode = modeLoading
		m.setStatus("", false)
		return m, tea.Batch(m.spin.Tick, loadCmd(m.client), historyCmd(m.client))
	case "j", "down", "right":
		if i, ok := m.seekVisible(m.index+1, +1); ok {
			m.index = i
		}
		return m, nil
	case "k", "up", "left":
		if i, ok := m.seekVisible(m.index-1, -1); ok {
			m.index = i
		}
		return m, nil
	case "g", "home":
		if i, ok := m.seekVisible(0, +1); ok {
			m.index = i
		}
		return m, nil
	case "G", "end":
		if i, ok := m.seekVisible(len(m.tasks)-1, -1); ok {
			m.index = i
		}
		return m, nil
	case "L":
		m.linksOnly = !m.linksOnly
		if m.linksOnly {
			m.snapToVisible()
			m.setStatus(fmt.Sprintf("🔗 links only — %d of %d items", m.visibleCount(), len(m.tasks)), false)
		} else {
			m.setStatus("showing all items", false)
		}
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
		if !m.confirmCascade("c", task, "completes") {
			return m, nil
		}
		m.busy = true
		id := task.ID
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.Complete(ctx, id) },
			actionDoneMsg{status: "✓ completed: " + displayName(task.Name), remove: true}))

	case "d":
		if !m.confirmCascade("d", task, "drops") {
			return m, nil
		}
		m.busy = true
		id := task.ID
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.Drop(ctx, id) },
			actionDoneMsg{status: "⌫ dropped: " + displayName(task.Name), remove: true}))

	case "l":
		url, ok := task.LinkURL()
		if !ok {
			m.setStatus("not a link (title or note must be just a URL)", true)
			return m, nil
		}
		proj, ok := m.findProject(linksProjectName, linksProjectFolder)
		if !ok {
			m.setStatus(fmt.Sprintf("project %q not found", linksProjectName), true)
			return m, nil
		}
		tag, ok := m.findTag(linkTagName)
		if !ok {
			m.setStatus(fmt.Sprintf("tag %q not found", linkTagName), true)
			return m, nil
		}
		label := displayName(task.Name)
		if oneline(task.Name) == "" {
			label = url
		}
		m.busy = true
		id := task.ID
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error {
				if err := m.client.AddTag(ctx, id, tag.ID); err != nil {
					return err
				}
				return m.client.MoveToProject(ctx, id, proj.ID)
			},
			actionDoneMsg{status: "🔗 → " + proj.Name + " +" + tag.Name + ": " + label, remove: true}))

	case "o":
		url, ok := task.LinkURL()
		if !ok {
			m.setStatus("not a link (title or note must be just a URL)", true)
			return m, nil
		}
		return m, openURLCmd(url)

	case "enter":
		if task.ChildCount == 0 {
			return m, nil
		}
		// Already expanded: jump to the first child instead of re-fetching.
		for i, t := range m.tasks {
			if t.ParentID == task.ID {
				m.index = i
				return m, nil
			}
		}
		m.busy = true
		return m, tea.Batch(m.spin.Tick, childrenCmd(m.client, task.ID))

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
		m.picker.createKind = "project"
		if m.rec != nil {
			cands := make([]suggest.Candidate, 0, len(m.projects))
			for _, p := range m.projects {
				cands = append(cands, suggest.Candidate{ID: p.ID, Name: p.Name})
			}
			m.picker.pin(m.rec.Projects(task.Name+" "+task.Note, cands, maxSuggestions))
		}
		m.mode = modePickProject
		return m, textinput.Blink

	case "t":
		items := make([]pickItem, 0, len(m.tags))
		for _, g := range m.tags {
			items = append(items, pickItem{id: g.ID, label: g.Name})
		}
		m.picker = newPicker("Add tag to \""+displayName(task.Name)+"\":", "type to filter tags", items)
		m.picker.createKind = "tag"
		if m.rec != nil {
			cands := make([]suggest.Candidate, 0, len(m.tags))
			for _, g := range m.tags {
				cands = append(cands, suggest.Candidate{ID: g.ID, Name: g.Name})
			}
			m.picker.pin(m.rec.Tags(task.Name+" "+task.Note, cands, maxSuggestions))
		}
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
	case wasProjectPick && it.id == createPickID:
		name := it.label
		taskName := displayName(task.Name)
		fullName := task.Name
		client := m.client
		return m, tea.Batch(m.spin.Tick, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
			defer cancel()
			proj, err := client.CreateProject(ctx, name)
			if err != nil {
				return actionDoneMsg{taskID: id, err: err}
			}
			if err := client.MoveToProject(ctx, id, proj.ID); err != nil {
				return actionDoneMsg{taskID: id, err: err}
			}
			return actionDoneMsg{taskID: id, remove: true, newProject: &proj,
				learn:  &omnifocus.HistoryEntry{Name: fullName, ProjectID: proj.ID},
				status: "→ filed to new project " + proj.Name + ": " + taskName}
		})

	case !wasProjectPick && it.id == createPickID:
		name := it.label
		taskName := displayName(task.Name)
		fullName := task.Name
		client := m.client
		return m, tea.Batch(m.spin.Tick, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
			defer cancel()
			tag, err := client.CreateTag(ctx, name)
			if err != nil {
				return actionDoneMsg{taskID: id, err: err}
			}
			if err := client.AddTag(ctx, id, tag.ID); err != nil {
				return actionDoneMsg{taskID: id, err: err}
			}
			tagName := tag.Name
			return actionDoneMsg{taskID: id, newTag: &tag,
				learn: &omnifocus.HistoryEntry{Name: fullName, TagIDs: []string{tag.ID}},
				status: "# tagged " + tagName + " (new): " + taskName,
				apply: func(t *omnifocus.Task) {
					for _, existing := range t.Tags {
						if existing == tagName {
							return
						}
					}
					t.Tags = append(t.Tags, tagName)
				}}
		})

	case wasProjectPick:
		pickID := it.id
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.MoveToProject(ctx, id, pickID) },
			actionDoneMsg{status: "→ filed to " + it.label + ": " + displayName(task.Name), remove: true,
				learn: &omnifocus.HistoryEntry{Name: task.Name, ProjectID: pickID}}))
	default: // Add tag
		pickID := it.id
		tagName := it.label
		return m, tea.Batch(m.spin.Tick, actionCmd(id,
			func(ctx context.Context) error { return m.client.AddTag(ctx, id, pickID) },
			actionDoneMsg{status: "# tagged " + tagName + ": " + displayName(task.Name),
				learn: &omnifocus.HistoryEntry{Name: task.Name, TagIDs: []string{pickID}},
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

// confirmCascade gates a destructive key on an action group: the first press
// arms a warning, the second press (same key, same task) reports true to let
// the action run. Leaf tasks always pass.
func (m *Model) confirmCascade(key string, task omnifocus.Task, verb string) bool {
	if task.ChildCount == 0 {
		return true
	}
	if m.confirmKey == key && m.confirmID == task.ID {
		m.confirmKey, m.confirmID = "", ""
		return true
	}
	m.confirmKey, m.confirmID = key, task.ID
	sub := "subtasks"
	if task.ChildCount == 1 {
		sub = "subtask"
	}
	m.setStatus(fmt.Sprintf("⚠ %s %d %s too — press %s again to confirm", verb, task.ChildCount, sub, key), true)
	return false
}

// removeFromQueue drops a task plus any of its dived-in descendants from the
// visible queue (OmniFocus cascades complete/drop/move to the subtree), fixes
// the cursor, and decrements the parent's badge when a child leaves.
func (m *Model) removeFromQueue(id string) {
	parentID := ""
	if i, ok := m.taskIndex(id); ok {
		parentID = m.tasks[i].ParentID
	}
	doomed := map[string]bool{id: true}
	for grew := true; grew; {
		grew = false
		for _, t := range m.tasks {
			if t.ParentID != "" && doomed[t.ParentID] && !doomed[t.ID] {
				doomed[t.ID] = true
				grew = true
			}
		}
	}
	kept := m.tasks[:0]
	removedBefore := 0
	for i, t := range m.tasks {
		if doomed[t.ID] {
			if i < m.index {
				removedBefore++
			}
			continue
		}
		kept = append(kept, t)
	}
	m.tasks = kept
	m.index = max(0, min(m.index-removedBefore, len(m.tasks)-1))
	if parentID != "" {
		if pi, ok := m.taskIndex(parentID); ok && m.tasks[pi].ChildCount > 0 {
			m.tasks[pi].ChildCount--
		}
	}
}

// findProject matches by name, preferring a project in the given folder when
// the name is ambiguous.
func (m *Model) findProject(name, folder string) (omnifocus.Project, bool) {
	var fallback omnifocus.Project
	found := false
	for _, p := range m.projects {
		if p.Name != name {
			continue
		}
		if p.Folder == folder {
			return p, true
		}
		if !found {
			fallback, found = p, true
		}
	}
	return fallback, found
}

func (m *Model) findTag(name string) (omnifocus.Tag, bool) {
	for _, g := range m.tags {
		if g.Name == name {
			return g, true
		}
	}
	return omnifocus.Tag{}, false
}

func (m *Model) current() (omnifocus.Task, bool) {
	if m.index < 0 || m.index >= len(m.tasks) {
		return omnifocus.Task{}, false
	}
	t := m.tasks[m.index]
	if !m.visible(t) {
		return omnifocus.Task{}, false
	}
	return t, true
}

// visible reports whether the task passes the active filter.
func (m *Model) visible(t omnifocus.Task) bool {
	if !m.linksOnly {
		return true
	}
	_, ok := t.LinkURL()
	return ok
}

// seekVisible scans from index `from` in direction `step` (±1) for a task
// that passes the filter.
func (m *Model) seekVisible(from, step int) (int, bool) {
	for i := from; i >= 0 && i < len(m.tasks); i += step {
		if m.visible(m.tasks[i]) {
			return i, true
		}
	}
	return 0, false
}

// snapToVisible moves the cursor onto the nearest task passing the filter
// (preferring forward), so it never rests on a hidden item.
func (m *Model) snapToVisible() {
	if i, ok := m.seekVisible(m.index, +1); ok {
		m.index = i
	} else if i, ok := m.seekVisible(m.index, -1); ok {
		m.index = i
	}
}

func (m *Model) visibleCount() int {
	n := 0
	for _, t := range m.tasks {
		if m.visible(t) {
			n++
		}
	}
	return n
}

// visiblePos is the current task's 1-based position among visible tasks.
func (m *Model) visiblePos() int {
	n := 0
	for i := 0; i <= m.index && i < len(m.tasks); i++ {
		if m.visible(m.tasks[i]) {
			n++
		}
	}
	return n
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

