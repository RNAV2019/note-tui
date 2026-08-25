package tui

import "charm.land/lipgloss/v2"

// Catppuccin Mocha. Section headers use sky so they read as structure rather
// than selection, leaving mauve to mean "this is where the cursor is".
var (
	colorAccent = lipgloss.Color("#cba6f7") // mauve
	colorHeader = lipgloss.Color("#89dceb") // sky
	colorText   = lipgloss.Color("#cdd6f4")
	colorDim    = lipgloss.Color("#6c7086")
	colorGood   = lipgloss.Color("#a6e3a1")
	colorBad    = lipgloss.Color("#f38ba8")

	frameStyle    = lipgloss.NewStyle().Foreground(colorDim)
	titleStyle    = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	headerStyle   = lipgloss.NewStyle().Foreground(colorHeader).Bold(true)
	textStyle     = lipgloss.NewStyle().Foreground(colorText)
	dimStyle      = lipgloss.NewStyle().Foreground(colorDim)
	goodStyle     = lipgloss.NewStyle().Foreground(colorGood)
	badStyle      = lipgloss.NewStyle().Foreground(colorBad)
	matchStyle    = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Underline(true)
	selectedStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
)

const cursorGlyph = "❯ "
