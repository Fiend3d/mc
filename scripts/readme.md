# PowerShell wrapper: cd on exit

Add the directory containing `mc.exe` to your user `PATH`, then open a new PowerShell session. Create and open your profile:

```powershell
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $PROFILE) | Out-Null
if (-not (Test-Path -LiteralPath $PROFILE)) {
    New-Item -ItemType File -Path $PROFILE | Out-Null
}
notepad $PROFILE
```

Paste this function into the profile and save it:

```powershell
function m {
    $tmp = (New-TemporaryFile).FullName
    try {
        mc.exe -o -tf="$tmp" $args
        $cwd = Get-Content -LiteralPath $tmp -Encoding UTF8
        if ($null -ne $cwd -and $cwd -ne $PWD.Path -and
            (Test-Path -LiteralPath $cwd -PathType Container)) {
            Set-Location -LiteralPath (Resolve-Path -LiteralPath $cwd).Path
        }
    } finally {
        Remove-Item -LiteralPath $tmp
    }
}
```

Reload your profile with `. $PROFILE`, or open a new PowerShell session. If PowerShell blocks scripts, run:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
. $PROFILE
```

Run `m` to launch mc, or pass directories with `m C:\projects C:\downloads`. Browse, focus the pane whose directory you want, and press `q` to change the calling shell to that pane's current directory. `Q` (Shift+Q) quits without changing directory; an empty focused pane also returns no directory. Running `mc.exe` directly cannot change the parent shell's directory.

# t.bat

`t.bat` runs `wt -d .` to open Windows Terminal in the calling shell's current directory, instead of relying on the terminal profile's default starting directory. Windows Terminal must be installed and `wt` available on `PATH`.

Release archives include `t.bat`. Add its containing directory to your user `PATH` and open a new shell to invoke it as `t`. Without PATH setup, use `& C:\path\to\t.bat`.

```powershell
m  # Browse to a directory, then press q to cd there
t  # Open Windows Terminal in that directory
```

# zz
`zz` is a zip bomb detector. It safely unzips archives using `7z` (https://www.7-zip.org/download.html).

There's no need to check the contents of an archive before unzipping.

# pp — paste picture

`pp.ps1` saves an image from the Windows clipboard into the current directory.
Add the directory containing `pp.ps1` to your `PATH` (release archives include it),
then run it from PowerShell:

```powershell
pp                 # image.png, image1.png, image2.png, ...
pp screenshot      # screenshot.png
pp "my screenshot" # my screenshot.png
pp photo.jpg       # JPEG instead of PNG
```

Without PATH setup, use `& C:\path\to\pp.ps1` instead of `pp`.
PNG is the default; `.png`, `.jpg`, `.jpeg` and `.bmp` extensions choose the format.
Existing files and directories are never overwritten: `pp photo.jpg` tries
`photo1.jpg`, `photo2.jpg`, etc. if needed, choosing the first available name.
Supply a filename only, not a path. Unsupported extensions and invalid Windows
filenames are rejected.

The saved file's full path is printed. If the clipboard contains only text,
copied file paths or no image, nothing is created and a clear error is shown.
Clipboard and write failures are also reported without closing your shell.
Works on Windows PowerShell 5.1 and PowerShell 7 on Windows; no external tools
are needed. Non-STA hosts delegate clipboard access to Windows PowerShell.
