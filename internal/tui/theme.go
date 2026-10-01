package tui

import (
	"fmt"
	imgcolor "image/color"
	"strconv"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// Colours come from the terminal, not from a built-in scheme. Text and the
// app background are the terminal's own defaults, so a transparent background
// stays transparent, and accents are ANSI palette slots, so they follow
// whatever scheme the terminal is set to. The neutrals in between are blended
// from the terminal's background towards its foreground once it reports them;
// on Rosé Pine's base and text the weights reproduce Rosé Pine's own ladder.
// Secondary text is pushed towards the foreground until it clears 4.5:1 and
// idle borders until they clear 3:1, if the scheme has the range for it.
// Dimmed popup backdrops are deliberately below that bar: they are inert.

type color uint8

const (
	cNone    color = iota // leave the cell's colour unchanged
	cBase                 // app background
	cSurface              // bars and popups
	cOverlay              // borders of a dimmed backdrop
	cHLLow                // parked selection
	cHLMed                // active selection
	cHLHigh               // rules and separators
	cMuted                // idle pane borders
	cSubtle               // secondary text
	cText                 // primary text
	cRed                  // danger, errors, CONFIRM
	cYellow               // keys, INPUT, uncommitted
	cMagenta              // leader, fuzzy matches, SPC
	cCyan                 // tags, clean, FIND
	cBlue                 // focus, NOR
	numColors
)

// ink is how a colour is named to the terminal. The zero ink is the
// terminal's default foreground or background.
type ink struct {
	kind    inkKind
	n       uint8 // palette slot, for inkANSI
	r, g, b uint8 // for inkRGB
}

type inkKind uint8

const (
	inkDefault inkKind = iota
	inkANSI
	inkRGB
)

func slot(n uint8) ink { return ink{kind: inkANSI, n: n} }

// code is the SGR parameter that paints k as a foreground or background.
func (k ink) code(bg bool) string {
	switch k.kind {
	case inkANSI:
		n := 30 + int(k.n)
		if k.n >= 8 {
			n = 90 + int(k.n) - 8
		}
		if bg {
			n += 10
		}
		return strconv.Itoa(n)
	case inkRGB:
		lead := "38"
		if bg {
			lead = "48"
		}
		return fmt.Sprintf("%s;2;%d;%d;%d", lead, k.r, k.g, k.b)
	default:
		if bg {
			return "49"
		}
		return "39"
	}
}

// palette maps every colour to the ink that paints it.
type palette [numColors]ink

// fallbackPalette is used until the terminal reports its colours, and for
// good if it never does. Without knowing the background, the only safe
// neutral is bright black, so parked selections lean on their markers.
var fallbackPalette = func() palette {
	var p palette
	for _, c := range []color{cOverlay, cHLMed, cHLHigh, cMuted, cSubtle} {
		p[c] = slot(8)
	}
	p[cRed], p[cYellow], p[cBlue], p[cMagenta], p[cCyan] = slot(1), slot(3), slot(4), slot(5), slot(6)
	return p
}()

// shades place each neutral between the background (0) and foreground (1) in
// OKLab, so light and dark schemes get the same perceived steps. minContrast,
// where set, is the ratio the shade must clear against base and surface.
var shades = []struct {
	c           color
	mix         float64
	minContrast float64
}{
	{cSurface, 0.040, 0},
	{cOverlay, 0.083, 0},
	{cHLLow, 0.054, 0},
	{cHLMed, 0.228, 0},
	{cHLHigh, 0.326, 0},
	{cMuted, 0.467, 3},
	{cSubtle, 0.633, 4.5},
}

// newPalette derives the neutrals from the terminal's background and
// foreground. Text, background and accents stay as the terminal paints them.
func newPalette(bg, fg imgcolor.Color) palette {
	p := fallbackPalette
	if bg == nil || fg == nil {
		return p
	}
	b, ok := colorful.MakeColor(bg)
	if !ok {
		return p
	}
	f, ok := colorful.MakeColor(fg)
	if !ok {
		return p
	}
	blend := func(t float64) colorful.Color { return b.BlendOkLab(f, t).Clamped() }
	surface := blend(shades[0].mix)
	for _, s := range shades {
		t := s.mix
		c := blend(t)
		for t < 1 && (contrast(c, b) < s.minContrast || contrast(c, surface) < s.minContrast) {
			t = min(t+0.01, 1)
			c = blend(t)
		}
		r, g, bl := c.RGB255()
		p[s.c] = ink{kind: inkRGB, r: r, g: g, b: bl}
	}
	return p
}

// contrast is the WCAG contrast ratio between two colours.
func contrast(a, b colorful.Color) float64 {
	la, lb := luminance(a)+0.05, luminance(b)+0.05
	return max(la, lb) / min(la, lb)
}

func luminance(c colorful.Color) float64 {
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func fg(c color) style     { return style{fg: c} }
func bold(c color) style   { return style{fg: c, bold: true} }
func on(f, b color) style  { return style{fg: f, bg: b} }
func onB(f, b color) style { return style{fg: f, bg: b, bold: true} }

// mode is what the statusline pill says the keyboard is doing.
type mode int

const (
	modeNormal mode = iota
	modeLeader
	modeFind
	modeInput
	modeConfirm
	modeKeys
)

func (m mode) label() string {
	return [...]string{"NOR", "SPC", "FIND", "INPUT", "CONFIRM", "KEYS"}[m]
}

func (m mode) color() color {
	return [...]color{cBlue, cMagenta, cCyan, cYellow, cRed, cSubtle}[m]
}

// Git marks never rely on colour alone.
type gitState int

const (
	gitClean gitState = iota
	gitDirty
	gitFailed
)

func (s gitState) mark() (string, color) {
	switch s {
	case gitDirty:
		return "+", cYellow
	case gitFailed:
		return "✗", cRed
	default:
		return "●", cCyan
	}
}

// statusLevel is the severity of a statusline message.
type statusLevel int

const (
	statusOK statusLevel = iota
	statusWarn
	statusErr
)

func (l statusLevel) mark() (string, color) {
	switch l {
	case statusWarn:
		return "!", cYellow
	case statusErr:
		return "✗", cRed
	default:
		return "✓", cCyan
	}
}

// cssColor is c as a CSS hex colour, or "" when the terminal never said.
func cssColor(c imgcolor.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
