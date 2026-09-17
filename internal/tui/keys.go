package tui

// binding is one action and the keys that trigger it. The help screen is
// generated from these, so it can never drift from what the keys really do.
type binding struct {
	keys  []string
	label string // how the key is written in help, e.g. "j k ↑ ↓"
	help  string
}

func (b binding) matches(key string) bool {
	for _, k := range b.keys {
		if k == key {
			return true
		}
	}
	return false
}

func bind(label, help string, keys ...string) binding {
	return binding{keys: keys, label: label, help: help}
}

var keys = struct {
	up, down, top, bottom      binding
	pane, prevTag, nextTag     binding
	jumpTag, enter             binding
	newItem, rename, del, move binding
	find, leader               binding
	sync, backup, help, quit   binding
	cancel                     binding

	// inside pickers, prompts and the finder, where letters are text
	pickUp, pickDown, accept, deleteWord binding
	finderMove, finderRename, finderDel  binding
}{
	up:       bind("j k ↑ ↓", "up / down", "k", "up"),
	down:     bind("", "", "j", "down"),
	top:      bind("g G", "first / last", "g", "home"),
	bottom:   bind("", "", "G", "end"),
	pane:     bind("h l tab", "switch pane", "tab", "shift+tab", "h", "l", "left", "right"),
	prevTag:  bind("[ ]", "prev / next tag", "["),
	nextTag:  bind("", "", "]"),
	jumpTag:  bind("1-9", "jump to tag", "1", "2", "3", "4", "5", "6", "7", "8", "9"),
	enter:    bind("enter", "open note · focus list", "enter"),
	newItem:  bind("n", "new (follows pane)", "n"),
	rename:   bind("r", "rename", "r"),
	del:      bind("d", "delete", "d"),
	move:     bind("m", "move note", "m"),
	find:     bind("/", "find any note", "/"),
	leader:   bind("space", "menu", "space"),
	sync:     bind("S", "sync (pull --rebase)", "S"),
	backup:   bind("B", "backup (commit + push)", "B"),
	help:     bind("?", "this screen", "?"),
	quit:     bind("q ctrl+c", "quit", "q", "ctrl+c"),
	cancel:   bind("esc", "close popup only", "esc"),
	pickUp:   bind("↑ ↓", "select", "up", "ctrl+p"),
	pickDown: bind("ctrl+n/p", "select", "down", "ctrl+n"),
	accept:   bind("enter", "accept", "enter"),
	// ctrl+w is handled by the text input itself; listed for help only.
	deleteWord:   bind("ctrl+w", "delete word"),
	finderMove:   bind("ctrl+v", "move highlighted", "ctrl+v"),
	finderRename: bind("ctrl+r", "rename highlighted", "ctrl+r"),
	finderDel:    bind("ctrl+d", "delete highlighted", "ctrl+d"),
}

// leaderItem is one row of the space menu.
type leaderItem struct {
	key, label, hint string
}

var leaderRoot = []leaderItem{
	{"f", "find note", "/"},
	{"n", "new note", "n"},
	{"N", "new notebook", ""},
	{"t", "tag", "›"},
	{"", "", ""},
	{"m", "move note", "m"},
	{"r", "rename", "r"},
	{"d", "delete", "d"},
	{"", "", ""},
	{"s", "sync", "git pull"},
	{"b", "backup", "commit + push"},
	{"?", "all keys", "?"},
}

func leaderTag(notebook, tag string) []leaderItem {
	target := tag
	if target == "" {
		target = "pick…"
	}
	return []leaderItem{
		{"n", "new tag", "in " + notebook},
		{"r", "rename tag", target},
		{"d", "delete tag", target},
		{"", "", ""},
		{"[", "previous tag", ""},
		{"]", "next tag", ""},
		{"1-9", "jump to tag", ""},
	}
}

// hint is one "key label" pair on the bottom line.
type hint struct{ key, label string }

type helpGroup struct {
	title string
	rows  []binding
}

func helpGroups() [][]helpGroup {
	sp := func(k, h string) binding { return bind("space "+k, h) }
	return [][]helpGroup{
		{
			{"Move", []binding{keys.up, keys.top, keys.pane, keys.prevTag, keys.jumpTag, keys.enter}},
			{"Edit", []binding{keys.newItem, keys.rename, keys.del, keys.move, keys.find}},
			{"Space", []binding{sp("f", "find note"), sp("n", "new note"), sp("N", "new notebook"),
				sp("t", "tag › n r d"), sp("s", "sync (pull)"), sp("b", "backup (push)")}},
		},
		{
			{"Pickers & prompts", []binding{keys.pickUp, keys.pickDown, keys.accept,
				bind("esc", "cancel / back"), keys.deleteWord}},
			{"Finder", []binding{keys.finderMove, keys.finderRename, keys.finderDel}},
			{"App", []binding{keys.sync, keys.backup, keys.help, keys.quit, keys.cancel}},
		},
	}
}
