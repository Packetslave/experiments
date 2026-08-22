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

// createPickID marks the synthetic "create new …" row; the item's label is
// the name to create.
const createPickID = "\x00create"

// picker is a minimal type-to-filter list: every printable key goes into the
// filter, up/down moves the cursor, enter/esc are handled by the caller.
type picker struct {
	title      string
	input      textinput.Model
	items      []pickItem
	filtered   []int // indexes into items; -1 is the synthetic create row
	cursor     int   // index into filtered
	maxVisible int
	createKind string // "project"/"tag" enables the create row; "" disables
	createName string // trimmed filter text the create row would use

	pinned      []int // suggested items (indexes into items), best first
	pinnedShown int   // pinned rows currently at the head of filtered
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

// pin marks the items with the given ids as suggestions: while the filter is
// empty they appear first (starred, in the given order) and the rest of the
// list follows; typing any filter text reverts to plain filtering.
func (p *picker) pin(ids []string) {
	p.pinned = p.pinned[:0]
	for _, id := range ids {
		for i, it := range p.items {
			if it.id == id {
				p.pinned = append(p.pinned, i)
				break
			}
		}
	}
	p.refilter()
}

func (p *picker) refilter() {
	p.createName = strings.TrimSpace(p.input.Value())
	q := strings.ToLower(p.createName)
	p.filtered = p.filtered[:0]
	p.pinnedShown = 0
	if q == "" {
		p.filtered = append(p.filtered, p.pinned...)
		p.pinnedShown = len(p.pinned)
	}
	exact := false
	for i, it := range p.items {
		if strings.ToLower(it.label) == q {
			exact = true
		}
		if q == "" && p.isPinned(i) {
			continue // already at the head
		}
		if q == "" ||
			strings.Contains(strings.ToLower(it.label), q) ||
			strings.Contains(strings.ToLower(it.desc), q) {
			p.filtered = append(p.filtered, i)
		}
	}
	if p.createKind != "" && q != "" && !exact {
		p.filtered = append(p.filtered, -1)
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *picker) isPinned(i int) bool {
	for _, pi := range p.pinned {
		if pi == i {
			return true
		}
	}
	return false
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
	if p.filtered[p.cursor] == -1 {
		return pickItem{id: createPickID, label: p.createName}, true
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
			var it pickItem
			if p.filtered[fi] == -1 {
				it = pickItem{label: "+ new " + p.createKind + ": \"" + p.createName + "\""}
			} else {
				it = p.items[p.filtered[fi]]
			}
			star := ""
			if fi < p.pinnedShown {
				star = flagStyle.Render("★ ")
			}
			line := star + it.label
			if it.desc != "" {
				line += dimStyle.Render("  ·  "+it.desc)
			}
			if fi == p.cursor {
				b.WriteString(pickerCursorStyle.Render("▸ ") + star + selectedStyle.Render(it.label))
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
