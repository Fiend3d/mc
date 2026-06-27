# Modal Commander (mc) — Agent Guide

## Build & Run

```powershell
.\build.ps1               # builds mc.exe (force-cleans stale binary)
.\build.ps1 dist          # builds + zips dist/
.\build.ps1 icon          # embed icon (needs `rsrc` tool)
```

Version info is injected via ldflags at build time (`Version`, `GitCommit`, `BuildTime`). Binary is force-deleted before each build to avoid stale version strings.

## CLI Flags

| Flag | Purpose |
|------|---------|
| `-v` / `-version` | Print version |
| `-o` | Enable temp-file output (for `cd` wrapper) |
| `-tf path` | Temp file path (default `output.tmp`) |

Pass directories as positional args to open them on launch.

## Setup Requirements

- **OS**: Windows only (Win32 API, CF_HDROP clipboard, Netapi32)
- **Go**: 1.26.1
- **External deps**: `bat` + `less` (from Git), `hx` (helix), `code`
- **Recommended**: Windows Terminal (for mouse support), JetBrainsMonoNL Nerd Font
- **Config**: `$env:APPDATA\mc\config.toml` — theme, F-key tools
- **PowerShell wrapper** (see `scripts/readme.md`) needed for `cd` on quit

Run `Set-ExecutionPolicy RemoteSigned -Scope CurrentUser` if PowerShell scripts fail.

## Architecture

Single Go module (`module mc`), single `package main` plus `internal/widgets/` (spinner, textinput, cursor, key, runeutil). Uses Bubble Tea v2 (`charm.land/bubbletea/v2`).

### Key files

| File | Role |
|------|------|
| `main.go` | Entrypoint, CLI flag parsing |
| `model.go` | `model` struct (state), mode constants, initialization |
| `update.go` | Message handling + event loop (`Update()`) — largest file (~1400 lines) |
| `view.go` | Rendering (`View()`) |
| `handle.go` | Action handlers (quit, paste, rename, tools, clipboard copy) |
| `commands.go` | Async command wrappers, directory reading, file ops |
| `commandmanager.go` | Command pattern (undo/redo); delete is NOT undoable |
| `item.go` | Item interface + 3 implementations: `filepathItem`, `driveItem`, `sharedItem` |
| `search.go` | Full-text/content search with gitignore support |
| `theme.go` | 8 themes (dracula is default) |
| `clipboard.go` | Windows CF_HDROP clipboard integration |
| `drives.go` | Drive enumeration via Windows API |
| `net.go` | Network share enumeration via Netapi32 |
| `config.go` | TOML config, bookmarks, shell history |
| `utils.go` | Path utilities, autocomplete, file ops, `uniquePath` naming |

### Modes (19 total)

Access via keybindings: normal, visual, jump, filter, sort, rename, create, copy, path, shell, search, go, theme, bookmarks, tabs, messages, help, hidden, confirm dialog.

## Build Process

No fork needed — all widget components live in `internal/widgets/`.

## Testing

No test files exist.

## Key Conventions

- `SHELL = "powershell"` (hardcoded in `config.go`)
- `#sl` macro in shell mode expands to selected file paths
- File filter uses comma/semicolon-separated patterns (case-insensitive `Contains`)
- Delete is permanent + undoable only for file actions (copy/move/rename), NOT for delete
- Themes set via `g -> T`, saved via `g -> C`
- Binary files in search are detected by null-byte scan (first 8KB); 5MB size limit for text search
