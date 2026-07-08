package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const maxVisible = 12

type pickerModel struct {
	title     string
	input     textinput.Model
	items     []string
	filtered  []Match
	cursor    int
	choice    string
	cancelled bool
}

func newPickerModel(title string, items []string) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Focus()
	return pickerModel{
		title:    title,
		input:    ti,
		items:    items,
		filtered: FilterItems("", items),
	}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if len(m.filtered) > 0 {
				m.choice = m.filtered[m.cursor].Value
			} else {
				m.cancelled = true
			}
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	// textinput.Model.Update returns (Model, tea.Cmd) in bubbles v2
	m.input, cmd = m.input.Update(msg)
	next := FilterItems(m.input.Value(), m.items)
	if len(next) != len(m.filtered) {
		m.cursor = 0
	}
	m.filtered = next
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
	return m, cmd
}

func highlight(match Match) string {
	idx := map[int]bool{}
	for _, i := range match.MatchedIndexes {
		idx[i] = true
	}
	var b strings.Builder
	for i, r := range match.Value {
		if idx[i] {
			b.WriteString(MatchStyle.Render(string(r)))
		} else {
			b.WriteString(string(r))
		}
	}
	return b.String()
}

func (m pickerModel) View() tea.View {
	var b strings.Builder
	b.WriteString(TitleStyle.Render(m.title))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	// Window the list around the cursor.
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.filtered))
	if len(m.filtered) == 0 {
		b.WriteString(DimStyle.Render("  no matches"))
		b.WriteString("\n")
	}
	for i := start; i < end; i++ {
		line := highlight(m.filtered[i])
		if i == m.cursor {
			b.WriteString(SelectedStyle.Render(CursorGlyph) + line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(fmt.Sprintf("%d/%d  ·  ↑/↓ move · enter select · esc cancel", len(m.filtered), len(m.items))))
	return tea.NewView(lipgloss.NewStyle().Padding(0, 1).Render(b.String()))
}

// RunPicker shows a fuzzy-filter picker and returns the chosen item.
// The second return is false if the user cancelled.
func RunPicker(title string, items []string) (string, bool, error) {
	p := tea.NewProgram(newPickerModel(title, items), tea.WithoutSignalHandler())
	result, err := p.Run()
	if err != nil {
		return "", false, fmt.Errorf("picker failed: %w", err)
	}
	m, ok := result.(pickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	if m.cancelled {
		return "", false, nil
	}
	return m.choice, true, nil
}
