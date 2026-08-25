# note-tui

`note-tui` is a keyboard-driven TUI for managing Typst lecture notes; it installs as the `note` command. Notes are stored as `.typ` files under `~/Documents/notes/<notebook>/<tag>/<name>.typ`, so the tree on disk is exactly what you see in the interface: notebooks group tags, tags group notes, and a note's tag *is* its parent directory.

When you create or open a note, `note` hands the terminal to your editor (Helix by default) alongside a live Typst preview served by Helix's own tinymist language server and displayed in a Helium app-mode window — so you write in your terminal and the rendered document updates as you type beside it. Quit the editor and you land back in the TUI. See [Helix setup](#helix-setup), which the preview depends on.

```
╭─ note 0.2.0 ───────────────────────────────────────────────────────╮
│  Notebooks             │                                           │
│  ❯ year-1          12  │  year-1 / cs118                           │
│    year-2           4  │  ───────────────────────────────────────  │
│                        │  ❯ b-trees                        2d ago  │
│  Tags                  │    dijkstra                       1w ago  │
│    all             12  │    hash-tables                    5d ago  │
│  ❯ cs118            3  │                                           │
│    cs126            9  │                                           │
│                        │                                           │
╰────────────────────────────────────────────────────────────────────╯
  n new · r rename · d delete · m move · / search · tab pane · ? keys · q quit
```

## Requirements

The following programs must be on your `PATH`:

- [`typst`](https://typst.app/) — Typst compiler
- [`tinymist`](https://github.com/Myriad-Dreamin/tinymist) — Typst language server and preview server
- [`hx`](https://helix-editor.com/) — Helix editor
- [`helium`](https://github.com/Alex-Shand/helium) — browser launcher used for the preview window

Alternatively, use the included Nix flake to get a reproducible environment:

```bash
nix build          # builds ./result/bin/note
nix develop        # drops you into a shell with go, typst, and tinymist
```

The dev shell intentionally omits `hx` and `helium` — install those through your regular profile (they're editor/browser choices, not build dependencies).

Add `result/bin` to your `PATH`, or copy `result/bin/note` to `~/.local/bin`.

## Command line

There are only two invocations — everything else lives in the TUI.

| Command | Description |
|---|---|
| `note` | Open the TUI |
| `note --version`, `note -v` | Print the version and exit |

## Keys

The interface has three panes — **Notebooks** and **Tags** in the sidebar, and the note list on the right. The sidebar acts as a filter: pick a notebook, then either a tag or the synthetic `all` row, and the note list follows.

| Key | Action |
|---|---|
| `j` / `k`, `↑` / `↓` | Move within the focused pane |
| `g` / `G` | Jump to the first / last item |
| `tab` / `shift+tab`, `h` / `l` | Cycle panes |
| `enter` | In the sidebar, focus the note list. In the note list, open the note |
| `n` | New note, notebook or tag — whichever the focused pane holds |
| `r` | Rename the selected item |
| `d` | Delete the selected item (asks first, and tells you how many notes go with it) |
| `m` | Move a note to another notebook or tag |
| `/` | Fuzzy search every note, across all notebooks |
| `S` / `B` | Git sync / backup |
| `?` | Show all keys |
| `q`, `esc` | Quit (`esc` closes an open dialog first) |

Creating a note only asks what it has to: with a tag already selected it prompts for the title alone, and only the `all` view stops to ask which tag. The new note opens in your editor immediately.

Renaming and moving are the same operation on disk as anywhere else in the tree — because a tag is a directory, `m` is how you retag a note after the fact. Names are slugified, so "Hash Tables" becomes `hash-tables.typ`.

## Configuration

`note` reads `~/.config/note/config.toml` on startup. All keys are optional; the defaults are shown below.

```toml
notes_dir       = "~/Documents/notes"
editor          = "hx"
preview         = "helium --app={url} --class=note-preview --user-data-dir={profile} --no-first-run --no-default-browser-check"
preview_url     = "http://127.0.0.1:23635"
preview_profile = "~/.cache/note/preview-profile"
```

- **`notes_dir`** — root directory for all notebooks and notes.
- **`editor`** — command used to open a note. Must accept a file path as its last argument.
- **`preview`** — command used to open the live preview window. `{url}` is replaced with `preview_url` and `{profile}` with this session's profile directory.
- **`preview_url`** — where tinymist serves the preview. **Must match `--data-plane-host` in your Helix config** (see below); tinymist serves the preview page from that same address.
- **`preview_profile`** — where `note` keeps browser profiles. Each editing session gets a fresh profile underneath it, which is what lets `note` close the preview window when you quit the editor: a Chromium-based browser launched against a profile another instance already holds hands its window over and exits immediately, leaving a window nothing can close. Profiles left behind by a crashed session are reclaimed on the next start. If you point `preview` at a browser of your own, keep `{profile}` in the command.

## Template

The note template lives at `~/Documents/notes/.template.typ` and is created automatically on first run. Edit it to customise how new notes look.

Supported placeholders:

| Placeholder | Replaced with |
|---|---|
| `{{title}}` | The note title you enter at creation time |
| `{{tag}}` | The tag (module folder) the note belongs to |
| `{{date}}` | The creation date in `YYYY-MM-DD` format |

The default template imports [cetz 0.4.2](https://typst.app/universe/package/cetz) and [fletcher 0.5.8](https://typst.app/universe/package/fletcher) so diagrams and graphs are available out of the box without any extra setup.

## Helix setup

This is **required**, not optional — the live preview is owned by Helix's tinymist language server, not by `note`. Add the following to `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "typst"
language-servers = ["tinymist"]
formatter = { command = "typstyle" }

[language-server.tinymist]
command = "tinymist"

[language-server.tinymist.config.preview.background]
enabled = true
args = ["--data-plane-host=127.0.0.1:23635", "--invert-colors=never"]
```

This requires `tinymist` and (optionally) `typstyle` on your `PATH`. Both are available in nixpkgs.

Two details matter:

- **`--data-plane-host` must match `preview_url`** in `~/.config/note/config.toml`. tinymist serves the preview page and the document stream from this one address, so `note` knows where to point the preview window without guessing a port.
- **Don't add `--open`.** That flag makes tinymist launch your *default* browser, which would give you a second window next to the one `note` opens.

Because the language server holds the buffer, the preview updates as you type — not just on save — and clicking in the preview moves your cursor in Helix.

### How a note opens

1. `note` launches Helix in the foreground.
2. Helix starts tinymist, which starts the preview server on `preview_url`.
3. `note` waits for that address to accept connections, then runs the `preview` command to open the window.
4. Quitting Helix closes the preview window and returns you to the TUI.

If the preview server never appears, `note` says so in the status line and you still get your editor — editing is never blocked by a broken preview.

## Sync

`~/Documents/notes` is a git repository, initialised on first run. The `B` and `S` keys delegate to git. Set up a remote once:

```bash
git -C ~/Documents/notes remote add origin <url>
```

After that, `B` commits all changes and pushes, and `S` pulls the latest commits with `git pull --rebase`; the result appears in the status line. Without a remote, `B` still commits locally and says so. Conflicts are left for you to resolve with standard git tooling.
