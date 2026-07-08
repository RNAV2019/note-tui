package ui

import "charm.land/lipgloss/v2"

var (
	ColorAccent = lipgloss.Color("#cba6f7") // mauve
	ColorDim    = lipgloss.Color("#6c7086")
	ColorGood   = lipgloss.Color("#a6e3a1") // green
	ColorBad    = lipgloss.Color("#f38ba8") // red

	TitleStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	DimStyle   = lipgloss.NewStyle().Foreground(ColorDim)
	GoodStyle  = lipgloss.NewStyle().Foreground(ColorGood)
	BadStyle   = lipgloss.NewStyle().Foreground(ColorBad)
	MatchStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Underline(true)

	SelectedStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	CursorGlyph   = "❯ "
)

// Success and Fail render one-line status messages for command output.
func Success(msg string) string { return GoodStyle.Render("✓ ") + msg }
func Fail(msg string) string    { return BadStyle.Render("✗ ") + msg }
