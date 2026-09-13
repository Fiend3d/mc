$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot
try {
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
    $version   = git describe --tags --abbrev=0 --always
    if ($LASTEXITCODE -ne 0) { throw "Could not determine version from Git" }
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

    $output = "mc.exe"
    if (Test-Path $output) {
        # Go doesn't rebuild if no source changes, which could leave outdated version flags
        # Force clean build to ensure accurate version and dirty state
        Remove-Item $output
    }

    go build -ldflags $ldflags -o $output
    if ($LASTEXITCODE -ne 0) { throw "Go build failed" }

    if ($dist) {
        $dependencies = @("deps", "koneko") | ForEach-Object {
            $directory = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\$_"))
            $script = Join-Path $directory "build.ps1"
            if (-not (Test-Path -LiteralPath $script -PathType Leaf)) {
                throw "Distribution builds require the sibling $_ checkout with build.ps1: $directory"
            }
            [pscustomobject]@{
                Name = $_
                Directory = $directory
                Script = $script
                Binary = Join-Path $directory "$_.exe"
            }
        }
        # Isolate dependency scripts from our variables and run in their own repositories.
        $buildHost = (Get-Process -Id $PID).Path
        foreach ($dependency in $dependencies) {
            Write-Host "Rebuilding $($dependency.Name)..."
            Push-Location -LiteralPath $dependency.Directory
            try {
                & $buildHost -NoProfile -File $dependency.Script
                if ($LASTEXITCODE -ne 0) {
                    throw "$($dependency.Name) build failed with exit code $LASTEXITCODE"
                }
                if (-not (Test-Path -LiteralPath $dependency.Binary -PathType Leaf)) {
                    throw "$($dependency.Name) build did not produce $($dependency.Binary)"
                }
            } finally {
                Pop-Location
            }
        }
        $distPath = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "dist"))
        New-Item -Path $distPath -ItemType Directory -Force | Out-Null
        $stagePath = Join-Path $distPath (".package-" + [guid]::NewGuid().ToString("N"))
        New-Item -Path $stagePath -ItemType Directory | Out-Null
        try {
            Copy-Item -LiteralPath $output -Destination $stagePath
            Get-ChildItem -Path ".\scripts" -Include "*.bat", "*.ps1" -Recurse |
                Copy-Item -Destination $stagePath
            foreach ($dependency in $dependencies) {
                Copy-Item -LiteralPath $dependency.Binary -Destination $stagePath
            }
            $archivePath = Join-Path $distPath "mc_$version.zip"
            Get-ChildItem -LiteralPath $stagePath | Compress-Archive -DestinationPath $archivePath -Force
            Write-Host "Created $archivePath"
            # dist is on PATH, so keep the unpacked tools there too: F2/F3 find deps and koneko through it.
            Get-ChildItem -LiteralPath $stagePath | Copy-Item -Destination $distPath -Force
            Write-Host "Updated $distPath"
        } finally {
            $resolvedStage = [System.IO.Path]::GetFullPath($stagePath)
            if ([System.IO.Path]::GetDirectoryName($resolvedStage) -ne $distPath -or
                -not [System.IO.Path]::GetFileName($resolvedStage).StartsWith(".package-")) {
                throw "Unexpected package staging path"
            }
            Remove-Item -LiteralPath $resolvedStage -Recurse
        }
    }

} finally { Pop-Location }
