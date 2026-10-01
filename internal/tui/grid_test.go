package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestGridRowsAreExactlyWidth(t *testing.T) {
	g := newGrid(20, 3)
	g.put(0, 0, strings.Repeat("x", 50), fg(cText)) // clipped
	g.put(18, 1, "日本", fg(cText))                   // second wide rune doesn't fit
	g.box(rect{0, 0, 20, 3}, cMuted, 0)
	for i, line := range strings.Split(g.String(), "\n") {
		if w := ansi.StringWidth(line); w != 20 {
			t.Errorf("row %d is %d cells wide, want 20: %q", i, w, line)
		}
	}
}

func TestGridPutHandlesWideRunes(t *testing.T) {
	g := newGrid(6, 1)
	next := g.put(0, 0, "日本x", fg(cText))
	if next != 5 {
		t.Errorf("next column = %d, want 5", next)
	}
	if got := g.plain(); got != "日本x " {
		t.Errorf("plain = %q", got)
	}
}

func TestGridPutReplacesControlCharacters(t *testing.T) {
	g := newGrid(4, 1)
	g.put(0, 0, "a\tb\x1b", fg(cText))
	if got := g.plain(); got != "a b " {
		t.Errorf("plain = %q, want control characters blanked", got)
	}
}

func TestGridPutKeepsBackgroundUnlessGiven(t *testing.T) {
	g := newGrid(4, 1)
	g.fill(0, 0, 4, 1, cHLMed)
	g.put(0, 0, "ab", fg(cBlue))
	if c := g.at(0, 0); c.st.bg != cHLMed || c.st.fg != cBlue {
		t.Errorf("cell style = %+v, want iris on the highlight", c.st)
	}
}

func TestBoxLabelIsClippedBetweenCorners(t *testing.T) {
	g := newGrid(16, 3)
	r := rect{0, 0, 16, 3}
	g.box(r, cMuted, 0)
	g.boxLabel(r, false, false, cBase, seg{"a-very-long-title", bold(cBlue)}, seg{"12", fg(cSubtle)})
	top := strings.Split(g.plain(), "\n")[0]
	if !strings.HasPrefix(top, "╭─ ") || !strings.HasSuffix(top, "─╮") {
		t.Errorf("label broke the border: %q", top)
	}
	if !strings.Contains(top, "…") {
		t.Errorf("long label was not truncated: %q", top)
	}
}

func TestDimFlattensEverything(t *testing.T) {
	g := newGrid(3, 1)
	g.put(0, 0, "abc", onB(cRed, cSurface))
	g.dim()
	for x := 0; x < 3; x++ {
		c := g.at(x, 0)
		if c.st.fg != cHLMed || c.st.bg != cHLLow || c.st.bold {
			t.Fatalf("cell %d = %+v, want a dimmed cell", x, c.st)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		w    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"日本語", 4, "日…"},
		{"x", 0, ""},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.w); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
		}
	}
}
