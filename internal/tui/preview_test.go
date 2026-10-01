package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "uni", "algos", "b-trees.typ")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("= B-Trees\n\tindented\r\nlast"), 0o644)

	p := loadPreview(root, path, time.Now())
	if p.err != nil {
		t.Fatal(p.err)
	}
	if p.rel != "uni/algos/b-trees.typ" {
		t.Errorf("rel = %q", p.rel)
	}
	if p.total != 3 || len(p.lines) != 3 {
		t.Errorf("total = %d, lines = %d, want 3 and 3", p.total, len(p.lines))
	}
	if p.lines[1] != "  indented" {
		t.Errorf("tabs and CRLF not normalised: %q", p.lines[1])
	}
}

func TestLoadPreviewMissingFile(t *testing.T) {
	p := loadPreview(t.TempDir(), "/nope/missing.typ", time.Now())
	if p.err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{12: "12 B", 4300: "4.2 KB", 2048: "2 KB", 3 << 20: "3 MB"} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

// colourOf returns the colour the highlighter gave to the first occurrence of
// needle in the line.
func colourOf(t *testing.T, segs []seg, needle string) style {
	t.Helper()
	for _, s := range segs {
		if strings.Contains(s.text, needle) {
			return s.st
		}
	}
	t.Fatalf("%q not found in %+v", needle, segs)
	return style{}
}

func TestHighlighter(t *testing.T) {
	var h highlighter
	line := h.line(`#set page(margin: 2cm, title: "x") // note`)
	if st := colourOf(t, line, "#set"); st.fg != cBlue {
		t.Errorf("#set = %+v, want iris", st)
	}
	if st := colourOf(t, line, "margin"); st.fg != cCyan {
		t.Errorf("named argument = %+v, want foam", st)
	}
	if st := colourOf(t, line, "2cm"); st.fg != cMagenta {
		t.Errorf("length = %+v, want rose", st)
	}
	if st := colourOf(t, line, `"x"`); st.fg != cYellow {
		t.Errorf("string = %+v, want gold", st)
	}
	if st := colourOf(t, line, "// note"); st.fg != cSubtle {
		t.Errorf("comment = %+v, want subtle", st)
	}

	if st := colourOf(t, h.line("== Definition"), "Definition"); st.fg != cRed || !st.bold {
		t.Errorf("heading = %+v, want bold love", st)
	}
	prose := h.line("A *B-tree* of order $m$ lets you search")
	if st := colourOf(t, prose, "*B-tree*"); !st.bold {
		t.Errorf("strong = %+v, want bold", st)
	}
	if st := colourOf(t, prose, "$m$"); st.fg != cCyan {
		t.Errorf("math = %+v, want foam", st)
	}
	if st := colourOf(t, prose, "lets you"); st.fg != cText {
		t.Errorf("prose keywords must not be highlighted: %+v", st)
	}
}

func TestHighlighterTracksCodeBlocksAcrossLines(t *testing.T) {
	var h highlighter
	h.line("#let search(node, k) = {")
	if st := colourOf(t, h.line("  let i = 0"), "let"); st.fg != cBlue {
		t.Errorf("keyword inside a code block = %+v, want iris", st)
	}
	h.line("}")
	if st := colourOf(t, h.line("let it be"), "let"); st.fg != cText {
		t.Errorf("after the block closes, prose = %+v, want plain text", st)
	}
}
