# How to edit PowerShell's config

I like using Helix for this. You should use whatever you prefer.

```powershell
hx $profile
````

Paste this there:

```powershell
function m {
	$tmp = (New-TemporaryFile).FullName                                 # create a temp file
	mc.exe $args -o -tf="$tmp"                                          # launch mc with output enabled
	$cwd = Get-Content -Path $tmp -Encoding UTF8                        # grab the path
	if ($cwd -ne $null -and `
		$cwd -ne $PWD.Path -and `                                       # check if the path is ok
		(Test-Path -LiteralPath $cwd -PathType Container)) {            
		Set-Location -LiteralPath (Resolve-Path -LiteralPath $cwd).Path # cd!
	}
	Remove-Item -Path $tmp                                              # remove the file
}
```

And it doesn't work `¯\_(ツ)_/¯` You need to enable powershell for some reason:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

Now you can just type `m` in the terminal and it will launch **mc**. When you quit, it will `cd` to the selected directory.

# t.bat
A useful script that launches Windows Terminal in the current working directory. By default, Windows Terminal doesn't do this. Simply typing `t` is convenient and easy to remember.

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
