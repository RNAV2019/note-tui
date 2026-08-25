package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type overlayKind int

const (
	overlayNone    overlayKind = iota
	overlayPrompt              // single-line text entry
	overlayConfirm             // yes/no
	overlayList                // fuzzy-filtered choice; also powers search
	overlayHelp
)

const overlayMaxRows = 10

// overlay is the modal layer: at most one is open at a time, and it swallows
// every key until it resolves.
type overlay struct {
	kind    overlayKind
	title   string
	message string
	input   textinput.Model
	items   []string
	matches []Match
	cursor  int

	accept  func(string) tea.Cmd // prompt and list
	confirm func() tea.Cmd       // confirm
}

func newInput(placeholder, initial string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(initial)
	ti.Focus()
	return ti
}

func newPrompt(title, initial string, accept func(string) tea.Cmd) overlay {
	return overlay{
		kind:   overlayPrompt,
		title:  title,
		input:  newInput("name…", initial),
		accept: accept,
	}
}

func newConfirm(message string, confirm func() tea.Cmd) overlay {
	return overlay{kind: overlayConfirm, title: "Confirm", message: message, confirm: confirm}
}

func newList(title string, items []string, accept func(string) tea.Cmd) overlay {
	return overlay{
		kind:    overlayList,
		title:   title,
		input:   newInput("type to filter…", ""),
		items:   items,
		matches: FilterItems("", items),
		accept:  accept,
	}
}

func newHelp() overlay { return overlay{kind: overlayHelp, title: "Keys"} }

func (o overlay) active() bool { return o.kind != overlayNone }

// selected returns the highlighted list entry, or "" when the list is empty.
func (o overlay) selected() string {
	if o.cursor < 0 || o.cursor >= len(o.matches) {
		return ""
	}
	return o.matches[o.cursor].Value
}

// update handles a key for the overlay. It returns the new overlay state, a
// command to run, and whether the overlay should close.
func (o overlay) update(msg tea.Msg) (overlay, tea.Cmd, bool) {
	key, isKey := msg.(tea.KeyMsg)
	if isKey {
		switch s := key.String(); {
		case s == "esc":
			return o, nil, true

		case o.kind == overlayHelp:
			// Any key dismisses help.
			return o, nil, true

		case o.kind == overlayConfirm:
			switch s {
			case "y", "Y", "enter":
				return o, o.confirm(), true
			case "n", "N", "q":
				return o, nil, true
			}
			return o, nil, false

		case s == "enter":
			if o.kind == overlayPrompt {
				value := strings.TrimSpace(o.input.Value())
				if value == "" {
					return o, nil, true
				}
				return o, o.accept(value), true
			}
			if sel := o.selected(); sel != "" {
				return o, o.accept(sel), true
			}
			return o, nil, true

		case keys.up.matches(s) && o.kind == overlayList && s != "k":
			// "k" is a literal character while filtering, so only the arrow
			// and ctrl bindings move the cursor here.
			if o.cursor > 0 {
				o.cursor--
			}
			return o, nil, false

		case keys.down.matches(s) && o.kind == overlayList && s != "j":
			if o.cursor < len(o.matches)-1 {
				o.cursor++
			}
			return o, nil, false
		}
	}

	if o.kind == overlayPrompt || o.kind == overlayList {
		prev := o.input.Value()
		var cmd tea.Cmd
		o.input, cmd = o.input.Update(msg)
		if o.kind == overlayList && o.input.Value() != prev {
			// Results are score-ranked, so any query change invalidates the cursor.
			o.matches = FilterItems(o.input.Value(), o.items)
			o.cursor = 0
		}
		return o, cmd, false
	}
	return o, nil, false
}

// render draws the overlay as a centred box, returning body lines of the
// given width.
func (o overlay) render(width, height int) []string {
	boxWidth := min(max(width-8, 20), 64)
	if o.kind == overlayHelp {
		boxWidth = min(max(width-8, 20), 72)
	}

	var content []string
	switch o.kind {
	case overlayPrompt:
		content = []string{o.input.View()}

	case overlayConfirm:
		content = append(wrap(o.message, boxWidth-2), "", dimStyle.Render("y confirm · n cancel"))

	case overlayList:
		content = append(content, o.input.View(), "")
		if len(o.matches) == 0 {
			content = append(content, dimStyle.Render("  no matches"))
			break
		}
		start := 0
		if o.cursor >= overlayMaxRows {
			start = o.cursor - overlayMaxRows + 1
		}
		for i := start; i < min(start+overlayMaxRows, len(o.matches)); i++ {
			line := highlight(o.matches[i])
			if i == o.cursor {
				content = append(content, selectedStyle.Render(cursorGlyph)+line)
			} else {
				content = append(content, "  "+line)
			}
		}

	case overlayHelp:
		content = helpLines()
	}

	box := boxLines(o.title, content, boxWidth)

	// Centre the box vertically in the body.
	top := max((height-len(box))/2, 0)
	body := make([]string, 0, height)
	for i := 0; i < top; i++ {
		body = append(body, "")
	}
	left := strings.Repeat(" ", max((width-boxWidth)/2, 0))
	for _, line := range box {
		body = append(body, left+line)
	}
	return body
}

// boxLines draws a titled rounded box of exactly width cells.
func boxLines(title string, content []string, width int) []string {
	inner := width - 2
	dashes := max(inner-3-lipgloss.Width(title), 0)
	out := []string{
		frameStyle.Render("╭─ ") + headerStyle.Render(title) +
			frameStyle.Render(" "+strings.Repeat("─", dashes)+"╮"),
	}
	blank := frameStyle.Render("│") + strings.Repeat(" ", inner) + frameStyle.Render("│")
	out = append(out, blank)
	for _, line := range content {
		out = append(out, frameStyle.Render("│")+" "+pad(line, inner-2)+" "+frameStyle.Render("│"))
	}
	out = append(out, blank)
	return append(out, frameStyle.Render("╰"+strings.Repeat("─", inner)+"╯"))
}

// wrap breaks text into lines of at most width cells, splitting on spaces.
func wrap(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	line := words[0]
	for _, w := range words[1:] {
		if lipgloss.Width(line)+1+lipgloss.Width(w) > width {
			out = append(out, line)
			line = w
			continue
		}
		line += " " + w
	}
	return append(out, line)
}
