package tui

import (
	"strings"
	"testing"

	"time"

	"github.com/charmbracelet/x/ansi"
)

// rows renders the model and returns its untangled text, one string per row.
func rows(t *testing.T, m Model) []string {
	t.Helper()
	return strings.Split(m.render().plain(), "\n")
}

func screen(t *testing.T, m Model) string {
	t.Helper()
	return m.render().plain()
}

// contains reports whether any row holds s, so tests can ignore padding.
func contains(lines []string, s string) bool {
	for _, l := range lines {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

func mustContain(t *testing.T, m Model, wants ...string) {
	t.Helper()
	lines := rows(t, m)
	for _, w := range wants {
		if !contains(lines, w) {
			t.Errorf("screen is missing %q:\n%s", w, strings.Join(lines, "\n"))
		}
	}
}

func TestWideLayoutDrawsEveryPane(t *testing.T) {
	m := sized(t, newTestModel(t), 120, 34)
	m = press(t, m, "tab", "j", "tab") // uni, notes pane
	mustContain(t, m,
		" note ",    // the wordmark chip
		" uni ",     // the notebook chip
		"Notebooks", // sidebar title
		"Notes",     // note list title
		"1 all 3",   // the all tab with its count
		"2 algos 2", // numbered tag tabs
		"3 networks 1",
		"repo",       // the repo panel
		"branch",     //   with its rows
		"tcp",        // newest note first
		"6d ago",     //   with a relative age
		"tcp.typ",    // the preview pane title
		"NOR",        // the mode pill
		"uni › all",  // the breadcrumb
		"space menu", // a hint
	)
}

func TestMediumLayoutDropsThePreview(t *testing.T) {
	m := sized(t, newTestModel(t), 90, 30)
	mustContain(t, m, "Notebooks", "Notes")
	if contains(rows(t, m), "modified") {
		t.Error("the preview pane should be gone at 90 columns")
	}
}

func TestNarrowLayoutDropsTheSidebar(t *testing.T) {
	m := sized(t, newTestModel(t), 60, 24)
	lines := rows(t, m)
	if contains(lines, "Notebooks") {
		t.Error("the sidebar should be gone at 60 columns")
	}
	if !contains(lines, "Notes") {
		t.Errorf("the note list should survive:\n%s", strings.Join(lines, "\n"))
	}
}

func TestEmptyTreeShowsTheWelcomeSteps(t *testing.T) {
	store := newEmptyStore(t)
	m := newModelOver(t, store)
	m = sized(t, m, 120, 34)
	mustContain(t, m, wordmark[0], "to create your first notebook", "template", "new notebook")
}

func TestTagOverflowIsMarked(t *testing.T) {
	m := newTestModel(t)
	for _, tag := range []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"} {
		if err := m.store.CreateTag("scratch", tag); err != nil {
			t.Fatal(err)
		}
	}
	m.reloadOrStatus()
	m = sized(t, m, 80, 24)
	if !contains(rows(t, m), "›") {
		t.Errorf("expected an overflow marker in the tab bar:\n%s", rows(t, m)[0])
	}
}

func TestStatuslineShowsMessagesAndMode(t *testing.T) {
	m := sized(t, newTestModel(t), 120, 34)
	m = press(t, m, "tab", "n") // new notebook prompt
	m.ov.input.SetValue("Year 1")
	mustContain(t, m, "INPUT", "New notebook", "creates") // the slug preview is path-dependent
	m = press(t, m, "enter")
	mustContain(t, m, "✓ created notebook year-1")
}

func TestOverlaysRender(t *testing.T) {
	base := sized(t, newTestModel(t), 120, 34)
	cases := []struct {
		name  string
		keys  []string
		wants []string
	}{
		{"leader", []string{"space"}, []string{"space", "find note", "new notebook", "all keys", "SPC"}},
		{"leader tag", []string{"space", "t"}, []string{"space t", "new tag", "jump to tag"}},
		{"prompt", []string{"n"}, []string{"New note", "file", "INPUT"}},
		{"picker", []string{"m"}, []string{"Move idea.typ", "from scratch/misc", "uni/algos"}},
		{"finder", []string{"/"}, []string{"Find", "search every note", "scratch/misc/idea", "FIND"}},
		{"confirm", []string{"d"}, []string{"Delete note", "Delete idea.typ?", "restored from git", " y ", "CONFIRM"}},
		{"help", []string{"?"}, []string{"Keys", "Move", "Finder", "quit", "clean", "uncommitted", "KEYS"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustContain(t, press(t, base, tc.keys...), tc.wants...)
		})
	}
}

func TestDimmedOverlaysKeepTheirOwnColours(t *testing.T) {
	m := withTerminalColours(t, sized(t, newTestModel(t), 120, 34), rosePineBase, rosePineText)
	m = press(t, m, "?")
	out := m.render().String()
	// The help box paints on surface; the backdrop is dimmed to hlLow.
	if !strings.Contains(out, m.pal[cSurface].code(true)) {
		t.Error("the popup lost its surface background")
	}
	if !strings.Contains(out, m.pal[cHLLow].code(true)) {
		t.Error("the backdrop was not dimmed")
	}
}

// Every row must be exactly as wide as the terminal, or the terminal will
// reflow the frame and the whole layout smears.
func TestEveryRowIsExactlyTerminalWidth(t *testing.T) {
	base := newTestModel(t)
	sizes := [][2]int{{160, 48}, {120, 34}, {110, 30}, {100, 28}, {80, 24}, {70, 20}, {69, 18}, {40, 12}, {24, 8}, {10, 4}, {1, 1}}
	overlays := [][]string{nil, {"space"}, {"space", "t"}, {"n"}, {"m"}, {"/"}, {"d"}, {"?"}}
	for _, size := range sizes {
		m := sized(t, base, size[0], size[1])
		for _, ks := range overlays {
			got := press(t, m, ks...)
			frame := got.render().String()
			for i, line := range strings.Split(frame, "\n") {
				if w := ansi.StringWidth(line); w != size[0] {
					t.Fatalf("%dx%d overlay %v: row %d is %d cells wide", size[0], size[1], ks, i, w)
				}
			}
			if n := strings.Count(frame, "\n") + 1; n != size[1] {
				t.Fatalf("%dx%d overlay %v: %d rows", size[0], size[1], ks, n)
			}
		}
	}
}

func TestTinyTerminalStillRenders(t *testing.T) {
	m := sized(t, newTestModel(t), 12, 5)
	if screen(t, m) == "" {
		t.Error("empty frame")
	}
}

func TestRelTime(t *testing.T) {
	now := nowFunc()
	cases := []struct {
		mins int
		want string
	}{
		{0, "just now"},
		{5, "5m ago"},
		{90, "1h ago"},
		{60 * 30, "1d ago"},
		{60 * 24 * 9, "1w ago"},
		{60 * 24 * 40, "1mo ago"},
		{60 * 24 * 400, "1y ago"},
	}
	for _, tc := range cases {
		if got := relTime(now.Add(-time.Duration(tc.mins) * time.Minute)); got != tc.want {
			t.Errorf("relTime(-%dm) = %q, want %q", tc.mins, got, tc.want)
		}
	}
}
