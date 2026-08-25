package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	sidebarWidth = 24 // inner width of the sidebar column
	// Below this total width the sidebar is dropped and only notes are shown.
	sidebarMinWidth = 70
)

// layout holds the geometry for one render. All fields are inner dimensions:
// the frame border is already accounted for.
type layout struct {
	Width, Height int // full terminal size
	Sidebar       int // sidebar inner width; 0 when collapsed
	Main          int // main pane inner width
	Body          int // rows available inside the frame
}

// computeLayout derives pane geometry from the terminal size. Pure, so the
// arithmetic can be tested without a terminal.
func computeLayout(width, height int) layout {
	l := layout{Width: width, Height: height}

	// Two rows for the frame's top and bottom, one for the footer hints.
	l.Body = max(height-3, 1)

	// Two columns for the frame's left and right edges.
	inner := max(width-2, 1)
	if width >= sidebarMinWidth {
		l.Sidebar = sidebarWidth
		l.Main = max(inner-sidebarWidth-1, 1) // -1 for the vertical rule
	} else {
		l.Main = inner
	}

	return l
}

// listCapacity is how many sidebar rows remain for lists once the two section
// headers (with their trailing blanks) are accounted for.
func (l layout) listCapacity() int {
	const used = 4 // "Notebooks" + blank + "Tags" headers and spacing
	return max(l.Body-used, 1)
}

// pad truncates or right-pads s to exactly width display cells, measuring
// ANSI-aware so styled text lines up.
func pad(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// truncate cuts s to width cells, ending in "…" when anything was removed.
// ANSI-aware, so it is safe on already-styled text.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// row renders "left" and "right" on one line of the given width, with right
// flushed to the end and left truncated if the two would collide.
func row(left, right string, width int) string {
	rw := lipgloss.Width(right)
	if rw >= width {
		return pad(right, width)
	}
	left = truncate(left, width-rw-1)
	gap := width - lipgloss.Width(left) - rw
	return left + strings.Repeat(" ", gap) + right
}

// frame draws the rounded border with a title tab, as in the reference design.
// body lines are padded to the inner width; extra lines are dropped.
func frame(title string, body []string, width, height int) string {
	inner := max(width-2, 1)

	var b strings.Builder
	// "╭─ " + title + " " + filler + "╮" must span width cells.
	dashes := max(inner-3-lipgloss.Width(title), 0)
	b.WriteString(frameStyle.Render("╭─ ") + titleStyle.Render(title) +
		frameStyle.Render(" "+strings.Repeat("─", dashes)+"╮"))
	b.WriteString("\n")

	for i := 0; i < height; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		b.WriteString(frameStyle.Render("│") + pad(line, inner) + frameStyle.Render("│"))
		b.WriteString("\n")
	}

	b.WriteString(frameStyle.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}
