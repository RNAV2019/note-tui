package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		name                            string
		width, height                   int
		wantSidebar, wantMain, wantBody int
	}{
		{"roomy terminal", 100, 30, sidebarWidth, 100 - 2 - sidebarWidth - 1, 27},
		{"narrow terminal drops the sidebar", 60, 30, 0, 58, 27},
		{"tiny terminal still yields one row", 10, 2, 0, 8, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := computeLayout(tt.width, tt.height)
			if l.Sidebar != tt.wantSidebar {
				t.Errorf("Sidebar = %d, want %d", l.Sidebar, tt.wantSidebar)
			}
			if l.Main != tt.wantMain {
				t.Errorf("Main = %d, want %d", l.Main, tt.wantMain)
			}
			if l.Body != tt.wantBody {
				t.Errorf("Body = %d, want %d", l.Body, tt.wantBody)
			}
		})
	}
}

// The frame is the whole aesthetic, so every line must be exactly as wide as
// the terminal or the border will ladder.
func TestFrameLinesAreExactlyWidth(t *testing.T) {
	body := []string{"short", strings.Repeat("x", 200), ""}
	const width, height = 40, 5
	out := frame("note 0.2.0", body, width, height)

	lines := strings.Split(out, "\n")
	if len(lines) != height+2 {
		t.Fatalf("got %d lines, want %d", len(lines), height+2)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("line %d width = %d, want %d: %q", i, w, width, line)
		}
	}
}

func TestPadCenterTruncateAreANSISafe(t *testing.T) {
	styled := selectedStyle.Render("hello")

	if got := lipgloss.Width(pad(styled, 20)); got != 20 {
		t.Errorf("pad width = %d, want 20", got)
	}
	if got := lipgloss.Width(truncate(styled, 3)); got != 3 {
		t.Errorf("truncate width = %d, want 3", got)
	}
	// Padding must not drop the styling it was given.
	if !strings.Contains(pad(styled, 20), "hello") {
		t.Error("pad lost its content")
	}
}

func TestRowFlushesRightHandSide(t *testing.T) {
	got := row("left", "9", 10)
	if got != "left     9" {
		t.Errorf("row = %q, want %q", got, "left     9")
	}
	// A left side too long to fit must be truncated, not allowed to overflow.
	if w := lipgloss.Width(row(strings.Repeat("x", 50), "9", 10)); w != 10 {
		t.Errorf("overlong row width = %d, want 10", w)
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		name                    string
		cursor, count, capacity int
		wantStart, wantEnd      int
	}{
		{"everything fits", 0, 3, 10, 0, 3},
		{"scrolls to centre the cursor", 5, 20, 5, 3, 8},
		{"clamps at the top", 0, 20, 5, 0, 5},
		{"clamps at the bottom", 19, 20, 5, 15, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := window(tt.cursor, tt.count, tt.capacity)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("window = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	got := wrap("delete the notebook and all of its notes", 20)
	for _, line := range got {
		if lipgloss.Width(line) > 20 {
			t.Errorf("line %q exceeds 20 cells", line)
		}
	}
	if strings.Join(got, " ") != "delete the notebook and all of its notes" {
		t.Errorf("wrap changed the text: %q", got)
	}
}
