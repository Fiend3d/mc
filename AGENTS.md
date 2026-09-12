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
- **Go**: 1.27.0
- **External deps**: `bat` + `less` (from Git), `hx` (helix), `code`
- **Recommended**: Windows Terminal (for mouse support), JetBrainsMonoNL Nerd Font
- **Config**: `$env:APPDATA\mc\config.toml` — theme, F-key tools
- **PowerShell wrapper** (see `scripts/readme.md`) needed for `cd` on quit

Run `Set-ExecutionPolicy RemoteSigned -Scope CurrentUser` if PowerShell scripts fail.

## Architecture

Single Go module (`module mc`), single `package main` plus `widgets/` (spinner, textinput, cursor, key, runeutil) and `shutil/`. Uses the released catatui (`github.com/Fiend3d/catatui v0.1.0`) from the module proxy; there is no local go.mod replacement.

### Key files

| File | Role |
|------|------|
| `main.go` | Entrypoint, CLI flag parsing |
| `model.go` | `model` struct (state), mode constants, initialization |
| `update.go` | Message handling + event loop (`Update()`) — largest file (~1470 lines) |
| `view_native.go` | Native catatui pane, dialog and task rendering |
| `runtime.go` | catatui terminal ownership, input and effect scheduling |
| `tasks.go` | Cancellable tasks, progress, undo/redo. File operations run one at a time; read-only tasks (`task.readOnly`, e.g. calculate size) skip the queue and run alongside them |
| `shutil/transfer.go` | Atomic copy, safe move and operation journals |
| `handle.go` | Action handlers (quit, paste, rename, tools, clipboard copy) |
| `commands.go` | Async command wrappers, directory reading, file ops |
| `commandmanager.go` | Command pattern (undo/redo); delete is NOT undoable |
| `item.go` | Item interface + 3 implementations: `filepathItem`, `driveItem`, `sharedItem` |
| `git.go` | Git status for the listing: repository lookup, `git status --porcelain` parsing, roll-up onto directories |
| `git_list.go` | The overlay behind a path-row tally: a snapshot of the repository's changed files, its cursor and its jump |
| `view_git.go` | Git list rendering |
| `search.go` | Full-text/content search with gitignore support |
| `sort.go` | Sorting methods (modified time, alpha, extension, size, random) |
| `tab.go` | Tab structure, navigation history, forward/back |
| `mouse.go` | Mouse click tracking and double-click detection |
| `bookmarks.go` | Bookmarks data structure and cursor management |
| `theme.go` | 8 themes (dracula is default) |
| `clipboard.go` | Windows CF_HDROP clipboard integration |
| `drives.go` | Drive enumeration via Windows API |
| `net.go` | Network share enumeration via Netapi32 |
| `config.go` | TOML config, bookmarks, shell history |
| `utils.go` | Path utilities, autocomplete, file ops, `uniquePath` naming |
| `view_help.go` | Help view rendering with topics |
| `view_bookmarks.go` | Bookmarks view rendering |
| `view_tabs.go` | Tabs view rendering |
| `view_utils.go` | View utility functions (colorizeDir, truncate) |
| `shutil/shutil.go` | File system utility functions |

### Modes

Access via keybindings: normal, hidden, help, helpFilter, go, confirmDialog, jump, messages, tabs, filter, sort, rename, create, path, copy, bookmarks, search, shell, theme, transfer. Tasks and quit confirmation are global overlays.

## Build Process

No fork needed — all widget components live in `widgets`.

## Testing

Run `go test ./...`, `go test -race ./...`, and `go vet ./...`. Tests cover panes, tasks, file operations, native rendering, Unicode editing, and ConPTY input/process handoff. `review_test.go` covers refresh bursts and errors, real filesystem notifications, cursor/viewport preservation, overlay mouse isolation, and Unicode breadcrumb hit testing.

## Key Conventions

- `SHELL = "powershell"` (hardcoded in `config.go`)
- `#sl` macro in shell mode expands to selected file paths
- File filter uses comma/semicolon-separated patterns (case-insensitive `Contains`)
- Delete and overwrites are not undoable; other completed file work is journaled, including partial operations.
- Tab switches panes; Ctrl+Left/Right move the current tab across panes and Shift+Left/Right copy it; Ctrl+J enters Jump; Shift+D compares the cursor file in each pane (ignoring marked selections); Y/X transfer to the opposite pane; w opens tasks.
- Compare is a read-only snapshot with cancellable, generation-guarded worker results. `compare.go` handles loading and bounded diffs; `view_compare.go` renders aligned lines. Its input must not reach pane actions. F5 reloads the captured paths.
- Pane/tab state is independent. Async reads carry the target tab, page and generation.
- A pane may hold zero tabs. `pane.hasTabs()` guards it; the key gate in `Update` (`emptyPaneBlocked`) swallows tab-dependent keys so handlers can keep calling `getTab()`. Use `currentDir()`/`paneDir()` where only a path is needed. An empty pane parks `currentTab` at 0, never -1.
- Only the UI loop mutates model state; workers send immutable progress/completion events.
- Git status is read in the background after every directory read and applied through `gitStatusMsg`, guarded by `liveTab` and `tab.gitGeneration` like directory reads. `findRepoRoot` keeps mc from spawning git outside a repository; markers are calculated data, so they are carried across re-reads and excluded from `unchangedListing`. The path row is split by `gitPathLane` — breadcrumb on the left, repository summary on the right — and both `drawPane` and the breadcrumb click handler must size it through that helper or hit testing drifts from what is drawn. The summary is a list of `gitSegment`s: rendering, hover and `gitSummaryAtX` all walk it, so what is clickable is defined once. `gitTally` groups states for both the counters and the lists, which keeps a tally and its list the same length.
- Themes set via `g -> T`, saved via `g -> C`
- The right pane's tab directories persist in `tabs.list` beside `config.toml`/`bookmarks.list`; `run` saves them on exit and `initialModel` restores them when no second directory argument is given. The left pane never persists.
- Binary files in search are detected by null-byte scan (first 8KB); 5MB size limit for text search
