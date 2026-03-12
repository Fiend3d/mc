# Modal Commander

Modal Commander (mc) is a TUI file manager for Windows (though it might be ported to other platforms in the future). It's heavily inspired by Yazi, Helix, and Total Commander. 

## Demo

![RECLibboard](assets/demo/demo01.png)
![RECLibboard](assets/demo/demo02.png)
![RECLibboard](assets/demo/demo03.png)
![RECLibboard](assets/demo/demo.gif)

## How to install

Currently, `mc` uses [bat](https://github.com/sharkdp/bat) for viewing files, and [helix](https://github.com/helix-editor/helix) for editing. `Bat` requires `less` to work and I strongly recommend using the one that comes with `git`. 

I also recommend using [Windows Terminal](https://github.com/microsoft/terminal) because it's the only terminal emulator, that I found, that makes the mouse work properly on Windows 10. It also looks kinda good if you install https://www.nerdfonts.com/font-downloads specifically `JetBrainsMonoNL Nerd Font`. 

Here is how you can configure your powershell to `cd` to the directory when you exit: https://github.com/Fiend3d/mc/tree/master/scripts you can also find there `t.bat` that makes launching Windows Terminal a lot easier, because by default it doesn't open the current directory and typing just `t` is very easy. 

## How to use

Pressing F1 shows documentation that can be filtered by pressing F. I tried to make `mc` as intuitive as possible, and for the most part, everything is accessible with the mouse.

### Normal Mode
The main mode of the program from witch most other modes can be entered.

- **q** - Quit, returning the current directory.
- **Q** - Quit without returning anything.
- **space** - Select.
- **y** - Copy selected items. It's just normal Window's copied filepaths, so they can pasted in Explorer to.
- **x** - Cut. 
- **d** - Delete PERMANENTLY. It will prompt for confirmation. 
- **r** - Rename. Selecting multiple items launched editor to edit their names. 
- **p** - Paste.
- **u** - Undo.
- **U** - Redo. 
- **t** - Copy current tab.
- **Ctrl+w** - Close current tab.
- **T** - Restore closed tab.
- **Ctrl+t, Ctrl+n** - Open selected dir in a new tab.
- **]** - Next tab. 
- **[** - Next tab. 
- **1-0** - Select tabs 1 to 10 (0 is tab 10).
- **Ctrl+b**  - Go back in history.
- **Ctrl+f** - Go forward in history.
- **F5** - Update.

- **B** - Bookmark the directory.
- **b** - Browse bookmarks.

### Jump Mode
Can be entered by pressing `tab` in the normal mode. Jump mode is to mimic Explorer's behavior when pressing buttons to jump to the needed item. 

### Visual Mode
Can be entered by pressing `v`. It's for range selecting. 

### Filter Mode
Entered by pressing `f` in the normal mode. Current tab can be filtered. 

### Copy Mode
Entered by pressing `c`. Capital letters convert slashes from `\` to `/`. "Copy the filenames as arguments" means it can be used as arguments for terminal commands (if path has spaces it will be quoted).

**c/C** - Copy the file path/Forward.
**d/D** - Copy the directory/Forward.
**f** - Copy the filename.
**n** - Copy the filename without extension.
**a/A** - Copy the file paths as arguments/Forward.
**s** - Copy the filenames as arguments.
**q/Q** - Copy the file paths as array/Forward.
**w** - Copy the filenames as array.

### Sort Mode
Entered by pressing `,`. Capital letters sort in reverse.

**m/M** - Sort by modified time.
**a/A** - Sort alphabetically.
**n/N** - Sort normally.
**e/E** - Sort by extension.
**s/S** - Sort by size.
**r** - Sort randomly.

### Create Mode
Entered by pressing `a`. If your name ends with a slash it's a directory.

### Message Mode
Entered by pressing `\``. The message history can be viewed here.

### Search Mode
Press `s` to enter search mode. Use `tab` to cycle through focus. By default, search respects `.gitignore`, but you can disable this by pressing `F1` while in search mode.

Press `F3` on a line to open it with `bat`; it will jump directly to that line. Press `n` to jump to the next match (or `N` to go backwards). Press `h` to hide all matched lines.

`F5` or `Enter` while focussing a text input - start searching.

### Shell Mode
Press `:` to enter shell mode. You can hide and unhide TUI by pressing `Ctrl+h` to see the result of a command. `#sl` - is a macro that is converted to a list of selected items for a command. 
**Ctrl+b** - Back in history. 
**Ctrl+f** - Forward in history.

### Go Mode
Go mode is just a menu.

**g** - Enter Path mode.
**t** - Browse tabs.
**c** - Open the settings directory. You can also find and delete bookmarks there, for example.
**C** - Save settings to config.toml for editing.
**s** - Calculate size for the selected directories.

### Path Mode
Press `gg` to enter path mode.

**ctrl+u** - Clear all left of cursor.
**ctrl+k** - Clear all right of cursor.
**ctrl+w** - Delete a word.
**tab** - Autocomplete.
**up/down** - Next/previous autocomplete.
**ctrl+e** - Expand environment variables.
**ctrl+n** - Open the path in a new tab.

### Tools
`F3-F4`, `F6-F12` - tools. They can be configured in `config.toml`. The default config can be saved by pressing `gC` (`g` and then `C`, and then `gc` to find it to edit).
**F3** - Viewer. 
**F4** - Editor.
**F6** - Open the directory in Explorer.
**F7** - Open the files in VS Code.
**F8** - Open the directory in VS Code.
**F9-F12** - Unassigned (configurable).

## How to Build

Because I fixed a few issues in [github.com/charmbracelet/bubbles](https://github.com/charmbracelet/bubbles), you'll need to get my fork first.

You can do this using:

```powershell
get_forks
```

If that doesn't work, you may need to enable PowerShell scripts first:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

Then simply run:


```powershell
build
```
