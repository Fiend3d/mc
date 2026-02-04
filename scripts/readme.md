# edit powershell's config

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
