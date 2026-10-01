package tui

import (
	imgcolor "image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	colorful "github.com/lucasb-eyer/go-colorful"
)

var (
	rosePineBase = imgcolor.RGBA{0x19, 0x17, 0x24, 0xff}
	rosePineText = imgcolor.RGBA{0xe0, 0xde, 0xf4, 0xff}
)

// withTerminalColours feeds the model the replies to its colour queries.
func withTerminalColours(t *testing.T, m Model, bg, fg imgcolor.Color) Model {
	t.Helper()
	next, _ := m.Update(tea.BackgroundColorMsg{Color: bg})
	next, _ = next.(Model).Update(tea.ForegroundColorMsg{Color: fg})
	return next.(Model)
}

func TestTextAndAccentsUseTheTerminalScheme(t *testing.T) {
	for name, p := range map[string]palette{
		"unknown":  fallbackPalette,
		"reported": newPalette(rosePineBase, rosePineText),
	} {
		t.Run(name, func(t *testing.T) {
			want := map[color]string{
				cText: "39", cRed: "31", cYellow: "33", cBlue: "34", cMagenta: "35", cCyan: "36",
			}
			for c, code := range want {
				if got := p[c].code(false); got != code {
					t.Errorf("colour %d paints as %q, want %q", c, got, code)
				}
			}
			if got := p[cBase].code(true); got != "49" {
				t.Errorf("the background paints as %q, want the terminal default", got)
			}
		})
	}
}

func TestNeutralsWaitForTheTerminal(t *testing.T) {
	m := newTestModel(t)
	if m.pal != fallbackPalette {
		t.Fatal("a fresh model should use the fallback palette")
	}
	m = withTerminalColours(t, m, rosePineBase, rosePineText)
	if m.pal[cSurface].kind != inkRGB {
		t.Error("surface was not blended from the reported colours")
	}
}

// The blend keeps Rosé Pine's lightness ladder; its extra purple tint is the
// scheme's own and isn't carried over.
func TestShadesReproduceRosePine(t *testing.T) {
	p := newPalette(rosePineBase, rosePineText)
	want := map[color]string{
		cSurface: "#1f1d2e", cHLMed: "#403d52", cMuted: "#6e6a86", cSubtle: "#908caa",
	}
	for c, hex := range want {
		w, _ := colorful.Hex(hex)
		k := p[c]
		got := colorful.Color{R: float64(k.r) / 255, G: float64(k.g) / 255, B: float64(k.b) / 255}
		gl, _, _ := got.OkLab()
		wl, _, _ := w.OkLab()
		if d := gl - wl; d > 0.01 || d < -0.01 {
			t.Errorf("colour %d is %s, want the lightness of %s (off by %.3f)", c, got.Hex(), hex, d)
		}
	}
}

// Low-contrast schemes push secondary text towards the foreground until it
// is readable again.
func TestSubtleTextStaysReadable(t *testing.T) {
	schemes := map[string][2]string{
		"rose pine":        {"#191724", "#e0def4"},
		"rose pine dawn":   {"#faf4ed", "#575279"},
		"catppuccin latte": {"#eff1f5", "#4c4f69"},
		"gruvbox":          {"#282828", "#ebdbb2"},
		"solarized light":  {"#fdf6e3", "#586e75"},
		"black on white":   {"#ffffff", "#000000"},
	}
	for name, s := range schemes {
		t.Run(name, func(t *testing.T) {
			bg, _ := colorful.Hex(s[0])
			fg, _ := colorful.Hex(s[1])
			p := newPalette(bg, fg)
			for _, c := range []struct {
				c   color
				min float64
			}{{cSubtle, 4.5}, {cMuted, 3}} {
				k := p[c.c]
				got := colorful.Color{R: float64(k.r) / 255, G: float64(k.g) / 255, B: float64(k.b) / 255}
				if r := contrast(got, bg); r < c.min {
					t.Errorf("colour %d is %s, only %.2f:1 on %s", c.c, got.Hex(), r, s[0])
				}
			}
		})
	}
}

// Pills cut their label out of the accent, so the label must be the
// terminal's own background, which only reverse video can name.
func TestPillLabelsUseReverseVideo(t *testing.T) {
	got := fallbackPalette.sgr(onB(cBase, cBlue))
	if got != "\x1b[0;34;49;7;1m" {
		t.Errorf("pill SGR is %q", strings.ReplaceAll(got, "\x1b", "ESC"))
	}
}

// The preview window's backdrop matches the terminal, so it loads as a blank
// pane in the terminal's own colour rather than tinymist's gray.
func TestPreviewBackdropIsTheTerminalBackground(t *testing.T) {
	m := newTestModel(t)
	if got := m.newEditSession(m.all[0]).opts.Backdrop; got != "" {
		t.Errorf("backdrop before the terminal answers = %q, want tinymist's own", got)
	}
	m = withTerminalColours(t, m, rosePineBase, rosePineText)
	if got := m.newEditSession(m.all[0]).opts.Backdrop; got != "#191724" {
		t.Errorf("backdrop = %q, want #191724", got)
	}
}
