# Modal Commander

Modal Commander (mc) is a TUI file manager for Windows (though it might be ported to other platforms in the future). It's heavily inspired by Yazi, Helix, and Total Commander.

## Version 2.0

mc now uses [catatui](https://github.com/Fiend3d/catatui), with two independent panes and background file tasks. Each pane has its own tabs, history, selection, filter and sorting. Existing themes and F-key tool configuration are retained.

- `Tab` switches panes. Click a pane to focus it; click a pane tab to switch directly.
- `Ctrl+Left` / `Ctrl+Right` move the current tab to that pane and follow it; `Shift+Left` / `Shift+Right` copy it there and keep the focus.
- A pane may hold no tabs at all: `Ctrl+W` closes the last one and a move can empty the source. The empty pane keeps its half of the screen; `T` restores the last closed tab, `gg` opens a path, `b` picks a bookmark, and `Shift+Left`/`Shift+Right` copies one in from the other side. Keys that need a current directory do nothing there. Quitting from an empty pane returns no directory.
- `Space` / `Insert` toggles an item and advances; select-all, invert and clear remain available. Visual mode has been removed.
- `Shift+Up/Down` extends or shrinks a range; `Shift+Home/End` extends to the first or last item; `Ctrl+click` extends to the clicked item. Earlier selections are preserved. Ordinary navigation starts a new range anchor. `Shift+click` also works in terminals that forward it, but Windows Terminal reserves it for terminal text selection.
- `Y` copies and `X` moves selected items to the opposite pane. Enter confirms the editable destination. Existing `y/x/p/P` clipboard operations are unchanged.
- `w` opens tasks; `c` cancels the highlighted task and Escape returns to browsing. File operations run sequentially while browsing remains available. Progress shows bytes and file counts, with scanning shown before totals are known.
- `Shift+Tab` enters Jump mode. `Ctrl+N` still opens a directory in a new tab.

Normal transfers choose unique names on collisions; `P` explicitly requests overwrite and confirms collisions. Cancellation retains completed files and removes unfinished temporary copies. Completed reversible work can be undone, including partial tasks. Delete is permanent; overwrites are not undoable. Undo/redo refuses conflicting or changed paths. Reparse points are reported as unsupported for transfers/deletion.

No arguments opens the working directory in the left pane and leaves the right one empty; a single directory argument does the same. Two directories initialize left/right; extra directories become left-pane tabs. The focused pane supplies the directory returned by the PowerShell wrapper. The right pane's tabs are saved to `$env:APPDATA\mc\tabs.list` when mc exits and restored at the next launch, unless a second directory argument names its directory. The left pane and task history are not persisted.

Set a theme with `g` -> `T`, save with `g` -> `C`, or edit `$env:APPDATA\mc\config.toml`.

### Building v2

Go 1.27 and Windows are required. The module depends on the released `github.com/Fiend3d/catatui v0.1.0`, so no sibling checkout is needed. Run `.\build.ps1` or `.\build.ps1 dist`. The generated executable and zip are standalone.

Run `go test ./...`, `go test -race ./...`, and `go vet ./...`. Tests include native catatui rendering, queued transfers, cancellation, partial undo/redo, Unicode editing, and an actual Windows pseudoconsole process handoff. When catatui needs changes, publish a new catatui tag and bump the requirement here rather than adding a local replacement.

![mc v2 with two panes and a background transfer](assets/demo/v2.png)

## v1 screenshots

![RECLibboard](assets/demo/demo01.png)
![RECLibboard](assets/demo/demo02.png)
![RECLibboard](assets/demo/demo03.png)
![RECLibboard](assets/demo/demo04.png)

`mc` can easily calculate the sizes of directories and sort them by size:

![RECLibboard](assets/demo/demo05.png)
![RECLibboard](assets/demo/demo.gif)

## How to install

F3 uses the configured viewer (`koneko` by default), and F4 uses [Helix](https://helix-editor.com/). You can configure `bat` with the `less` pager as an alternative viewer in config.toml.

I also recommend using [Windows Terminal](https://github.com/microsoft/terminal) because it's the only terminal emulator, that I found, that makes the mouse work properly on Windows 10. It also looks kinda good if you install https://www.nerdfonts.com/font-downloads specifically `JetBrainsMonoNL Nerd Font`.

Here is how you can configure your powershell to `cd` to the directory when you exit: https://github.com/Fiend3d/mc/tree/master/scripts you can also find there `t.bat` that makes launching Windows Terminal a lot easier, because by default it doesn't open the current directory and typing just `t` is convenient.

## How to use

Pressing `F1` shows documentation that can be filtered by pressing `f`. I tried to make `mc` as intuitive as possible, and for the most part, everything is accessible with the mouse.

### Normal Mode

The main mode of the program, from which most other modes can be accessed.

**q** - Quit, returning the current directory.<br/>
**Q** - Quit without returning anything.<br/>
**space** - Select.<br/>
**Shift+Up / Shift+Down** - Extend or shrink the selection range.<br/>
**Shift+Home / Shift+End** - Extend the selection range to the first or last item.<br/>
**Ctrl+click** - Extend the selection range to the clicked item.<br/>
**Ctrl+a** - Select all.<br/>
**Ctrl+d** - Deselect all.<br/>
**Ctrl+r** - Toggle selection (invert all).<br/>
**y** - Copy selected items. This uses standard Windows file paths, so you can paste them directly into Explorer.<br/>
**x** - Cut.<br/>
**d** - Delete PERMANENTLY. It will prompt for confirmation.<br/>
**r** - Rename. When multiple items are selected, an editor opens so you can edit all the names at once.<br/>
**p** - Paste.<br/>
**P** - Paste with override. Prompts for confirmation if there's a collision.<br/>
**u** - Undo.<br/>
**U** - Redo.<br/>
**t** - Copy current tab.<br/>
**Ctrl+w** - Close current tab.<br/>
**T** - Restore closed tab.<br/>
**Ctrl+n** - Open selected directory in a new tab.<br/>
**]** - Next tab.<br/>
**[** - Previous tab.<br/>
**1-0** - Select tabs 1 to 10 (0 is tab 10).<br/>
**Ctrl+b**  - Go back in history.<br/>
**Ctrl+f** - Go forward in history.<br/>
Directories update automatically from filesystem notifications, with periodic checks for missed changes and clipboard updates. Refresh keeps the focused file and scroll position when those items still exist. **F5** forces an immediate update.<br/>
<br/>
**B** - Bookmark the directory.<br/>
**b** - Browse bookmarks.<br/>

### Jump Mode

Can be entered by pressing `Shift+Tab` in the normal mode. Jump mode is to mimic Explorer's behavior when pressing buttons to jump to the needed item.

### Filter Mode

Entered by pressing `f` in the normal mode. Current tab can be filtered.

### Copy Mode

Entered by pressing `c`. Capital letters convert slashes from `\` to `/`. "Copy the filenames as arguments" means it can be used as arguments for terminal commands (if path has spaces it will be quoted).

**c/C** - Copy the file path/Forward.<br/>
**d/D** - Copy the directory/Forward.<br/>
**f** - Copy the filename.<br/>
**n** - Copy the filename without extension.<br/>
**a/A** - Copy the file paths as arguments/Forward.<br/>
**s** - Copy the filenames as arguments.<br/>
**q/Q** - Copy the file paths as array/Forward.<br/>
**w** - Copy the filenames as array.<br/>

### Sort Mode

Entered by pressing `,` (comma). Capital letters sort in reverse.

**m/M** - Sort by modified time.<br/>
**a/A** - Sort alphabetically.<br/>
**n/N** - Sort normally.<br/>
**e/E** - Sort by extension.<br/>
**s/S** - Sort by size.<br/>
**r** - Sort randomly.<br/>

### Create Mode

Entered by pressing `a`. If your name ends with a slash it's a directory.

### Message Mode

Entered by pressing `` ` `` (backtick). The message history can be viewed here.

### Search Mode

Press `s` to enter search mode. Use `tab` to cycle through focus. By default, search respects `.gitignore`, but you can disable this by pressing `F1` while in search mode. Search is case-insensitive by default; toggle with `F2`.

Press `F3` on a line to open it with `bat`; it will jump directly to that line. Press `n` to jump to the next match (or `N` to go backwards). Press `h` to hide all matched lines.

`F5` or `Enter` while focussing a text input - start searching.

### Shell Mode

Press `:` to enter shell mode. You can hide and show TUI by pressing `Ctrl+h` to see the result of a command. `#sl` - is a macro that is converted to a list of selected items for a command.

Press `;` in normal mode to rerun the last shell command without reopening shell mode. `#sl` expands against the current selection, so the same command can be applied to different items.

**Ctrl+b** - Back in history.<br/>
**Ctrl+f** - Forward in history.<br/>

### Go Mode

Go mode is just a menu.

**g** - Enter Path mode.<br/>
**t** - Browse tabs.<br/>
**T** - Set theme.<br/>
**c** - Open the settings directory. You can also find and delete bookmarks there, for example.<br/>
**C** - Save settings to config.toml for editing.<br/>
**s** - Calculate size for the selected directories. Runs as a background task alongside file operations; watch or cancel it with **w**.<br/>

### Path Mode

Press `gg` to enter path mode.

**ctrl+u** - Clear all left of cursor.<br/>
**ctrl+k** - Clear all right of cursor.<br/>
**ctrl+w** - Delete a word.<br/>
**tab** - Autocomplete.<br/>
**up/down** - Next/previous autocomplete.<br/>
**ctrl+e** - Expand environment variables.<br/>
**ctrl+n** - Open the path in a new tab.<br/>

### Tools

`F2-F4`, `F6-F12` - tools. They can be configured in `config.toml`. The default config can be saved by pressing `gC` (`g` and then `C`, and then `gc` to find it).

**F2** - Dependency walker. [deps](https://github.com/Fiend3d/deps) by default, but everything is configurable.<br/>
**F3** - Viewer.<br/>
**F4** - Editor.<br/>
**F6** - Open the directory in Explorer.<br/>
**F7** - Open the files in VS Code.<br/>
**F8** - Open the directory in VS Code.<br/>
**F9-F12** - Unassigned (configurable).<br/>

## How to Build

```powershell
.\build.ps1
```

If that doesn't work, you may need to enable PowerShell scripts first:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```
