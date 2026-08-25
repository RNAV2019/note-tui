package tui

import "strings"

// binding is one action and the keys that trigger it. The footer hints and the
// help overlay are both generated from these, so they can never drift apart.
type binding struct {
	keys  []string
	label string // short form for the footer
	help  string // long form for the help overlay
}

func (b binding) matches(key string) bool {
	for _, k := range b.keys {
		if k == key {
			return true
		}
	}
	return false
}

// hint renders "j/k move" for the footer.
func (b binding) hint() string {
	return dimStyle.Render(strings.Join(b.keys, "/")) + " " + dimStyle.Render(b.label)
}

type keymap struct {
	up, down       binding
	top, bottom    binding
	nextPane       binding
	prevPane       binding
	enter          binding
	newItem        binding
	rename, delete binding
	move           binding
	search         binding
	sync, backup   binding
	help, quit     binding
}

var keys = keymap{
	up:       binding{[]string{"k", "up", "ctrl+p"}, "up", "move up"},
	down:     binding{[]string{"j", "down", "ctrl+n"}, "down", "move down"},
	top:      binding{[]string{"g", "home"}, "top", "jump to first item"},
	bottom:   binding{[]string{"G", "end"}, "bottom", "jump to last item"},
	nextPane: binding{[]string{"tab", "l", "right"}, "pane", "focus the next pane"},
	prevPane: binding{[]string{"shift+tab", "h", "left"}, "pane", "focus the previous pane"},
	enter:    binding{[]string{"enter"}, "open", "open the note, or focus the note list"},
	newItem:  binding{[]string{"n"}, "new", "new note, notebook or tag (follows the focused pane)"},
	rename:   binding{[]string{"r"}, "rename", "rename the selected item"},
	delete:   binding{[]string{"d"}, "delete", "delete the selected item"},
	move:     binding{[]string{"m"}, "move", "move a note to another notebook or tag"},
	search:   binding{[]string{"/"}, "search", "fuzzy search every note"},
	sync:     binding{[]string{"S"}, "sync", "git pull --rebase the notes repo"},
	backup:   binding{[]string{"B"}, "backup", "commit and push the notes repo"},
	help:     binding{[]string{"?"}, "keys", "toggle this help"},
	quit:     binding{[]string{"q", "ctrl+c"}, "quit", "quit"},
}

// footer renders the one-line hint bar under the frame for the focused pane.
func footer(p pane) string {
	hints := []binding{keys.newItem, keys.rename, keys.delete}
	if p == paneNotes {
		hints = append(hints, keys.move)
	}
	hints = append(hints, keys.search, keys.nextPane, keys.help, keys.quit)

	parts := make([]string, len(hints))
	for i, h := range hints {
		// Only the first key of each binding earns footer space.
		parts[i] = dimStyle.Render(h.keys[0] + " " + h.label)
	}
	return "  " + strings.Join(parts, dimStyle.Render(" · "))
}

// helpLines renders the full keymap for the help overlay.
func helpLines() []string {
	all := []binding{
		keys.up, keys.down, keys.top, keys.bottom,
		keys.nextPane, keys.prevPane, keys.enter,
		keys.newItem, keys.rename, keys.delete, keys.move,
		keys.search, keys.sync, keys.backup, keys.help, keys.quit,
	}
	out := make([]string, 0, len(all))
	for _, b := range all {
		out = append(out, "  "+pad(selectedStyle.Render(strings.Join(b.keys, ", ")), 22)+dimStyle.Render(b.help))
	}
	return out
}
