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
