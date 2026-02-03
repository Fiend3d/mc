# edit powershell's config

```powershell
hx $profile
````

Paste this there:

```powershell
function m {
	$tmp = (New-TemporaryFile).FullName                                              # create a temp file
	mc.exe $args -o -tf="$tmp"                                                       # launch mc with output enabled
	$cwd = Get-Content -Path $tmp -Encoding UTF8                                     # grab the path
	if ($cwd -ne $PWD.Path -and (Test-Path -LiteralPath $cwd -PathType Container)) { # check if the path is ok
		Set-Location -LiteralPath (Resolve-Path -LiteralPath $cwd).Path              # cd!
	}
	Remove-Item -Path $tmp                                                           # remove the file
}  
```

And it doesn't work! You need to enable powershell for some reason:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```
