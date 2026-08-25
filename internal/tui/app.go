package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/RNAV2019/note/internal/config"
	"github.com/RNAV2019/note/internal/notes"
	"github.com/RNAV2019/note/internal/session"
)

type pane int

const (
	paneNotebooks pane = iota
	paneTags
	paneNotes
)

// Model is the whole application: one persistent Bubble Tea program holding an
// in-memory snapshot of the note tree, re-read after every mutation.
type Model struct {
	store *notes.Store
	cfg   config.Config
	title string

	all       []notes.Note // every note on disk
	notebooks []string
	tags      []string     // tags in the selected notebook
	visible   []notes.Note // notes matching the selected notebook and tag

	focus                  pane
	nbCur, tagCur, noteCur int

	l  layout
	ov overlay

	status    string
	statusErr bool

	// pendingTag carries the tag chosen in the first step of the two-step
	// "new note" flow through to the title prompt.
	pendingTag string
}

// New builds the model and loads the note tree.
func New(store *notes.Store, cfg config.Config, version string) (Model, error) {
	m := Model{store: store, cfg: cfg, title: "note " + version}
	if err := m.reload(); err != nil {
		return m, err
	}
	m.l = computeLayout(80, 24) // replaced by the first WindowSizeMsg
	return m, nil
}

func (m Model) Init() tea.Cmd { return nil }

// ---------------------------------------------------------------- data

// reload re-reads the note tree from disk and re-clamps every cursor.
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
	return m.refreshTags()
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
	// +1 for the synthetic "all" row.
	m.tagCur = clamp(m.tagCur, len(m.tags)+1)
	m.refreshVisible()
	return nil
}

// reloadOrStatus reloads and surfaces any failure in the status line. Used by
// the action handlers, which have nowhere else to return an error to.
func (m *Model) reloadOrStatus() {
	if err := m.reload(); err != nil {
		m.setStatus("", err)
	}
}

func (m *Model) refreshTagsOrStatus() {
	if err := m.refreshTags(); err != nil {
		m.setStatus("", err)
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
	sort.Slice(m.visible, func(i, j int) bool {
		if m.visible[i].Tag != m.visible[j].Tag {
			return m.visible[i].Tag < m.visible[j].Tag
		}
		return m.visible[i].Name < m.visible[j].Name
	})
	m.noteCur = clamp(m.noteCur, len(m.visible))
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

// selectedTag returns the focused tag, or "" for the synthetic "all" row.
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

func (m Model) notebookCount(nb string) int {
	n := 0
	for _, note := range m.all {
		if note.Notebook == nb {
			n++
		}
	}
	return n
}

func (m Model) tagCount(nb, tag string) int {
	n := 0
	for _, note := range m.all {
		if note.Notebook == nb && note.Tag == tag {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------- update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.l = computeLayout(msg.Width, msg.Height)
		return m, nil

	case actionMsg:
		return m.applyAction(msg)

	case editorDoneMsg:
		return m.afterEditor(msg)

	case gitDoneMsg:
		m.setStatus(msg.text, msg.err)
		return m, nil

	case tea.KeyMsg:
		if m.ov.active() {
			ov, cmd, done := m.ov.update(msg)
			m.ov = ov
			if done {
				m.ov = overlay{}
			}
			return m, cmd
		}
		return m.handleKey(msg.String())
	}

	// Non-key messages still need to reach an open text input (paste, blink).
	if m.ov.active() {
		ov, cmd, done := m.ov.update(msg)
		m.ov = ov
		if done {
			m.ov = overlay{}
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case keys.quit.matches(key), key == "esc":
		return m, tea.Quit

	case keys.help.matches(key):
		m.ov = newHelp()
		return m, nil

	case keys.nextPane.matches(key):
		m.focus = (m.focus + 1) % 3
		return m, nil

	case keys.prevPane.matches(key):
		m.focus = (m.focus + 2) % 3
		return m, nil

	case keys.up.matches(key):
		m.moveCursor(-1)
		return m, nil

	case keys.down.matches(key):
		m.moveCursor(1)
		return m, nil

	case keys.top.matches(key):
		m.setCursor(0)
		return m, nil

	case keys.bottom.matches(key):
		m.setCursor(m.paneLen() - 1)
		return m, nil

	case keys.enter.matches(key):
		if m.focus != paneNotes {
			m.focus = paneNotes
			return m, nil
		}
		return m.openSelected()

	case keys.search.matches(key):
		return m.startSearch(), nil

	case keys.newItem.matches(key):
		return m.startNew()

	case keys.rename.matches(key):
		return m.startRename()

	case keys.delete.matches(key):
		return m.startDelete()

	case keys.move.matches(key) && m.focus == paneNotes:
		return m.startMove()

	case keys.sync.matches(key):
		return m, gitCmd("sync", m.store.Sync)

	case keys.backup.matches(key):
		return m, m.backupCmd()
	}
	return m, nil
}

func (m Model) paneLen() int {
	switch m.focus {
	case paneNotebooks:
		return len(m.notebooks)
	case paneTags:
		return len(m.tags) + 1
	default:
		return len(m.visible)
	}
}

func (m *Model) moveCursor(delta int) { m.setCursor(m.cursor() + delta) }

func (m Model) cursor() int {
	switch m.focus {
	case paneNotebooks:
		return m.nbCur
	case paneTags:
		return m.tagCur
	default:
		return m.noteCur
	}
}

func (m *Model) setCursor(to int) {
	to = clamp(max(to, 0), m.paneLen())
	switch m.focus {
	case paneNotebooks:
		if to == m.nbCur {
			return
		}
		// A different notebook means a different tag list; reset the tag
		// filter to "all" so the note pane is never empty by surprise.
		m.nbCur, m.tagCur = to, 0
		m.refreshTagsOrStatus()
	case paneTags:
		m.tagCur = to
		m.refreshVisible()
	default:
		m.noteCur = to
	}
}

func (m *Model) setStatus(text string, err error) {
	if err != nil {
		m.status, m.statusErr = err.Error(), true
		return
	}
	m.status, m.statusErr = text, false
}

// ---------------------------------------------------------------- view

func (m Model) View() tea.View {
	body := make([]string, 0, m.l.Body)

	if m.ov.active() {
		body = m.ov.render(m.l.Width-2, m.l.Body)
	} else {
		sidebar := m.renderSidebar()
		main := m.renderNotes()
		if m.l.Sidebar == 0 {
			body = append(body, main...)
		} else {
			for i := 0; i < m.l.Body; i++ {
				left, right := "", ""
				if i < len(sidebar) {
					left = sidebar[i]
				}
				if i < len(main) {
					right = main[i]
				}
				body = append(body, pad(left, m.l.Sidebar)+frameStyle.Render("│")+pad(right, m.l.Main))
			}
		}
	}

	// The status line replaces the last body row when there is something to
	// say. An open overlay owns the whole body, so it is left alone.
	if m.status != "" && len(body) > 0 && !m.ov.active() {
		style := goodStyle
		glyph := "✓ "
		if m.statusErr {
			style, glyph = badStyle, "✗ "
		}
		body[len(body)-1] = "  " + style.Render(glyph) + textStyle.Render(m.status)
	}

	v := tea.NewView(frame(m.title, body, m.l.Width, m.l.Body) + "\n" + footer(m.focus))
	v.AltScreen = true
	return v
}

// ---------------------------------------------------------------- session

type editorDoneMsg struct {
	err      error
	warnings []string
}

type gitDoneMsg struct {
	text string
	err  error
}

func gitCmd(label string, run func() error) tea.Cmd {
	return func() tea.Msg {
		if err := run(); err != nil {
			return gitDoneMsg{err: err}
		}
		return gitDoneMsg{text: label + " complete"}
	}
}

// editSession adapts session.Open to Bubble Tea's ExecCommand interface, so
// the runtime releases the terminal for helix and restores it afterwards.
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
		// Collected rather than printed: helix is about to take the screen.
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
	// The note's mtime changed, and helix may have been used to create files.
	if err := m.reload(); err != nil {
		m.setStatus("", err)
		return m, nil
	}
	switch {
	case msg.err != nil:
		m.setStatus("", msg.err)
	case len(msg.warnings) > 0:
		m.setStatus(strings.Join(msg.warnings, "; "), nil)
		m.statusErr = true
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
