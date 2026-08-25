package tui

import (
	"fmt"
	"strings"
	"time"
)

// relTime renders a note's mtime as a short age, e.g. "2d ago".
func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

// renderNotes draws the breadcrumb, a rule, and the windowed note list.
func (m Model) renderNotes() []string {
	w := m.l.Main
	nb := m.currentNotebook()
	if nb == "" {
		return []string{
			"",
			"  " + dimStyle.Render("No notebooks yet."),
			"  " + dimStyle.Render("Press n in the Notebooks pane to make one."),
		}
	}

	tag := m.selectedTag()
	if tag == "" {
		tag = "all"
	}
	crumb := headerStyle.Render(nb) + dimStyle.Render(" / ") + textStyle.Render(tag)

	out := []string{
		"",
		"  " + crumb,
		"  " + frameStyle.Render(strings.Repeat("─", max(w-4, 1))),
	}

	if len(m.visible) == 0 {
		return append(out, "", "  "+dimStyle.Render("No notes here. Press n to write one."))
	}

	capacity := max(m.l.Body-len(out)-1, 1)
	start, end := window(m.noteCur, len(m.visible), capacity)
	for i := start; i < end; i++ {
		n := m.visible[i]
		age := dimStyle.Render(relTime(n.ModTime))

		nameStyle := textStyle
		glyph := "  "
		if i == m.noteCur {
			glyph = dimStyle.Render(cursorGlyph)
			if m.focus == paneNotes {
				glyph, nameStyle = selectedStyle.Render(cursorGlyph), selectedStyle
			}
		}
		label := nameStyle.Render(n.Name)
		// In an "all" view the tag is the only thing distinguishing notes.
		if m.selectedTag() == "" {
			label += dimStyle.Render("  " + n.Tag)
		}
		out = append(out, "  "+glyph+row(label, age, w-6))
	}
	return out
}
