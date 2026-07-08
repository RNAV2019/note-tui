# `note` — Typst lecture-notes CLI (design)

Date: 2026-07-08
Status: approved pending user review

## Goal

Replace quasar with a low-maintenance workflow: Helix as the editor, Typst as
the note format, tinymist for live preview, and a small aesthetic CLI called
`note` that manages the note tree and launches the edit+preview session.

## Requirements

Needs (all must hold):
- Local-first plain files.
- Formatted text, tables, code blocks with per-language syntax highlighting,
  math with live preview — all provided by Typst + tinymist.
- Quick diagrams — fletcher / CeTZ Typst packages, imported by the template.

Wants:
- Open source throughout (Go CLI, Typst, tinymist, Helix, Helium — all FOSS).
- Sync — git-based `note backup` / `note sync`.
- AI — out of scope for the CLI; plain `.typ` files work directly with any
  terminal AI tool (aichat, Claude Code) using an OpenRouter key.

## Note tree

```
~/Documents/notes/            # git repo, bootstrapped on first run
├── .template.typ             # shared lecture-note template
├── <notebook>/               # top-level directory, e.g. "university"
│   └── <tag>/                # module folder, e.g. "algorithms"
│       └── <name>.typ        # the note
```

- Bootstrap: on any command, if `~/Documents/notes` is missing, create it,
  `git init`, and write a default `.template.typ`.
- The template defines a header (title, module/tag, date), sensible page/text
  settings, and imports fletcher and CeTZ so diagrams are always available.
  New notes are created by filling the template's title/tag/date placeholders.

## CLI

Go, using the charmbracelet stack: Bubble Tea + bubbles (fuzzy-filter list),
huh (forms, select, confirm), lipgloss (styling). One consistent theme across
all commands. No raw fzf.

Commands:

| Command | Behaviour |
|---|---|
| `note new` | huh select notebook → select tag (or "new tag…" inline) → input note name → create file from template → open flow |
| `note find` | Bubble Tea fuzzy-filter list of all notes as `notebook/tag/name` → open flow |
| `note nb new\|delete\|list` | Manage notebooks. Delete shows a styled confirmation naming the notebook and its note count |
| `note tag new\|delete\|list` | Manage tag folders within a chosen notebook. Same confirmation behaviour |
| `note delete` | Fuzzy-pick a note → confirm → delete file |
| `note backup` | `git add -A && git commit && git push` in the notes repo |
| `note sync` | `git pull` in the notes repo |

Notes:
- All destructive operations (nb delete, tag delete, note delete) require an
  explicit huh confirmation dialog stating exactly what is removed.
- `new` / `find` with zero notebooks prompts to create one instead of erroring.
- Name validation: notebook/tag/note names are slugified to filesystem-safe
  kebab-case; the display name may keep spaces (stored as the filename slug,
  shown prettified).

## Open flow (edit + live preview)

On `note new` / `note find` selection:

1. Spawn `tinymist preview <file> --no-open` in the background; parse the
   preview URL from its output.
2. Launch the preview window: `helium --app=<url>` (default; the command is a
   template string in config, e.g. `helium --app={url}`).
3. `exec` into an interactive `hx <file>` in the foreground.
4. When Helix exits, kill the tinymist process and the browser window process.

Failure handling: if tinymist fails to start or the URL never appears within a
timeout, warn (styled) and still open Helix — editing must never be blocked by
preview problems.

## Configuration

`~/.config/note/config.toml`:

```toml
notes_dir = "~/Documents/notes"      # override allowed
editor    = "hx"                      # command to open files
preview   = "helium --app={url}"      # {url} substituted
```

Defaults are used when the file is absent; no config is required.

## Helix integration (documented, not code)

The repo's README documents the `languages.toml` snippet enabling tinymist as
the LSP for Typst (completions, hover, format on save via `tinymist`).

## Repo & packaging

- Repo: `~/Projects/note`, Go module, cobra for command routing (matches
  quasar's structure the user already knows).
- Nix flake: packages the `note` binary and a devShell; runtime deps (typst,
  tinymist, helix, helium) are already in the user's NixOS profile, so the
  flake wraps but does not force them.

## Testing

- Unit tests for the pure parts: tree scanning/listing, slugification,
  template rendering, tinymist URL parsing, config loading.
- TUI flows (huh/bubbletea) and process spawning are covered by thin
  interfaces so the command logic is testable without a TTY; interactive
  behaviour is verified manually.

## Out of scope

- AI features, PDF bundling, in-CLI markdown migration of old quasar notes
  (pandoc can convert them ad hoc), Helix config management.
