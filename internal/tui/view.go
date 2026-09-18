package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note-tui/internal/notes"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render().String())
	v.AltScreen = true
	return v
}

// render draws the whole screen. Popups other than the space menu dim what's
// underneath, and the statusline and hints are drawn last so they always
// describe the current mode.
func (m Model) render() *grid {
	g := newGrid(m.l.width, m.l.height)
	if g.w == 0 || g.h == 0 {
		return g
	}
	m.drawTabs(g)
	if len(m.notebooks) == 0 {
		if m.l.hasSidebar() {
			m.drawSidebar(g, m.l.sidebar)
		}
		m.drawWelcome(g, m.welcomeRect())
	} else {
		if m.l.hasSidebar() {
			m.drawSidebar(g, m.l.sidebar)
		}
		m.drawNotes(g, m.l.notes)
		if m.l.hasPreview() {
			m.drawPreview(g, m.l.preview, previewOpts{})
		}
	}

	if m.ov.active() {
		if m.ov.kind != ovLeader {
			g.dim()
		}
		m.drawOverlay(g)
	}
	if m.l.statusRow >= 1 {
		m.drawStatus(g, m.l.statusRow)
	}
	if m.l.hintRow >= 1 {
		g.clear(0, m.l.hintRow, g.w, 1, cBase)
		drawHints(g, m.l.hintRow, m.hints())
	}
	return g
}

func (m Model) welcomeRect() rect {
	r := m.l.notes
	if m.l.hasPreview() {
		r.w += m.l.preview.w
	}
	return r
}

// ---------------------------------------------------------------- tab bar

func (m Model) drawTabs(g *grid) {
	g.clear(0, 0, g.w, 1, cSurface)
	x := g.put(0, 0, " note ", onB(cBase, cIris))
	nb := m.currentNotebook()
	if nb == "" {
		g.put(x+1, 0, "no notebooks", on(cSubtle, cSurface))
		if g.w >= previewMinWidth {
			g.putRight(g.w-1, 0, tildify(m.store.Root), on(cSubtle, cSurface))
		}
		return
	}
	x = g.put(x, 0, " "+nb+" ", onB(cIris, cHLLow)) + 1

	limit := g.w - 1
	if g.w >= previewMinWidth {
		dir := tildify(m.store.Root)
		limit = g.w - textWidth(dir) - 3
		g.putRight(g.w-1, 0, dir, on(cSubtle, cSurface))
	}

	type tab struct{ num, name, count string }
	tabs := []tab{{"1", "all", strconv.Itoa(m.countNotes(nb, ""))}}
	for i, t := range m.tags {
		num := "·"
		if i+2 <= 9 {
			num = strconv.Itoa(i + 2)
		}
		tabs = append(tabs, tab{num, t, strconv.Itoa(m.countNotes(nb, t))})
	}
	width := func(t tab) int { return textWidth(t.num+t.name+t.count) + 5 }

	// Scroll so the active tab is always on screen.
	start := 0
	for start < m.tagCur {
		used := x
		if start > 0 {
			used += 2
		}
		for i := start; i <= m.tagCur; i++ {
			used += width(tabs[i])
		}
		if used+5 <= limit {
			break
		}
		start++
	}
	if start > 0 {
		x = g.put(x, 0, "‹ ", onB(cGold, cSurface))
	}
	for i := start; i < len(tabs); i++ {
		t, active := tabs[i], i == m.tagCur
		rest := len(tabs) - i - 1
		reserve := 0
		if rest > 0 {
			reserve = 6
		}
		if x+width(t)+reserve > limit && !(active && rest == 0) {
			g.put(x+1, 0, fmt.Sprintf("+%d ›", len(tabs)-i), onB(cGold, cSurface))
			break
		}
		bg := cSurface
		numFg, nameFg := cSubtle, cSubtle
		if active {
			bg, numFg, nameFg = cHLMed, cIris, cText
		}
		x = g.put(x, 0, " "+t.num+" ", on(numFg, bg))
		x = g.put(x, 0, t.name, style{fg: nameFg, bg: bg, bold: active})
		x = g.put(x, 0, " "+t.count+" ", on(nameFg, bg))
		if rest > 0 && !active && i+1 != m.tagCur {
			g.put(x, 0, "│", on(cHLMed, cSurface))
		}
		x++
	}
}

func tildify(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+string(filepath.Separator)) {
			return "~" + path[len(home):]
		}
	}
	return path
}

// ---------------------------------------------------------------- panes

func paneColors(focused bool) (border, title color) {
	if focused {
		return cIris, cIris
	}
	return cMuted, cSubtle
}

// selection returns the row background and marker colour for a list row.
func selection(selected, focused bool) (bg, marker color) {
	switch {
	case !selected:
		return 0, 0
	case focused:
		return cHLMed, cIris
	default:
		return cHLLow, cSubtle
	}
}

const repoRows = 9 // separator, blank, five rows, blank, and the bottom border

func (m Model) drawSidebar(g *grid, r rect) {
	if r.h < 3 {
		return
	}
	focused := m.focus == paneSidebar
	border, title := paneColors(focused)
	g.box(r, border, 0)
	g.boxLabel(r, false, false, cBase, seg{"Notebooks", bold(title)}, seg{strconv.Itoa(len(m.notebooks)), fg(cSubtle)})
	in := r.inner()

	repoTop := r.y + r.h - repoRows
	showRepo := repoTop >= in.y+1+max(len(m.notebooks), 3)+1
	listEnd := in.y + in.h
	if showRepo {
		listEnd = repoTop - 1
	}

	if len(m.notebooks) == 0 {
		g.put(in.x+2, in.y+1, "(none)", fg(cSubtle))
		if in.y+3 < listEnd {
			g.segs(in.x+2, in.y+3, seg{"n", bold(cGold)}, seg{"  new notebook", fg(cText)})
		}
	}
	top := in.y + 1
	start, end := window(m.nbCur, len(m.notebooks), listEnd-top)
	for i := start; i < end; i++ {
		y := top + i - start
		nb := m.notebooks[i]
		sel := i == m.nbCur
		bg, marker := selection(sel, focused)
		if sel {
			g.fill(in.x, y, in.w, 1, bg)
			g.put(in.x, y, "▎", fg(marker))
		}
		mark, markFg := m.gitStateOf(nb).mark()
		g.put(in.x+2, y, mark, fg(markFg))
		count := strconv.Itoa(m.countNotes(nb, ""))
		nameFg, countFg := cText, cSubtle
		if sel && focused {
			nameFg, countFg = cIris, cText
		}
		g.put(in.x+4, y, truncate(nb, in.w-6-len(count)), style{fg: nameFg, bold: sel})
		g.putRight(in.x+in.w-1, y, count, fg(countFg))
	}

	if showRepo {
		m.drawRepo(g, in, repoTop)
	}
}

func (m Model) drawRepo(g *grid, in rect, top int) {
	g.put(in.x, top, "╶"+strings.Repeat("─", max(in.w-2, 0))+"╴", fg(cHLHigh))
	g.put(in.x+2, top, " repo ", bold(cSubtle))

	st := m.repo
	type row struct {
		key, value string
		c          color
	}
	branch := st.Branch
	if branch == "" {
		branch = "—"
	}
	remote := row{"remote", st.Remote, cText}
	if st.Remote == "" {
		remote = row{"remote", "none", cGold}
	}
	sync := row{"sync", "up to date", cFoam}
	switch {
	case st.Remote == "":
		sync = row{"sync", "—", cSubtle}
	case st.Ahead > 0 || st.Behind > 0:
		var parts []string
		c := cFoam
		if st.Ahead > 0 {
			parts = append(parts, fmt.Sprintf("↑%d", st.Ahead))
		}
		if st.Behind > 0 {
			parts = append(parts, fmt.Sprintf("↓%d", st.Behind))
			c = cLove
		}
		sync = row{"sync", strings.Join(parts, " "), c}
	}
	changes := row{"changes", "none", cFoam}
	if st.Changed > 0 {
		changes = row{"changes", plural(st.Changed, "file"), cGold}
	}
	synced := row{"synced", "never", cText}
	switch {
	case m.gitFailed:
		synced = row{"synced", "failed", cLove}
	case !st.LastSync.IsZero():
		synced = row{"synced", relTime(st.LastSync), cText}
	}

	for i, r := range []row{{"branch", branch, cText}, remote, sync, changes, synced} {
		y := top + 2 + i
		g.put(in.x+2, y, r.key, fg(cSubtle))
		g.putRight(in.x+in.w-1, y, truncate(r.value, in.w-12), fg(r.c))
	}
}

func (m Model) drawNotes(g *grid, r rect) {
	if r.h < 3 {
		return
	}
	focused := m.focus == paneNotes
	border, title := paneColors(focused)
	g.box(r, border, 0)
	g.boxLabel(r, false, false, cBase, seg{"Notes", bold(title)}, seg{strconv.Itoa(len(m.visible)), fg(cSubtle)})
	in := r.inner()

	if len(m.visible) == 0 {
		nb, tag := m.currentNotebook(), m.selectedTag()
		mid := in.y + in.h/2 - 1
		switch {
		case len(m.tags) == 0:
			g.putCenter(in.x, in.w, mid, "No tags in "+nb+" yet.", fg(cText))
			m.centerKeyLine(g, in, mid+2, "Press ", "space t n", " to add one.")
		case tag == "":
			g.putCenter(in.x, in.w, mid, "No notes in "+nb+" yet.", fg(cText))
			m.centerKeyLine(g, in, mid+2, "Press ", "n", " to write one.")
		default:
			g.putCenter(in.x, in.w, mid, "No notes in "+tag+" yet.", fg(cText))
			m.centerKeyLine(g, in, mid+2, "Press ", "n", " to write one.")
		}
		return
	}

	allView := m.selectedTag() == ""
	const ageW = 8
	ageEnd := in.x + in.w - 1
	tagW := 0
	if allView {
		for _, n := range m.visible {
			tagW = max(tagW, textWidth(n.Tag))
		}
		tagW = min(tagW, max(in.w/3, 4))
	}
	tagCol := ageEnd - ageW - 1 - tagW
	nameEnd := ageEnd - ageW - 1
	if allView {
		nameEnd = tagCol - 2
	}

	top := in.y + 1
	start, end := window(m.noteCur, len(m.visible), in.y+in.h-top)
	for i := start; i < end; i++ {
		y := top + i - start
		n := m.visible[i]
		sel := i == m.noteCur
		bg, marker := selection(sel, focused)
		if sel {
			g.fill(in.x, y, in.w, 1, bg)
			g.put(in.x, y, "▎", fg(marker))
		}
		nameFg, ageFg := cText, cSubtle
		if sel && focused {
			nameFg, ageFg = cIris, cText
		}
		g.put(in.x+2, y, truncate(n.Name, nameEnd-(in.x+2)), style{fg: nameFg, bold: sel})
		if allView && tagW > 0 {
			g.put(tagCol, y, truncate(n.Tag, tagW), fg(cFoam))
		}
		g.putRight(ageEnd, y, relTime(n.ModTime), fg(ageFg))
	}
}

func (m Model) centerKeyLine(g *grid, in rect, y int, before, key, after string) {
	w := textWidth(before + key + after)
	g.segs(in.x+max((in.w-w)/2, 0), y, seg{before, fg(cSubtle)}, seg{key, bold(cGold)}, seg{after, fg(cSubtle)})
}

type previewOpts struct {
	bg, border color
	title      string // defaults to the file name
	footer     string // defaults to the line count
	short      bool   // only tag and modified in the header
}

func (m Model) drawPreview(g *grid, r rect, o previewOpts) {
	if r.h < 3 || r.w < 12 {
		return
	}
	if o.border == 0 {
		o.border = cMuted
	}
	bg := o.bg
	labelBg := bg
	if labelBg == 0 {
		labelBg = cBase
	}
	g.box(r, o.border, bg)
	in := r.inner()
	p := m.prev

	titleFg := cSubtle
	if o.border != cMuted {
		titleFg = o.border
	}
	if p.path == "" {
		g.boxLabel(r, false, false, labelBg, seg{"Preview", bold(titleFg)})
		g.putCenter(in.x, in.w, in.y+in.h/2-1, "nothing selected", on(cSubtle, bg))
		return
	}
	title := o.title
	if title == "" {
		title = filepath.Base(p.path)
	}
	g.boxLabel(r, false, false, labelBg, seg{title, bold(titleFg)})
	if p.err != nil {
		g.put(in.x+1, in.y+1, "can't read this note", on(cLove, bg))
		g.put(in.x+1, in.y+2, truncate(p.err.Error(), in.w-2), on(cSubtle, bg))
		return
	}
	footer := o.footer
	if footer == "" {
		footer = plural(p.total, "line")
	}
	g.boxLabel(r, true, true, labelBg, seg{footer, fg(cSubtle)})

	n, _ := m.noteByPath(p.path)
	mod := p.modTime.Format("2006-01-02 15:04") + " · " + relTime(p.modTime)
	meta := [][2]string{{"path", p.rel}, {"tag", n.Tag}, {"modified", mod}, {"size", humanSize(p.size)}}
	if o.short {
		meta = [][2]string{{"tag", n.Tag}, {"modified", mod}}
	}
	y := in.y
	for _, kv := range meta {
		if y >= in.y+in.h {
			return
		}
		valueFg := cText
		if kv[0] == "tag" {
			valueFg = cFoam
		}
		g.put(in.x+1, y, kv[0], on(cSubtle, bg))
		g.put(in.x+10, y, truncate(kv[1], in.w-11), on(valueFg, bg))
		y++
	}
	if y < in.y+in.h {
		g.put(in.x, y, "╶"+strings.Repeat("─", max(in.w-2, 0))+"╴", on(cHLHigh, bg))
		y++
	}

	gutter := max(len(strconv.Itoa(p.total)), 3)
	textX := in.x + gutter + 2
	room := in.x + in.w - 1 - textX
	var h highlighter
	for i, line := range p.lines {
		if y >= in.y+in.h {
			break
		}
		g.putRight(in.x+gutter, y, strconv.Itoa(i+1), on(cSubtle, bg))
		x, used := textX, 0
		for _, s := range h.line(line) {
			if used >= room {
				break
			}
			text := s.text
			if used+textWidth(text) > room {
				text = truncate(text, room-used)
			}
			st := s.st
			st.bg = bg
			x = g.put(x, y, text, st)
			used += textWidth(text)
		}
		y++
	}
}

func (m Model) noteByPath(path string) (notes.Note, bool) {
	for _, n := range m.all {
		if n.Path == path {
			return n, true
		}
	}
	return notes.Note{}, false
}

// wordmark is the welcome pane's title. Every line is the same width so the
// rows stay aligned when each is centred on its own.
var wordmark = []string{
	"█▀▀▄ ▄▀▀▄ ▄█▄  ▄▀▀▄",
	"█  █ █  █  █   █▀▀▀",
	"▀  ▀  ▀▀   ▀▀   ▀▀▀",
}

func (m Model) drawWelcome(g *grid, r rect) {
	if r.h < 3 {
		return
	}
	g.box(r, cMuted, 0)
	g.boxLabel(r, false, false, cBase, seg{"Welcome", bold(cSubtle)})
	in := r.inner()
	fits := func(y int) bool { return y < in.y+in.h }

	editor := "your editor"
	if f := strings.Fields(m.cfg.Editor); len(f) > 0 {
		editor = filepath.Base(f[0])
	}
	root := tildify(m.store.Root)
	steps := [][]seg{
		{{"1  ", fg(cSubtle)}, {"Press ", fg(cText)}, {"n", bold(cGold)}, {" to create your first notebook (e.g. year-1).", fg(cText)}},
		{{"2  ", fg(cSubtle)}, {"Press ", fg(cText)}, {"space t n", bold(cGold)}, {" to add a tag (a module, e.g. cs118).", fg(cText)}},
		{{"3  ", fg(cSubtle)}, {"Press ", fg(cText)}, {"n", bold(cGold)}, {" again to write a note. It opens in " + editor + ".", fg(cText)}},
	}
	info := [][]seg{
		{{"notes    ", fg(cSubtle)}, {root, fg(cText)}},
		{{"template ", fg(cSubtle)}, {filepath.Join(root, ".template.typ"), fg(cText)}},
		{{"remote   ", fg(cSubtle)}, {"git -C " + root + " remote add origin <url>", fg(cText)}},
	}
	blockW := 0
	for _, line := range append(steps, info...) {
		w := 0
		for _, s := range line {
			w += textWidth(s.text)
		}
		blockW = max(blockW, w)
	}
	x := in.x + max((in.w-blockW)/2, 1)

	// Drop the wordmark first when the pane is short.
	y := in.y + max(min((in.h-20)/2, 5), 0)
	if in.h >= 20 {
		for i, line := range wordmark {
			g.putCenter(in.x, in.w, y+i, line, bold(cIris))
		}
		g.putCenter(in.x, in.w, y+len(wordmark)+1, "Typst lecture notes, one keystroke away.", fg(cSubtle))
		y += len(wordmark) + 4
	}
	for _, s := range steps {
		if fits(y) {
			g.segs(x, y, s...)
		}
		y += 2
	}
	if fits(y) {
		g.put(x, y, "╶"+strings.Repeat("─", max(blockW-2, 0))+"╴", fg(cHLHigh))
	}
	y += 2
	for _, s := range info {
		if fits(y) {
			g.segs(x, y, s...)
		}
		y++
	}
}

// ---------------------------------------------------------------- statusline

func (m Model) drawStatus(g *grid, y int) {
	md := m.ov.mode()
	g.clear(0, y, g.w, 1, cSurface)
	x := g.put(0, y, " "+md.label()+" ", onB(cBase, md.color())) + 1

	// Right-hand segments, laid out from the edge inwards.
	type part struct {
		text string
		st   style
	}
	var right []part
	if g.w >= sidebarMinWidth {
		if m.repo.Changed > 0 {
			right = append(right, part{fmt.Sprintf("+%d changed", m.repo.Changed), on(cGold, cSurface)})
		} else {
			right = append(right, part{"● clean", on(cFoam, cSurface)})
		}
		if m.repo.Ahead > 0 {
			right = append(right, part{fmt.Sprintf("↑%d", m.repo.Ahead), on(cFoam, cSurface)})
		}
		if m.repo.Branch != "" {
			right = append(right, part{m.repo.Branch, on(cSubtle, cSurface)})
		}
	}
	right = append(right, part{" " + m.position() + " ", onB(cBase, cSubtle)})
	end := g.w
	for i := len(right) - 1; i >= 0; i-- {
		start := end - textWidth(right[i].text)
		if start <= x+8 {
			break // never squeeze out the mode and breadcrumb
		}
		g.put(start, y, right[i].text, right[i].st)
		end = start - 1
		if i < len(right)-1 {
			end-- // wider gap between statusline segments
		}
	}
	room := end - 1

	crumbs := m.crumbs()
	for i, c := range crumbs {
		if i > 0 {
			x = g.put(x, y, " › ", on(cSubtle, cSurface))
		}
		last := i == len(crumbs)-1
		st := on(cSubtle, cSurface)
		if last {
			st = onB(cText, cSurface)
		}
		x = g.put(x, y, truncate(c, max(room-x, 0)), st)
		if x >= room {
			return
		}
	}

	if m.status != "" && !m.ov.active() {
		mark, c := m.statusLevel.mark()
		x += 4
		if room-x > 4 {
			g.put(x, y, truncate(mark+" "+m.status, room-x), on(c, cSurface))
		}
	}
}

func (m Model) crumbs() []string {
	if m.ov.kind == ovFinder {
		return []string{"find", m.ov.query()}
	}
	nb := m.currentNotebook()
	if nb == "" {
		return append([]string{"no notebook"}, nonEmpty(m.ov.crumb())...)
	}
	out := []string{nb}
	if m.focus == paneNotes || m.ov.active() {
		tag := m.selectedTag()
		if tag == "" {
			tag = "all"
		}
		out = append(out, tag)
		if n, ok := m.currentNote(); ok && m.focus == paneNotes {
			out = append(out, n.Name)
		}
	}
	return append(out, nonEmpty(m.ov.crumb())...)
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func (m Model) position() string {
	if m.ov.kind == ovFinder {
		if len(m.ov.matches) == 0 {
			return "0/0"
		}
		return fmt.Sprintf("%d/%d", m.ov.cursor+1, len(m.ov.matches))
	}
	cur, n := m.noteCur, len(m.visible)
	if m.focus == paneSidebar {
		cur, n = m.nbCur, len(m.notebooks)
	}
	if n == 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", cur+1, n)
}

// ---------------------------------------------------------------- hints

func drawHints(g *grid, y int, hs []hint) {
	x := 1
	for _, h := range hs {
		if x+textWidth(h.key)+1+textWidth(h.label) > g.w-1 {
			break // drop hints whole rather than cut one in half
		}
		x = g.put(x, y, h.key, bold(cText))
		x = g.put(x+1, y, h.label, fg(cSubtle)) + 3
	}
}

func (m Model) hints() []hint {
	switch m.ov.kind {
	case ovLeader:
		if m.ov.sub == "tag" {
			return []hint{{"esc", "cancel"}, {"backspace", "back"}, {"n r d", "tag action"}, {"[ ] 1-9", "switch tag"}}
		}
		return []hint{{"esc", "cancel"}, {"f n N t m r d s b ?", "pick an action"}}
	case ovPrompt:
		switch m.ov.purpose {
		case forNewNote:
			esc := "cancel"
			if m.ov.back {
				esc = "back"
			}
			return []hint{{"enter", "create & open"}, {"esc", esc}, {"ctrl+w", "delete word"}}
		case forRenameNotebook, forRenameTag, forRenameNote:
			return []hint{{"enter", "rename"}, {"esc", "cancel"}, {"ctrl+w", "delete word"}}
		}
		return []hint{{"enter", "create"}, {"esc", "cancel"}, {"ctrl+w", "delete word"}}
	case ovPicker:
		verb := "choose"
		if m.ov.purpose == forMoveNote {
			verb = "move"
		}
		return []hint{{"enter", verb}, {"↑↓ ctrl+n/p", "select"}, {"esc", "cancel"}}
	case ovFinder:
		return []hint{{"enter", "open"}, {"↑↓ ctrl+n/p", "select"}, {"ctrl+v", "move note"},
			{"ctrl+r", "rename"}, {"ctrl+d", "delete"}, {"esc", "close"}}
	case ovConfirm:
		return []hint{{"y", "delete"}, {"n esc", "cancel"}}
	case ovHelp:
		return []hint{{"any key", "close"}}
	}

	switch {
	case len(m.notebooks) == 0:
		return []hint{{"n", "new notebook"}, {"?", "keys"}, {"q", "quit"}}
	case m.focus == paneSidebar:
		return []hint{{"n", "new notebook"}, {"r", "rename"}, {"d", "delete"}, {"enter", "focus notes"},
			{"/", "find"}, {"space", "menu"}, {"?", "keys"}, {"q", "quit"}}
	case !m.l.hasSidebar():
		return []hint{{"/", "find"}, {"space", "menu"}, {"?", "keys"}, {"q", "quit"}}
	case m.gitFailed:
		return []hint{{"n", "new note"}, {"S", "sync"}, {"[ ]", "tag"}, {"space", "menu"}, {"?", "keys"}, {"q", "quit"}}
	}
	return []hint{{"n", "new"}, {"r", "rename"}, {"d", "delete"}, {"m", "move"}, {"/", "find"},
		{"[ ]", "tag"}, {"space", "menu"}, {"?", "keys"}, {"q", "quit"}}
}

// ---------------------------------------------------------------- overlays

func (m Model) drawOverlay(g *grid) {
	switch m.ov.kind {
	case ovLeader:
		m.drawLeader(g)
	case ovPrompt:
		m.drawPrompt(g)
	case ovPicker:
		m.drawPicker(g)
	case ovFinder:
		m.drawFinder(g)
	case ovConfirm:
		m.drawConfirm(g)
	case ovHelp:
		m.drawHelp(g)
	}
}

// paneRows is the first and last row a popup may use: everything between the
// tab bar and the statusline.
func (m Model) paneRows(g *grid) (top, bottom int) {
	if g.h < 5 {
		return 0, g.h - 1
	}
	return 1, g.h - 3
}

// popup centres a box of at most w columns and h rows, sitting above centre
// so the eye lands on it. A zero-width result means there is no room.
func (m Model) popup(g *grid, w, h int) rect {
	top, bottom := m.paneRows(g)
	avail := bottom - top + 1
	w, h = min(w, g.w-2), min(h, avail)
	if w < 24 || h < 3 {
		return rect{}
	}
	return rect{(g.w - w) / 2, top + max((avail-h)/3, 0), w, h}
}

// drawInput writes a prompt arrow, the typed text and a block cursor. Long
// text scrolls so the cursor stays visible.
func drawInput(g *grid, x, y, w int, in textinput.Model, arrow, bg color, placeholder string) {
	room := w - 2
	if room < 1 {
		return
	}
	g.put(x, y, "›", on(arrow, bg))
	rs := []rune(in.Value())
	if len(rs) == 0 {
		g.put(x+2, y, " ", on(cBase, cText))
		g.put(x+4, y, truncate(placeholder, room-4), on(cSubtle, bg))
		return
	}
	pos := min(in.Position(), len(rs))
	start := max(pos-room+1, 0)
	cx, end := x+2, x+2+room
	for i := start; i < len(rs) && cx < end; i++ {
		st := on(cText, bg)
		if i == pos {
			st = on(cBase, cText)
		}
		cx = g.put(cx, y, string(rs[i]), st)
	}
	if pos >= len(rs) && cx < end {
		g.put(cx, y, " ", on(cBase, cText))
	}
}

// rule draws the thin separator popups use between their input and body.
func rule(g *grid, x, y, w int, bg color) {
	if w >= 2 {
		g.put(x, y, "╶"+strings.Repeat("─", w-2)+"╴", on(cHLHigh, bg))
	}
}

// ---------------------------------------------------------------- leader

// The space menu is anchored bottom-right, like which-key: it annotates the
// keyboard rather than taking over the screen, so it never dims the panes.
func (m Model) drawLeader(g *grid) {
	items, title := leaderRoot, "space"
	if m.ov.sub == "tag" {
		items, title = leaderTag(m.currentNotebook(), m.selectedTag()), "space t"
	}
	top, bottom := m.paneRows(g)
	w := min(40, g.w-2)
	h := min(len(items)+2, bottom-top+1)
	if w < 24 || h < 3 {
		return
	}
	r := rect{max(g.w-w-1, 0), bottom - h + 1, w, h}
	g.box(r, cRose, cSurface)
	g.boxLabel(r, false, false, cSurface, seg{title, bold(cRose)})
	in := r.inner()
	for i, it := range items {
		y := in.y + i
		if y >= in.y+in.h {
			break
		}
		if it.key == "" {
			rule(g, in.x+1, y, in.w-2, cSurface)
			continue
		}
		g.putRight(in.x+4, y, it.key, onB(cGold, cSurface))
		hintW := textWidth(it.hint)
		g.put(in.x+6, y, truncate(it.label, in.w-8-hintW), on(cText, cSurface))
		g.putRight(in.x+in.w-1, y, it.hint, on(cSubtle, cSurface))
	}
}

// ---------------------------------------------------------------- prompt

func orDash(s string) string {
	if s == "" {
		return "…"
	}
	return s
}

// The prompt shows what the typed title will actually become on disk, so the
// slugging is never a surprise.
func (m Model) drawPrompt(g *grid) {
	o := m.ov
	slug := notes.Slugify(o.query())
	title, placeholder := "Input", "type a name"
	var rows [][2]string
	switch o.purpose {
	case forNewNotebook:
		title, placeholder = "New notebook", "e.g. year-1"
		rows = [][2]string{{"creates", filepath.Join(tildify(m.store.Root), orDash(slug)) + "/"}}
	case forNewTag:
		title, placeholder = "New tag", "e.g. cs118"
		rows = [][2]string{{"creates", o.notebook + "/" + orDash(slug) + "/"}}
	case forNewNote:
		title, placeholder = "New note", "e.g. B-Trees"
		rows = [][2]string{{"file", orDash(slug) + ".typ"}, {"in", o.notebook + "/" + o.tag + "/"}}
	case forRenameNotebook:
		title = "Rename notebook"
		rows = [][2]string{{"from", o.notebook}, {"to", orDash(slug)}}
	case forRenameTag:
		title = "Rename tag"
		rows = [][2]string{{"from", o.notebook + "/" + o.tag}, {"to", o.notebook + "/" + orDash(slug)}}
	case forRenameNote:
		title = "Rename note"
		rows = [][2]string{{"from", o.note.Name + ".typ"}, {"to", orDash(slug) + ".typ"}}
	}

	r := m.popup(g, 56, len(rows)+5)
	if r.w == 0 {
		return
	}
	g.box(r, cGold, cSurface)
	g.boxLabel(r, false, false, cSurface, seg{title, bold(cGold)})
	in := r.inner()
	drawInput(g, in.x+1, in.y, in.w-2, o.input, cGold, cSurface, placeholder)
	rule(g, in.x+1, in.y+1, in.w-2, cSurface)
	for i, kv := range rows {
		y := in.y + 2 + i
		if y >= in.y+in.h {
			break
		}
		g.put(in.x+1, y, kv[0], on(cSubtle, cSurface))
		g.put(in.x+10, y, truncate(kv[1], in.w-11), on(cText, cSurface))
	}
}

// ---------------------------------------------------------------- picker

const pickerRows = 8

func (m Model) drawPicker(g *grid) {
	o := m.ov
	title, placeholder, footer := "Pick one", "filter", ""
	switch o.purpose {
	case forPickNewNoteTag:
		title, placeholder = "New note · which tag?", "filter tags"
	case forPickRenameTag:
		title, placeholder = "Rename tag · which one?", "filter tags"
	case forPickDeleteTag:
		title, placeholder = "Delete tag · which one?", "filter tags"
	case forMoveNote:
		title, placeholder = "Move "+o.note.Name+".typ", "filter destinations"
		dest := "…"
		if i := o.selected(); i >= 0 {
			dest = o.items[i].value + "/" + o.note.Name + ".typ"
		}
		footer = "from " + o.note.Notebook + "/" + o.note.Tag + " → " + dest
	}

	rows := min(max(len(o.matches), 1), pickerRows)
	r := m.popup(g, 64, rows+5)
	if r.w == 0 {
		return
	}
	g.box(r, cGold, cSurface)
	g.boxLabel(r, false, false, cSurface, seg{title, bold(cGold)})
	if footer != "" {
		g.boxLabel(r, true, false, cSurface, seg{truncate(footer, r.w-6), fg(cSubtle)})
	}
	in := r.inner()

	count := fmt.Sprintf("%d/%d", len(o.matches), len(o.items))
	g.putRight(in.x+in.w-1, in.y, count, on(cSubtle, cSurface))
	drawInput(g, in.x+1, in.y, in.w-3-textWidth(count), o.input, cGold, cSurface, placeholder)
	rule(g, in.x+1, in.y+1, in.w-2, cSurface)

	if len(o.matches) == 0 {
		g.putCenter(in.x, in.w, in.y+3, "no match", on(cSubtle, cSurface))
		return
	}
	top := in.y + 2
	start, end := window(o.cursor, len(o.matches), min(rows, in.y+in.h-top))
	for i := start; i < end; i++ {
		y := top + i - start
		mt := o.matches[i]
		item := o.items[mt.index]
		sel := i == o.cursor
		bg := cSurface
		if sel {
			bg = cHLMed
			g.fill(in.x, y, in.w, 1, bg)
			g.put(in.x, y, "▎", on(cGold, bg))
		}
		label := o.labels[mt.index]
		dim := strings.LastIndex(label, "/") + 1
		base := on(cText, bg)
		if sel {
			base.bold = true
		}
		g.putMatch(in.x+2, y, truncate(label, in.w-14), mt, base, dim, cSubtle)
		g.putRight(in.x+in.w-1, y, plural(item.count, "note"), on(cSubtle, bg))
	}
}

// ---------------------------------------------------------------- finder

// The finder is laid out bottom-up, Telescope style: the prompt sits at the
// bottom with the best match directly above it, so the eye never travels.
func (m Model) drawFinder(g *grid) {
	o := m.ov
	top, bottom := m.paneRows(g)
	if bottom-top < 5 || g.w < 24 {
		return
	}
	listW := g.w - 2
	var prevR rect
	if g.w >= previewMinWidth {
		listW = max(g.w*45/100, 44)
		prevR = rect{1 + listW + 1, top, g.w - listW - 3, bottom - top + 1}
		// The gutter between the two boxes would otherwise show a stripe of
		// the dimmed screen underneath.
		g.clear(prevR.x-1, top, 1, prevR.h, cBase)
	}
	const promptH = 3
	listR := rect{1, top, listW, bottom - top + 1 - promptH}
	promptR := rect{1, bottom - promptH + 1, listW, promptH}

	g.box(listR, cFoam, cSurface)
	g.boxLabel(listR, false, false, cSurface, seg{"Find", bold(cFoam)},
		seg{fmt.Sprintf("%d/%d", len(o.matches), len(o.candidates)), fg(cSubtle)})
	in := listR.inner()
	switch {
	case len(o.candidates) == 0:
		g.putCenter(in.x, in.w, in.y+in.h/2, "no notes yet", on(cSubtle, cSurface))
	case len(o.matches) == 0:
		g.putCenter(in.x, in.w, in.y+in.h/2, "no note matches "+o.query(), on(cSubtle, cSurface))
	}
	showAge := in.w >= 48
	start, end := window(o.cursor, len(o.matches), in.h)
	for i := start; i < end; i++ {
		y := in.y + in.h - 1 - (i - start) // best match nearest the prompt
		mt := o.matches[i]
		n := o.candidates[mt.index]
		sel := i == o.cursor
		bg := cSurface
		if sel {
			bg = cHLMed
			g.fill(in.x, y, in.w, 1, bg)
			g.put(in.x, y, "▎", on(cFoam, bg))
		}
		room := in.w - 3
		if showAge {
			room -= 9
			g.putRight(in.x+in.w-1, y, relTime(n.ModTime), on(cSubtle, bg))
		}
		base := on(cText, bg)
		if sel {
			base.bold = true
		}
		// Dim the notebook/tag prefix so the note names read as a column.
		dim := len(n.Notebook) + len(n.Tag) + 2
		g.putMatch(in.x+2, y, truncate(o.labels[mt.index], room), mt, base, dim, cSubtle)
	}

	g.box(promptR, cFoam, cSurface)
	pin := promptR.inner()
	drawInput(g, pin.x+1, pin.y, pin.w-2, o.input, cFoam, cSurface, "search every note")

	if prevR.w > 0 {
		m.drawPreview(g, prevR, previewOpts{bg: cSurface, border: cFoam, short: true})
	}
}

// ---------------------------------------------------------------- confirm

const confirmItems = 5

func (m Model) drawConfirm(g *grid) {
	o := m.ov
	var title, head string
	var items []string
	switch o.purpose {
	case forDeleteNotebook:
		title = "Delete notebook"
		head = "Delete " + o.notebook + " and everything inside it?"
		tags, _ := m.store.Tags(o.notebook)
		for _, t := range tags {
			items = append(items, fmt.Sprintf("%-18s %s", truncate(t, 18), plural(m.countNotes(o.notebook, t), "note")))
		}
	case forDeleteTag:
		title = "Delete tag"
		head = "Delete " + o.notebook + "/" + o.tag + " and its notes?"
		for _, n := range m.notesIn(o.notebook, o.tag) {
			items = append(items, n.Name+".typ")
		}
	case forDeleteNote:
		title = "Delete note"
		head = "Delete " + o.note.Name + ".typ?"
		items = []string{o.note.Notebook + "/" + o.note.Tag + "/" + o.note.Name + ".typ"}
	default:
		return
	}
	more := 0
	if len(items) > confirmItems {
		more, items = len(items)-confirmItems, items[:confirmItems]
	}

	body := 5 + len(items) // head, blanks, restore note, buttons
	if len(items) > 0 {
		body++ // blank line between the question and the list
	}
	if more > 0 {
		body++
	}
	r := m.popup(g, 60, body+3)
	if r.w == 0 {
		return
	}
	g.box(r, cLove, cSurface)
	g.boxLabel(r, false, false, cSurface, seg{title, bold(cLove)})
	in := r.inner()
	y, last := in.y+1, in.y+in.h
	line := func(s string, st style) {
		if y < last {
			st.bg = cSurface
			g.put(in.x+2, y, truncate(s, in.w-4), st)
		}
		y++
	}
	line(head, bold(cText))
	if len(items) > 0 {
		y++
		for _, it := range items {
			line("  "+it, fg(cSubtle))
		}
		if more > 0 {
			line(fmt.Sprintf("  +%d more", more), fg(cSubtle))
		}
	}
	y++
	line("Committed notes can be restored from git.", fg(cSubtle))
	y++
	if y < last {
		x := g.put(in.x+2, y, " y ", onB(cBase, cLove))
		x = g.put(x+1, y, "delete", on(cText, cSurface)) + 3
		x = g.put(x, y, " n ", onB(cText, cHLHigh))
		g.put(x+1, y, "cancel", on(cSubtle, cSurface))
	}
}

// ---------------------------------------------------------------- help

const helpColWidth = 33

func (m Model) drawHelp(g *grid) {
	bands := helpGroups()
	bandHeight := func(b []helpGroup) int {
		n := 0
		for _, gr := range b {
			n = max(n, len(gr.rows))
		}
		return n + 1 // the group titles
	}
	h := 2 + 1 + 3 // borders, top padding, two legend rows and a blank
	for _, b := range bands {
		h += bandHeight(b) + 1
	}
	r := m.popup(g, 3*helpColWidth+4, h)
	if r.w == 0 {
		return
	}
	g.box(r, cSubtle, cSurface)
	g.boxLabel(r, false, false, cSurface, seg{"Keys", bold(cText)})
	g.boxLabel(r, true, true, cSurface, seg{"note " + m.version, fg(cSubtle)})
	in := r.inner()
	colW := min(helpColWidth, in.w/3)
	y, last := in.y+1, in.y+in.h

	for _, band := range bands {
		for i, gr := range band {
			x := in.x + 1 + i*colW
			if x+8 > in.x+in.w {
				break
			}
			room := colW - 2
			if y < last {
				g.put(x, y, truncate(gr.title, room), bold(cIris))
			}
			for j, b := range gr.rows {
				ry := y + 1 + j
				if ry >= last || b.label == "" {
					continue
				}
				g.put(x, ry, truncate(b.label, 9), onB(cGold, cSurface))
				g.put(x+10, ry, truncate(b.help, colW-11), on(cText, cSurface))
			}
		}
		y += bandHeight(band) + 1
	}

	if y+1 >= last {
		return
	}
	legend := func(y int, key string, draw func(x int) int) {
		g.put(in.x+1, y, key, on(cSubtle, cSurface))
		draw(in.x + 9)
	}
	legend(y, "mode", func(x int) int {
		for _, md := range []struct {
			m    mode
			what string
		}{{modeNormal, "normal"}, {modeLeader, "menu"}, {modeFind, "find"},
			{modeInput, "typing"}, {modeConfirm, "delete"}} {
			x = g.put(x, y, " "+md.m.label()+" ", onB(cBase, md.m.color()))
			x = g.put(x+1, y, md.what, on(cSubtle, cSurface)) + 3
		}
		return x
	})
	legend(y+1, "git", func(x int) int {
		for _, st := range []struct {
			s    gitState
			what string
		}{{gitClean, "clean"}, {gitDirty, "uncommitted"}, {gitFailed, "last sync failed"}} {
			mark, c := st.s.mark()
			x = g.put(x, y+1, mark, on(c, cSurface))
			x = g.put(x+2, y+1, st.what, on(cSubtle, cSurface)) + 3
		}
		return x
	})
}

// ---------------------------------------------------------------- helpers

// relTime renders a time as a short age, e.g. "2d ago".
func relTime(t time.Time) string {
	d := nowFunc().Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
