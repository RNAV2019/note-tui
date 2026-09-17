package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note-tui/internal/config"
	"github.com/RNAV2019/note-tui/internal/notes"
	"github.com/RNAV2019/note-tui/internal/session"
)

type pane int

const (
	paneSidebar pane = iota
	paneNotes
)

// Model is the whole application: one persistent Bubble Tea program holding an
// in-memory snapshot of the note tree, re-read after every mutation.
type Model struct {
	store   *notes.Store
	cfg     config.Config
	version string

	all       []notes.Note // every note on disk
	notebooks []string
	tags      []string     // tags in the selected notebook
	visible   []notes.Note // notes in the selected notebook and tag, newest first

	focus pane
	nbCur int
	// tagCur is the active tab: 0 is the synthetic "all" tab, i is tags[i-1].
	tagCur  int
	noteCur int

	l  layout
	ov overlay

	status      string
	statusLevel statusLevel

	repo      notes.RepoStatus
	gitFailed bool // the last sync or backup failed

	prev preview

	// pendingTag carries the tag chosen in the first step of the two-step
	// "new note" flow through to the title prompt.
	pendingTag string
}

// New builds the model and loads the note tree.
func New(store *notes.Store, cfg config.Config, version string) (Model, error) {
	m := Model{store: store, cfg: cfg, version: version, focus: paneNotes}
	if err := m.reload(); err != nil {
		return m, err
	}
	m.l = computeLayout(120, 34) // replaced by the first WindowSizeMsg
	if len(m.notebooks) == 0 {
		m.focus = paneSidebar
	}
	return m, nil
}

func (m Model) Init() tea.Cmd { return nil }

// ---------------------------------------------------------------- data

// reload re-reads the note tree and repo state from disk and re-clamps every
// cursor.
func (m *Model) reload() error {
	all, err := m.store.AllNotes()
	if err != nil {
		return err
	}
	notebooks, err := m.store.Notebooks()
	if err != nil {
		return err
	}
	m.all, m.notebooks = all, notebooks
	m.nbCur = clamp(m.nbCur, len(m.notebooks))
	m.refreshRepo()
	return m.refreshTags()
}

// refreshRepo re-reads git state. A repo we can't read just shows nothing.
func (m *Model) refreshRepo() {
	if st, err := m.store.Status(); err == nil {
		m.repo = st
	}
}

func (m *Model) refreshTags() error {
	nb := m.currentNotebook()
	m.tags = nil
	if nb != "" {
		tags, err := m.store.Tags(nb)
		if err != nil {
			return err
		}
		m.tags = tags
	}
	m.tagCur = clamp(m.tagCur, len(m.tags)+1)
	m.refreshVisible()
	return nil
}

func (m *Model) reloadOrStatus() {
	if err := m.reload(); err != nil {
		m.setError(err)
	}
}

func (m *Model) refreshTagsOrStatus() {
	if err := m.refreshTags(); err != nil {
		m.setError(err)
	}
}

func (m *Model) refreshVisible() {
	nb, tag := m.currentNotebook(), m.selectedTag()
	// A fresh slice, not m.visible[:0]: Bubble Tea copies the model by value,
	// so reusing the backing array would let one copy stomp another's view.
	m.visible = nil
	for _, n := range m.all {
		if n.Notebook == nb && (tag == "" || n.Tag == tag) {
			m.visible = append(m.visible, n)
		}
	}
	sortRecent(m.visible)
	m.noteCur = clamp(m.noteCur, len(m.visible))
	m.refreshPreview()
}

// sortRecent orders notes newest first, falling back to the path so the order
// is stable when timestamps tie.
func sortRecent(ns []notes.Note) {
	sort.SliceStable(ns, func(i, j int) bool {
		if !ns[i].ModTime.Equal(ns[j].ModTime) {
			return ns[i].ModTime.After(ns[j].ModTime)
		}
		return ns[i].Display() < ns[j].Display()
	})
}

// refreshPreview reloads the preview only when the selected file changed.
func (m *Model) refreshPreview() {
	n, ok := m.currentNote()
	if !ok {
		m.prev = preview{}
		return
	}
	if m.prev.path == n.Path && m.prev.modTime.Equal(n.ModTime) {
		return
	}
	m.prev = loadPreview(m.store.Root, n.Path, n.ModTime)
}

func clamp(cursor, count int) int {
	if cursor >= count {
		cursor = count - 1
	}
	return max(cursor, 0)
}

func (m Model) currentNotebook() string {
	if m.nbCur < 0 || m.nbCur >= len(m.notebooks) {
		return ""
	}
	return m.notebooks[m.nbCur]
}

// selectedTag returns the active tag, or "" for the synthetic "all" tab.
func (m Model) selectedTag() string {
	if m.tagCur <= 0 || m.tagCur > len(m.tags) {
		return ""
	}
	return m.tags[m.tagCur-1]
}

func (m Model) currentNote() (notes.Note, bool) {
	if m.noteCur < 0 || m.noteCur >= len(m.visible) {
		return notes.Note{}, false
	}
	return m.visible[m.noteCur], true
}

func (m Model) countNotes(nb, tag string) int {
	n := 0
	for _, note := range m.all {
		if note.Notebook == nb && (tag == "" || note.Tag == tag) {
			n++
		}
	}
	return n
}

func (m Model) notesIn(nb, tag string) []notes.Note {
	var out []notes.Note
	for _, n := range m.all {
		if n.Notebook == nb && (tag == "" || n.Tag == tag) {
			out = append(out, n)
		}
	}
	sortRecent(out)
	return out
}

func (m Model) gitStateOf(nb string) gitState {
	switch {
	case m.repo.Dirty[nb] && m.gitFailed:
		return gitFailed
	case m.repo.Dirty[nb]:
		return gitDirty
	}
	return gitClean
}

func (m *Model) setStatus(level statusLevel, text string) {
	m.status, m.statusLevel = text, level
}

func (m *Model) setError(err error) {
	// git errors can span lines; the statusline has room for one.
	text, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
	m.setStatus(statusErr, text)
}

// ---------------------------------------------------------------- update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.l = computeLayout(msg.Width, msg.Height)
		if !m.l.hasSidebar() {
			m.focus = paneNotes
		}
		return m, nil

	case editorDoneMsg:
		return m.afterEditor(msg)

	case gitDoneMsg:
		m.gitFailed = msg.err != nil
		if msg.err != nil {
			first, _, _ := strings.Cut(strings.TrimSpace(msg.err.Error()), "\n")
			m.setStatus(statusErr, msg.op+" failed · "+first)
		} else {
			m.setStatus(statusOK, msg.text)
		}
		m.reloadOrStatus()
		return m, nil

	case tea.KeyPressMsg:
		if m.ov.active() {
			return m.overlayKey(msg)
		}
		return m.handleKey(msg.String())
	}

	// Non-key messages still need to reach an open text input (paste, blink).
	if m.ov.kind == ovPrompt || m.ov.kind == ovPicker || m.ov.kind == ovFinder {
		cmd := m.ov.updateInput(msg)
		if m.ov.kind == ovFinder {
			m.syncFinderPreview()
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	// A status message lasts until the next key.
	m.status = ""

	switch {
	case keys.quit.matches(key):
		return m, tea.Quit
	case keys.cancel.matches(key):
		return m, nil // esc never quits; it only closes things
	case keys.help.matches(key):
		m.ov = overlay{kind: ovHelp}
	case keys.leader.matches(key):
		m.ov = overlay{kind: ovLeader}
	case keys.pane.matches(key):
		if m.l.hasSidebar() && len(m.notebooks) > 0 {
			m.focus = 1 - m.focus
		}
	case keys.up.matches(key):
		m.setCursor(m.cursor() - 1)
	case keys.down.matches(key):
		m.setCursor(m.cursor() + 1)
	case keys.top.matches(key):
		m.setCursor(0)
	case keys.bottom.matches(key):
		m.setCursor(m.paneLen() - 1)
	case keys.prevTag.matches(key):
		m.cycleTag(-1)
	case keys.nextTag.matches(key):
		m.cycleTag(1)
	case keys.jumpTag.matches(key):
		m.jumpTag(int(key[0] - '1'))
	case keys.enter.matches(key):
		if m.focus == paneSidebar {
			if len(m.notebooks) > 0 {
				m.focus = paneNotes
			}
			return m, nil
		}
		return m.openSelected()
	case keys.find.matches(key):
		m.openFinder()
	case keys.newItem.matches(key):
		if m.focus == paneSidebar || len(m.notebooks) == 0 {
			m.startNewNotebook()
		} else {
			m.startNewNote()
		}
	case keys.rename.matches(key):
		m.startRename()
	case keys.del.matches(key):
		m.startDelete()
	case keys.move.matches(key):
		if m.focus == paneNotes {
			if n, ok := m.currentNote(); ok {
				m.startMove(n)
			}
		}
	case keys.sync.matches(key):
		return m, m.syncCmd()
	case keys.backup.matches(key):
		return m, m.backupCmd()
	}
	return m, nil
}

func (m Model) paneLen() int {
	if m.focus == paneSidebar {
		return len(m.notebooks)
	}
	return len(m.visible)
}

func (m Model) cursor() int {
	if m.focus == paneSidebar {
		return m.nbCur
	}
	return m.noteCur
}

func (m *Model) setCursor(to int) {
	to = clamp(to, m.paneLen())
	if m.focus == paneNotes {
		m.noteCur = to
		m.refreshPreview()
		return
	}
	if to == m.nbCur {
		return
	}
	// A different notebook means different tabs; start on "all" so the note
	// list is never empty by surprise.
	m.nbCur, m.tagCur, m.noteCur = to, 0, 0
	m.refreshTagsOrStatus()
}

func (m *Model) cycleTag(delta int) {
	n := len(m.tags) + 1
	m.setTag(((m.tagCur+delta)%n + n) % n)
}

func (m *Model) jumpTag(i int) {
	if i <= len(m.tags) {
		m.setTag(i)
	}
}

func (m *Model) setTag(i int) {
	if i == m.tagCur {
		return
	}
	m.tagCur, m.noteCur = i, 0
	m.refreshVisible()
}

// ---------------------------------------------------------------- overlays

func (m Model) overlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.ov.kind {
	case ovHelp:
		m.ov = overlay{} // any key closes help
		return m, nil

	case ovLeader:
		return m.leaderKey(key)

	case ovConfirm:
		switch key {
		case "y", "Y":
			o := m.ov
			m.ov = overlay{}
			m.applyDelete(o)
		case "n", "N", "q", "esc":
			m.ov = overlay{}
		}
		// enter deliberately does nothing: only y deletes.
		return m, nil

	case ovPrompt:
		switch key {
		case "esc":
			if m.ov.back {
				m.startNewNote() // step back to the tag picker
				return m, nil
			}
			m.ov = overlay{}
			return m, nil
		case "enter":
			o := m.ov
			m.ov = overlay{}
			if o.query() == "" {
				return m, nil
			}
			return m.applyPrompt(o)
		}
		return m, m.ov.updateInput(msg)

	case ovPicker:
		if m.ov.moveSelection(key) {
			return m, nil
		}
		switch key {
		case "esc":
			m.ov = overlay{}
			return m, nil
		case "enter":
			o := m.ov
			m.ov = overlay{}
			if i := o.selected(); i >= 0 {
				m.applyPick(o, o.items[i].value)
			}
			return m, nil
		}
		return m, m.ov.updateInput(msg)

	case ovFinder:
		if m.ov.moveSelection(key) {
			m.syncFinderPreview()
			return m, nil
		}
		switch {
		case key == "esc":
			m.ov = overlay{}
			m.refreshPreview() // back to the main selection
			return m, nil
		case key == "enter":
			return m.finderAct(func(m *Model, n notes.Note) tea.Cmd {
				_, cmd := m.openNote(n)
				return cmd
			})
		case keys.finderMove.matches(key):
			return m.finderAct(func(m *Model, n notes.Note) tea.Cmd { m.startMove(n); return nil })
		case keys.finderRename.matches(key):
			return m.finderAct(func(m *Model, n notes.Note) tea.Cmd { m.startRenameNote(n); return nil })
		case keys.finderDel.matches(key):
			return m.finderAct(func(m *Model, n notes.Note) tea.Cmd { m.startDeleteNote(n); return nil })
		}
		cmd := m.ov.updateInput(msg)
		m.syncFinderPreview()
		return m, cmd
	}
	return m, nil
}

func (m Model) leaderKey(key string) (tea.Model, tea.Cmd) {
	sub := m.ov.sub
	m.ov = overlay{}
	if sub == "tag" {
		switch key {
		case "n":
			m.startNewTag()
		case "r":
			m.startRenameTag()
		case "d":
			m.startDeleteTag()
		case "[":
			m.cycleTag(-1)
		case "]":
			m.cycleTag(1)
		case "backspace":
			m.ov = overlay{kind: ovLeader}
		default:
			if keys.jumpTag.matches(key) {
				m.jumpTag(int(key[0] - '1'))
			}
		}
		return m, nil
	}

	switch key {
	case "f":
		m.openFinder()
	case "n":
		if len(m.notebooks) > 0 {
			m.focus = paneNotes
		}
		m.startNewNote()
	case "N":
		m.startNewNotebook()
	case "t":
		if m.currentNotebook() == "" {
			m.setStatus(statusErr, "create a notebook first — press space N")
			return m, nil
		}
		m.ov = overlay{kind: ovLeader, sub: "tag"}
	case "m":
		if n, ok := m.currentNote(); ok {
			m.startMove(n)
		}
	case "r":
		m.startRename()
	case "d":
		m.startDelete()
	case "s":
		return m, m.syncCmd()
	case "b":
		return m, m.backupCmd()
	case "?":
		m.ov = overlay{kind: ovHelp}
	}
	return m, nil
}

// finderAct closes the finder, points the main view at the highlighted note
// and runs fn on it.
func (m Model) finderAct(fn func(*Model, notes.Note) tea.Cmd) (tea.Model, tea.Cmd) {
	i := m.ov.selected()
	if i < 0 {
		m.ov = overlay{}
		return m, nil
	}
	n := m.ov.candidates[i]
	m.ov = overlay{}
	m.revealNote(n)
	cmd := fn(&m, n)
	return m, cmd
}

// revealNote selects n in the main view.
func (m *Model) revealNote(n notes.Note) {
	m.selectNotebook(n.Notebook)
	m.setTag(0)
	m.selectNote(n.Name, n.Tag)
	m.focus = paneNotes
}

func (m *Model) openFinder() {
	cands := append([]notes.Note(nil), m.all...)
	sortRecent(cands)
	labels := make([]string, len(cands))
	for i, n := range cands {
		labels[i] = n.Display()
	}
	m.ov = overlay{kind: ovFinder, input: newTextInput(""), labels: labels, candidates: cands}
	m.ov.refilter()
	m.syncFinderPreview()
}

// syncFinderPreview points the preview cache at the finder's selection.
func (m *Model) syncFinderPreview() {
	i := m.ov.selected()
	if i < 0 {
		return
	}
	n := m.ov.candidates[i]
	if m.prev.path != n.Path || !m.prev.modTime.Equal(n.ModTime) {
		m.prev = loadPreview(m.store.Root, n.Path, n.ModTime)
	}
}

// ---------------------------------------------------------------- selection

func (m *Model) selectNotebook(name string) {
	for i, nb := range m.notebooks {
		if nb == name {
			if i != m.nbCur {
				m.nbCur, m.tagCur, m.noteCur = i, 0, 0
			}
			m.refreshTagsOrStatus()
			return
		}
	}
}

func (m *Model) selectTag(name string) {
	for i, t := range m.tags {
		if t == name {
			m.setTag(i + 1)
			return
		}
	}
}

func (m *Model) selectNote(name, tag string) {
	for i, n := range m.visible {
		if n.Name == name && (tag == "" || n.Tag == tag) {
			m.noteCur = i
			m.refreshPreview()
			return
		}
	}
}

// ---------------------------------------------------------------- git

type gitDoneMsg struct {
	op   string // "sync" or "backup", for error messages
	text string
	err  error
}

func (m Model) upstream() string {
	if m.repo.Remote == "" {
		return ""
	}
	return m.repo.Remote + "/" + m.repo.Branch
}

func (m Model) syncCmd() tea.Cmd {
	store, up := m.store, m.upstream()
	return func() tea.Msg {
		if err := store.Sync(); err != nil {
			return gitDoneMsg{op: "sync", err: err}
		}
		return gitDoneMsg{op: "sync", text: "synced with " + up}
	}
}

func (m Model) backupCmd() tea.Cmd {
	store, up := m.store, m.upstream()
	return func() tea.Msg {
		pushed, err := store.Backup("backup: " + nowFunc().Format("2006-01-02 15:04"))
		if err != nil {
			return gitDoneMsg{op: "backup", err: err}
		}
		if pushed {
			return gitDoneMsg{op: "backup", text: "backed up · pushed to " + up}
		}
		return gitDoneMsg{op: "backup", text: "backed up locally (no remote configured)"}
	}
}

// ---------------------------------------------------------------- session

type editorDoneMsg struct {
	err      error
	warnings []string
}

// editSession adapts session.Open to Bubble Tea's ExecCommand interface, so
// the runtime releases the terminal for the editor and restores it afterwards.
type editSession struct {
	file           string
	opts           session.Options
	stdin          io.Reader
	stdout, stderr io.Writer
	warnings       []string
}

func (e *editSession) SetStdin(r io.Reader)  { e.stdin = r }
func (e *editSession) SetStdout(w io.Writer) { e.stdout = w }
func (e *editSession) SetStderr(w io.Writer) { e.stderr = w }

func (e *editSession) Run() error {
	tty := session.IO{In: e.stdin, Out: e.stdout, Err: e.stderr}
	return session.Open(e.file, e.opts, tty, func(msg string) {
		// Collected rather than printed: the editor is about to take the screen.
		e.warnings = append(e.warnings, msg)
	})
}

func (m Model) newEditSession(n notes.Note) *editSession {
	return &editSession{
		file: n.Path,
		opts: session.Options{
			Editor:         m.cfg.Editor,
			Preview:        m.cfg.Preview,
			PreviewURL:     m.cfg.PreviewURL,
			PreviewProfile: m.cfg.PreviewProfile,
		},
	}
}

func (m Model) openNote(n notes.Note) (tea.Model, tea.Cmd) {
	sess := m.newEditSession(n)
	return m, tea.Exec(sess, func(err error) tea.Msg {
		return editorDoneMsg{err: err, warnings: sess.warnings}
	})
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	n, ok := m.currentNote()
	if !ok {
		return m, nil
	}
	return m.openNote(n)
}

func (m Model) afterEditor(msg editorDoneMsg) (tea.Model, tea.Cmd) {
	// The note's mtime changed, and the editor may have created files.
	if err := m.reload(); err != nil {
		m.setError(err)
		return m, nil
	}
	switch {
	case msg.err != nil:
		m.setError(msg.err)
	case len(msg.warnings) > 0:
		m.setStatus(statusWarn, strings.Join(msg.warnings, "; "))
	default:
		m.status = ""
	}
	return m, nil
}

// ---------------------------------------------------------------- program

// Run starts the TUI.
func Run(store *notes.Store, cfg config.Config, version string) error {
	m, err := New(store, cfg, version)
	if err != nil {
		return fmt.Errorf("reading notes: %w", err)
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
