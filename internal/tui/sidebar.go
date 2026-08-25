package tui

import "strconv"

// window returns the visible slice bounds for a list of count items with the
// cursor at cursor, showing at most capacity rows.
func window(cursor, count, capacity int) (start, end int) {
	if capacity >= count {
		return 0, count
	}
	start = cursor - capacity/2
	if start < 0 {
		start = 0
	}
	if start+capacity > count {
		start = count - capacity
	}
	return start, start + capacity
}

// listRows renders a labelled list with right-aligned counts. focused controls
// whether the cursor reads as active (accent) or parked (dim).
func listRows(items []string, counts []int, cursor int, focused bool, width, capacity int) []string {
	if len(items) == 0 {
		return []string{"  " + dimStyle.Render("(none)")}
	}
	start, end := window(cursor, len(items), capacity)
	out := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		name, count := items[i], dimStyle.Render(strconv.Itoa(counts[i]))

		nameStyle := textStyle
		glyph := "  "
		if i == cursor {
			glyph = dimStyle.Render(cursorGlyph)
			if focused {
				glyph, nameStyle = selectedStyle.Render(cursorGlyph), selectedStyle
			}
		}
		// 2 cells of margin each side keeps the count off the vertical rule.
		out = append(out, "  "+glyph+row(nameStyle.Render(name), count, width-6)+"  ")
	}
	return out
}

// renderSidebar draws the Notebooks list and the Tags list.
func (m Model) renderSidebar() []string {
	w := m.l.Sidebar
	var out []string

	// Split the remaining rows between the two lists.
	capacity := m.l.listCapacity()
	nbCap := max(min(len(m.notebooks), capacity/2), 1)
	tagCap := max(capacity-nbCap, 1)

	nbCounts := make([]int, len(m.notebooks))
	for i, nb := range m.notebooks {
		nbCounts[i] = m.notebookCount(nb)
	}
	out = append(out, "  "+headerStyle.Render("Notebooks"))
	out = append(out, listRows(m.notebooks, nbCounts, m.nbCur, m.focus == paneNotebooks, w, nbCap)...)
	out = append(out, "")

	tagRows, tagCounts := m.tagRows()
	out = append(out, "  "+headerStyle.Render("Tags"))
	out = append(out, listRows(tagRows, tagCounts, m.tagCur, m.focus == paneTags, w, tagCap)...)
	return out
}

// tagRows returns the tag list as displayed, with the synthetic "all" row
// first, plus each row's note count.
func (m Model) tagRows() ([]string, []int) {
	nb := m.currentNotebook()
	rows := make([]string, 0, len(m.tags)+1)
	counts := make([]int, 0, len(m.tags)+1)
	rows = append(rows, "all")
	counts = append(counts, m.notebookCount(nb))
	for _, t := range m.tags {
		rows = append(rows, t)
		counts = append(counts, m.tagCount(nb, t))
	}
	return rows, counts
}
