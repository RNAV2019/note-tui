package tui

import "testing"

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		name                    string
		width, height           int
		sidebar, notes, preview int // widths
	}{
		{"wide terminal shows three panes", 120, 34, sidebarWidth, notesWidth, 120 - sidebarWidth - notesWidth},
		{"medium terminal drops the preview", 90, 28, sidebarWidth, 90 - sidebarWidth, 0},
		{"narrow terminal shows notes only", 60, 24, 0, 60, 0},
		{"tiny terminal", 10, 2, 0, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := computeLayout(tt.width, tt.height)
			if l.sidebar.w != tt.sidebar || l.notes.w != tt.notes || l.preview.w != tt.preview {
				t.Errorf("widths = %d/%d/%d, want %d/%d/%d",
					l.sidebar.w, l.notes.w, l.preview.w, tt.sidebar, tt.notes, tt.preview)
			}
			if total := l.sidebar.w + l.notes.w + l.preview.w; total != tt.width {
				t.Errorf("panes cover %d columns, want all %d", total, tt.width)
			}
			if l.statusRow != tt.height-2 || l.hintRow != tt.height-1 {
				t.Errorf("status/hint rows = %d/%d", l.statusRow, l.hintRow)
			}
			if l.notes.h != max(tt.height-3, 0) {
				t.Errorf("pane height = %d, want %d", l.notes.h, tt.height-3)
			}
		})
	}
}

func TestWindowKeepsCursorVisible(t *testing.T) {
	for cursor := 0; cursor < 20; cursor++ {
		start, end := window(cursor, 20, 5)
		if cursor < start || cursor >= end || end-start != 5 {
			t.Fatalf("window(%d) = [%d,%d)", cursor, start, end)
		}
	}
	if s, e := window(3, 4, 10); s != 0 || e != 4 {
		t.Errorf("short list window = [%d,%d), want everything", s, e)
	}
}
