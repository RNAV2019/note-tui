# note

`note` is a command-line tool for managing Typst lecture notes. Notes are stored as `.typ` files under `~/Documents/notes/<notebook>/<tag>/<name>.typ`. When you create or open a note, `note` launches your editor (Helix by default) alongside a live Typst preview served by tinymist and displayed in a Helium app-mode window — so you write in your terminal and the rendered PDF updates in real time beside it.

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

## Commands

| Command | Description |
|---|---|
| `note new` | Prompt for a notebook, tag, and title; create the note; open it in Helix with live preview |
| `note find` | Fuzzy-find across all notes; open the selected note in Helix with live preview |
| `note delete` | Fuzzy-find a note and delete it |
| `note nb new` | Create a new notebook directory |
| `note nb list` | List all notebooks |
| `note nb delete` | Delete a notebook and all its contents |
| `note tag new` | Create a new tag (module folder) inside a notebook |
| `note tag list` | List all tags inside a notebook |
| `note tag delete` | Delete a tag and all its notes |
| `note backup` | Commit every change in `~/Documents/notes` and push to the configured remote |
| `note sync` | Pull the latest notes from the remote |

## Configuration

`note` reads `~/.config/note/config.toml` on startup. All keys are optional; the defaults are shown below.

```toml
notes_dir = "~/Documents/notes"
editor    = "hx"
preview   = "helium --app={url}"
```

- **`notes_dir`** — root directory for all notebooks and notes.
- **`editor`** — command used to open a note. Must accept a file path as its last argument.
- **`preview`** — command used to open the live preview window. `{url}` is replaced with the tinymist preview URL at runtime.

## Template

The note template lives at `~/Documents/notes/.template.typ` and is created automatically on first run. Edit it to customise how new notes look.

Supported placeholders:

| Placeholder | Replaced with |
|---|---|
| `{{title}}` | The note title you enter at creation time |
| `{{tag}}` | The tag (module folder) the note belongs to |
| `{{date}}` | The creation date in `YYYY-MM-DD` format |

The default template imports [cetz 0.4.2](https://typst.app/universe/package/cetz) and [fletcher 0.5.8](https://typst.app/universe/package/fletcher) so diagrams and graphs are available out of the box without any extra setup.

## Helix LSP setup

Add the following to `~/.config/helix/languages.toml` to enable Typst completions, hover documentation, and auto-formatting in Helix:

```toml
[[language]]
name = "typst"
language-servers = ["tinymist"]
formatter = { command = "typstyle" }

[language-server.tinymist]
command = "tinymist"
```

This requires `tinymist` and (optionally) `typstyle` on your `PATH`. Both are available in nixpkgs.

## Sync

`note backup` and `note sync` delegate to git. Set up a remote once:

```bash
git -C ~/Documents/notes remote add origin <url>
```

After that, `note backup` commits all changes and pushes, and `note sync` pulls the latest commits with `git pull --rebase`. Conflicts are left for you to resolve with standard git tooling.
