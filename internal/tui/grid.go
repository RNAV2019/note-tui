package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// The whole screen is drawn into a grid of cells and serialised once per
// frame. Overlays paint over the dimmed screen beneath them, which a
// string-joining renderer can't do without re-measuring ANSI text.

// style is how a run of text is painted. A zero fg or bg leaves the cell's
// existing colour alone, so text can be written over a highlight bar.
type style struct {
	fg, bg color
	bold   bool
}

type cell struct {
	r    rune
	st   style
	cont bool // right half of a wide rune; never printed itself
}

type grid struct {
	w, h  int
	cells [][]cell
}

func newGrid(w, h int) *grid {
	g := &grid{w: max(w, 0), h: max(h, 0)}
	g.cells = make([][]cell, g.h)
	for y := range g.cells {
		g.cells[y] = make([]cell, g.w)
		for x := range g.cells[y] {
			g.cells[y][x] = cell{r: ' ', st: style{fg: cText, bg: cBase}}
		}
	}
	return g
}

func (g *grid) at(x, y int) *cell {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return nil
	}
	return &g.cells[y][x]
}

// put writes s starting at column x and returns the column after it. Text is
// clipped at the grid edge, and a wide rune that doesn't fit is dropped whole.
func (g *grid) put(x, y int, s string, st style) int {
	for _, r := range s {
		if r < ' ' || r == 0x7f {
			r = ' '
		}
		rw := runewidth.RuneWidth(r)
		if rw == 0 {
			continue // combining marks would desync the column count
		}
		if x+rw > g.w {
			return x + rw
		}
		for i := 0; i < rw; i++ {
			g.splitWide(x+i, y)
			c := g.at(x+i, y)
			if c == nil {
				continue
			}
			c.r, c.cont = r, i > 0
			if st.fg != 0 {
				c.st.fg = st.fg
			}
			if st.bg != 0 {
				c.st.bg = st.bg
			}
			c.st.bold = st.bold
		}
		x += rw
	}
	return x
}

// splitWide blanks the other half of the wide rune covering (x, y). Writing
// over one column of a wide rune would otherwise leave a stray half behind
// and push the rest of the row along by a cell.
func (g *grid) splitWide(x, y int) {
	c := g.at(x, y)
	if c == nil {
		return
	}
	if c.cont {
		if lead := g.at(x-1, y); lead != nil {
			lead.r, lead.cont = ' ', false
		}
		return
	}
	if runewidth.RuneWidth(c.r) == 2 {
		if tail := g.at(x+1, y); tail != nil && tail.cont {
			tail.r, tail.cont = ' ', false
		}
	}
}

// putRight writes s so that it ends just before column end.
func (g *grid) putRight(end, y int, s string, st style) int {
	return g.put(end-textWidth(s), y, s, st)
}

// putCenter writes s centred within [x, x+w).
func (g *grid) putCenter(x, w, y int, s string, st style) {
	g.put(x+max((w-textWidth(s))/2, 0), y, s, st)
}

// fill paints the background of a rectangle without touching its text.
func (g *grid) fill(x, y, w, h int, bg color) {
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			if c := g.at(col, row); c != nil {
				c.st.bg = bg
			}
		}
	}
}

// clear blanks a rectangle to bg.
func (g *grid) clear(x, y, w, h int, bg color) {
	for row := y; row < y+h; row++ {
		g.splitWide(x, row)
		g.splitWide(x+w-1, row)
		for col := x; col < x+w; col++ {
			if c := g.at(col, row); c != nil {
				*c = cell{r: ' ', st: style{fg: cText, bg: bg}}
			}
		}
	}
}

// box draws a rounded border around r. With a non-zero bg the interior is
// cleared first, which is how popups hide what's underneath.
func (g *grid) box(r rect, border, bg color) {
	if r.w < 2 || r.h < 2 {
		return
	}
	if bg != 0 {
		g.clear(r.x, r.y, r.w, r.h, bg)
	}
	st := style{fg: border, bg: bg}
	g.put(r.x, r.y, "╭"+strings.Repeat("─", r.w-2)+"╮", st)
	g.put(r.x, r.y+r.h-1, "╰"+strings.Repeat("─", r.w-2)+"╯", st)
	for y := r.y + 1; y < r.y+r.h-1; y++ {
		g.put(r.x, y, "│", st)
		g.put(r.x+r.w-1, y, "│", st)
	}
}

// boxLabel writes " text " into a box's top (or bottom) edge. Segments are
// joined with single spaces; the label is clipped to fit between the corners.
func (g *grid) boxLabel(r rect, bottom, right bool, bg color, segs ...seg) {
	y := r.y
	if bottom {
		y = r.y + r.h - 1
	}
	room := r.w - 6 // corner + dash + space on both sides
	if room <= 0 {
		return
	}
	var parts []seg
	width := 0
	for i, s := range segs {
		sep := 0
		if i > 0 {
			sep = 1
		}
		if width+sep+textWidth(s.text) > room {
			if left := room - width - sep; left > 1 {
				parts = append(parts, seg{truncate(s.text, left), s.st})
				width += sep + left
			}
			break
		}
		parts = append(parts, s)
		width += sep + textWidth(s.text)
	}
	x := r.x + 2
	if right {
		x = r.x + r.w - 4 - width
	}
	x = g.put(x, y, " ", style{bg: bg})
	for i, p := range parts {
		if i > 0 {
			x = g.put(x, y, " ", style{bg: bg})
		}
		p.st.bg = bg
		x = g.put(x, y, p.text, p.st)
	}
	g.put(x, y, " ", style{bg: bg})
}

// seg is a styled run of text.
type seg struct {
	text string
	st   style
}

func (g *grid) segs(x, y int, parts ...seg) int {
	for _, p := range parts {
		x = g.put(x, y, p.text, p.st)
	}
	return x
}

// dim turns the whole grid into an inert backdrop for an open popup.
func (g *grid) dim() {
	for y := range g.cells {
		for x := range g.cells[y] {
			c := &g.cells[y][x]
			c.st.fg, c.st.bold = cHLMed, false
			if c.st.bg != cBase {
				c.st.bg = cHLLow
			}
		}
	}
}

// String renders the grid as ANSI text, one line per row, emitting SGR codes
// only where the style changes.
func (g *grid) String() string {
	var b strings.Builder
	for y, row := range g.cells {
		if y > 0 {
			b.WriteByte('\n')
		}
		var cur style
		first := true
		for _, c := range row {
			if c.cont {
				continue
			}
			if first || c.st != cur {
				b.WriteString(sgr(c.st))
				cur, first = c.st, false
			}
			b.WriteRune(c.r)
		}
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// plain returns the grid's text without styling, for tests.
func (g *grid) plain() string {
	var b strings.Builder
	for y, row := range g.cells {
		if y > 0 {
			b.WriteByte('\n')
		}
		for _, c := range row {
			if !c.cont {
				b.WriteRune(c.r)
			}
		}
	}
	return b.String()
}

func sgr(st style) string {
	fg, bg := palette[st.fg], palette[st.bg]
	s := fmt.Sprintf("\x1b[0;38;2;%d;%d;%d;48;2;%d;%d;%d", fg.r, fg.g, fg.b, bg.r, bg.g, bg.b)
	if st.bold {
		s += ";1"
	}
	return s + "m"
}

// textWidth is the number of cells s occupies.
func textWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runewidth.RuneWidth(r)
	}
	return w
}

// truncate cuts s to at most width cells, ending in "…" if anything was cut.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if textWidth(s) <= width {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > width-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

// rect is a screen region in cells.
type rect struct{ x, y, w, h int }

// inner is the region inside a box border.
func (r rect) inner() rect { return rect{r.x + 1, r.y + 1, r.w - 2, r.h - 2} }
