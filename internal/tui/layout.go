package tui

const (
	sidebarWidth = 26
	notesWidth   = 44
	// Below previewMinWidth the preview is dropped; below sidebarMinWidth the
	// sidebar goes too and only the note list is left.
	previewMinWidth = 110
	sidebarMinWidth = 70
)

// layout is the screen geometry for one render. Row 0 is the tab bar, the
// last two rows are the statusline and the hint line, and the panes share
// everything in between. A hidden pane has zero width.
type layout struct {
	width, height int
	sidebar       rect
	notes         rect
	preview       rect
	statusRow     int
	hintRow       int
}

// computeLayout derives pane geometry from the terminal size. Pure, so the
// arithmetic can be tested without a terminal.
func computeLayout(width, height int) layout {
	l := layout{width: width, height: height, statusRow: height - 2, hintRow: height - 1}
	paneH := max(height-3, 0)

	switch {
	case width >= previewMinWidth:
		l.sidebar = rect{0, 1, sidebarWidth, paneH}
		l.notes = rect{sidebarWidth, 1, notesWidth, paneH}
		l.preview = rect{sidebarWidth + notesWidth, 1, width - sidebarWidth - notesWidth, paneH}
	case width >= sidebarMinWidth:
		l.sidebar = rect{0, 1, sidebarWidth, paneH}
		l.notes = rect{sidebarWidth, 1, width - sidebarWidth, paneH}
	default:
		l.notes = rect{0, 1, width, paneH}
	}
	return l
}

func (l layout) hasSidebar() bool { return l.sidebar.w > 0 }
func (l layout) hasPreview() bool { return l.preview.w > 0 }

// window returns the visible slice bounds for a list of count items with the
// cursor kept in view, showing at most capacity rows.
func window(cursor, count, capacity int) (start, end int) {
	if capacity <= 0 {
		return 0, 0
	}
	if capacity >= count {
		return 0, count
	}
	start = max(cursor-capacity/2, 0)
	if start+capacity > count {
		start = count - capacity
	}
	return start, start + capacity
}
