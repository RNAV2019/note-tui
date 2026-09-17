package tui

// Rosé Pine. Every text colour here clears 4.5:1 on the backgrounds it is
// used on; muted (3.4:1) is kept for borders and pine (3.4:1) isn't used at all.
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
	cLove                 // danger, errors, CONFIRM
	cGold                 // keys, INPUT, uncommitted
	cRose                 // leader, fuzzy matches, SPC
	cFoam                 // tags, clean, FIND
	cIris                 // focus, NOR
)

type rgb struct{ r, g, b uint8 }

var palette = [...]rgb{
	cNone:    {0x19, 0x17, 0x24},
	cBase:    {0x19, 0x17, 0x24},
	cSurface: {0x1f, 0x1d, 0x2e},
	cOverlay: {0x26, 0x23, 0x3a},
	cHLLow:   {0x21, 0x20, 0x2e},
	cHLMed:   {0x40, 0x3d, 0x52},
	cHLHigh:  {0x52, 0x4f, 0x67},
	cMuted:   {0x6e, 0x6a, 0x86},
	cSubtle:  {0x90, 0x8c, 0xaa},
	cText:    {0xe0, 0xde, 0xf4},
	cLove:    {0xeb, 0x6f, 0x92},
	cGold:    {0xf6, 0xc1, 0x77},
	cRose:    {0xeb, 0xbc, 0xba},
	cFoam:    {0x9c, 0xcf, 0xd8},
	cIris:    {0xc4, 0xa7, 0xe7},
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
	return [...]color{cIris, cRose, cFoam, cGold, cLove, cSubtle}[m]
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
		return "+", cGold
	case gitFailed:
		return "✗", cLove
	default:
		return "●", cFoam
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
		return "!", cGold
	case statusErr:
		return "✗", cLove
	default:
		return "✓", cFoam
	}
}
