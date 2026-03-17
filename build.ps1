$icon = $false
$dist = $false

foreach ($arg in $args) {
    if ($arg -eq "icon") {
        $icon = $true
    }
    if ($arg -eq "dist") {
        $dist = $true
    }
}

$commit    = git rev-parse --short HEAD
$buildTime = Get-Date -Format "dd.MM.yyyy HH:mm"
$version   = git describe --tags --abbrev=0
$dirty     = git status --porcelain

if (-not $version) { $version = "dev" }
if ($dirty) { $version += "-dirty" }

$ldflags = @(
    "-X 'main.Version=$version'"
    "-X 'main.GitCommit=$commit'"
    "-X 'main.BuildTime=$buildTime'"
) -join " "

if ($icon) {
    # go install github.com/akavel/rsrc@latest
    rsrc -ico .\assets\icon.ico
}
go build -ldflags $ldflags

if ($dist) {
    $distPath = ".\dist"
    if (Test-Path $distPath) {
        Remove-Item $distPath -Recurse
    }
    New-Item -Path $distPath -ItemType Directory
    Copy-Item .\mc.exe -Destination $distPath
    Get-ChildItem -Path ".\scripts" -Include "*.bat", "*.ps1" -Recurse | 
        Copy-Item -Destination $distPath
    $deps = "..\deps\deps.exe"
    if (Test-Path $deps) {
        Copy-Item $deps -Destination $distPath
    }
    7z a "$distPath\mc_$version.zip" "$distPath\*"
}