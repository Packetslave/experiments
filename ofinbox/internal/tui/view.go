package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night-ish palette, matching the repo's web tools.
var (
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	flagStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	tagStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c0caf5"))
	headStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3b4261")).
			Padding(1, 2)

	selectedStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	pickerCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	pickerTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c0caf5"))
)

const maxNoteLines = 8

func (m Model) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	cardW := min(w-4, 78)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + headStyle.Render("OmniFocus Inbox"))
	if m.mode != modeLoading {
		b.WriteString(dimStyle.Render(fmt.Sprintf("   %d in inbox · %d processed this session", len(m.tasks), m.processed)))
		if m.linksOnly {
			b.WriteString("   " + tagStyle.Render("🔗 links only"))
		}
	}
	b.WriteString("\n\n")

	switch {
	case m.mode == modeLoading:
		b.WriteString("  " + m.spin.View() + " loading from OmniFocus…\n")
		return b.String()

	case m.loadErr != nil:
		b.WriteString("  " + errStyle.Render("✗ "+m.loadErr.Error()) + "\n\n")
		b.WriteString("  " + dimStyle.Render("If this is a permissions error, allow your terminal to control") + "\n")
		b.WriteString("  " + dimStyle.Render("OmniFocus in System Settings → Privacy & Security → Automation.") + "\n\n")
		b.WriteString("  " + dimStyle.Render("r retry · q quit") + "\n")
		return b.String()

	case m.mode == modePickProject || m.mode == modePickTag:
		b.WriteString(indent(m.picker.view(), 2))
		return b.String()

	case m.mode == modeEditTitle || m.mode == modeEditDefer || m.mode == modeEditDue:
		label := map[mode]string{
			modeEditTitle: "Edit title",
			modeEditDefer: "Defer until",
			modeEditDue:   "Due",
		}[m.mode]
		b.WriteString("  " + pickerTitleStyle.Render(label) + "\n")
		b.WriteString("  " + m.input.View() + "\n")
		if m.inputErr != "" {
			b.WriteString("  " + errStyle.Render(m.inputErr) + "\n")
		}
		b.WriteString("\n  " + dimStyle.Render("enter apply · esc cancel") + "\n")
		return b.String()
	}

	// Browse mode.
	if len(m.tasks) == 0 {
		b.WriteString("  " + okStyle.Render("Inbox zero! 🎉") + "\n\n")
		b.WriteString("  " + dimStyle.Render("r refresh · q quit") + "\n")
		b.WriteString(m.statusLine())
		return b.String()
	}
	if m.linksOnly && m.visibleCount() == 0 {
		b.WriteString("  " + okStyle.Render("No links in the inbox 🎉") + "\n\n")
		b.WriteString("  " + dimStyle.Render("L show all · r refresh · q quit") + "\n")
		b.WriteString(m.statusLine())
		return b.String()
	}

	task := m.tasks[m.index]

	var card strings.Builder
	name := oneline(task.Name)
	var title string
	if name == "" {
		title = dimStyle.Italic(true).Render("(untitled)")
	} else {
		title = titleStyle.Render(name)
	}
	if task.Flagged {
		title = flagStyle.Render("⚑ ") + title
	}
	card.WriteString(title)

	var meta []string
	if _, ok := task.LinkURL(); ok {
		meta = append(meta, tagStyle.Render("🔗 link"))
	}
	if task.ChildCount > 0 {
		sub := "subtasks"
		if task.ChildCount == 1 {
			sub = "subtask"
		}
		meta = append(meta, accentStyle.Render(fmt.Sprintf("▸ %d %s", task.ChildCount, sub)))
	}
	if task.ParentID != "" {
		if pi, ok := m.taskIndex(task.ParentID); ok {
			meta = append(meta, dimStyle.Render("under “"+displayName(m.tasks[pi].Name)+"”"))
		}
	}
	if len(task.Tags) > 0 {
		meta = append(meta, tagStyle.Render("# "+strings.Join(task.Tags, ", ")))
	}
	if task.DeferDate != nil {
		meta = append(meta, dimStyle.Render("defer ")+accentStyle.Render(fmtWhen(task.DeferDate)))
	}
	if task.DueDate != nil {
		s := accentStyle
		if task.DueDate.Before(time.Now()) {
			s = errStyle
		}
		meta = append(meta, dimStyle.Render("due ")+s.Render(fmtWhen(task.DueDate)))
	}
	if len(meta) > 0 {
		card.WriteString("\n" + strings.Join(meta, dimStyle.Render("  ·  ")))
	}

	if strings.TrimSpace(task.Note) != "" {
		card.WriteString("\n\n" + dimStyle.Render(truncateLines(task.Note, maxNoteLines)))
	}

	b.WriteString(indent(cardStyle.Width(cardW).Render(card.String()), 2))
	b.WriteString("\n\n")

	pos := fmt.Sprintf("item %d of %d", m.index+1, len(m.tasks))
	if m.linksOnly {
		pos = fmt.Sprintf("link %d of %d · %d in inbox", m.visiblePos(), m.visibleCount(), len(m.tasks))
	}
	if m.busy {
		pos += "  " + m.spin.View() + "working…"
	}
	b.WriteString("  " + dimStyle.Render(pos) + "\n")
	b.WriteString(m.statusLine())
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render("j/k next/prev · g/G first/last · c complete · d drop · f file · l file link · o open link · t tag") + "\n")
	b.WriteString("  " + dimStyle.Render("! flag · e edit title · s defer · u due · enter subtasks · L links only · r refresh · q quit") + "\n")
	return b.String()
}

func (m Model) statusLine() string {
	if m.status == "" {
		return "\n"
	}
	style := okStyle
	if m.statusIsErr {
		style = errStyle
	}
	return "  " + style.Render(m.status) + "\n"
}

// oneline collapses all whitespace (including newlines) in a task name so
// stray line breaks in real data can't distort the card or status line.
func oneline(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// displayName is oneline with a placeholder for nameless tasks (real inboxes
// contain them — e.g. a captured link whose URL landed in the note).
func displayName(s string) string {
	if t := oneline(s); t != "" {
		return t
	}
	return "(untitled)"
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:maxLines], "\n") + "\n…"
}
