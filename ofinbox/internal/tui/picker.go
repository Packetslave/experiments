package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// pickItem is one selectable row in a picker.
type pickItem struct {
	id    string
	label string
	desc  string // secondary text, also matched when filtering
}

// picker is a minimal type-to-filter list: every printable key goes into the
// filter, up/down moves the cursor, enter/esc are handled by the caller.
type picker struct {
	title      string
	input      textinput.Model
	items      []pickItem
	filtered   []int // indexes into items
	cursor     int   // index into filtered
	maxVisible int
}

func newPicker(title, placeholder string, items []pickItem) picker {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Prompt = "› "
	in.Focus()
	p := picker{title: title, input: in, items: items, maxVisible: 8}
	p.refilter()
	return p
}

func (p *picker) refilter() {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	p.filtered = p.filtered[:0]
	for i, it := range p.items {
		if q == "" ||
			strings.Contains(strings.ToLower(it.label), q) ||
			strings.Contains(strings.ToLower(it.desc), q) {
			p.filtered = append(p.filtered, i)
		}
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// update handles navigation keys itself and feeds everything else to the
// filter input. enter/esc are not consumed here.
func (p *picker) update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "ctrl+p":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil
	case "down", "ctrl+n":
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
		return nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.refilter()
	return cmd
}

func (p *picker) selected() (pickItem, bool) {
	if len(p.filtered) == 0 {
		return pickItem{}, false
	}
	return p.items[p.filtered[p.cursor]], true
}

func (p *picker) view() string {
	var b strings.Builder
	b.WriteString(pickerTitleStyle.Render(p.title))
	b.WriteString("\n")
	b.WriteString(p.input.View())
	b.WriteString("\n\n")

	if len(p.filtered) == 0 {
		b.WriteString(dimStyle.Render("  no matches"))
		b.WriteString("\n")
	} else {
		start := 0
		if p.cursor >= p.maxVisible {
			start = p.cursor - p.maxVisible + 1
		}
		end := start + p.maxVisible
		if end > len(p.filtered) {
			end = len(p.filtered)
		}
		for fi := start; fi < end; fi++ {
			it := p.items[p.filtered[fi]]
			line := it.label
			if it.desc != "" {
				line += dimStyle.Render("  ·  "+it.desc)
			}
			if fi == p.cursor {
				b.WriteString(pickerCursorStyle.Render("▸ ") + selectedStyle.Render(it.label))
				if it.desc != "" {
					b.WriteString(dimStyle.Render("  ·  " + it.desc))
				}
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d/%d · ↑/↓ move · enter select · esc cancel", len(p.filtered), len(p.items))))
	return b.String()
}
