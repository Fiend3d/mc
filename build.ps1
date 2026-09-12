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
    $version   = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
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
        $distPath = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "dist"))
        New-Item -Path $distPath -ItemType Directory -Force | Out-Null
        $stagePath = Join-Path $distPath (".package-" + [guid]::NewGuid().ToString("N"))
        New-Item -Path $stagePath -ItemType Directory | Out-Null
        try {
            Copy-Item -LiteralPath $output -Destination $stagePath
            Get-ChildItem -Path ".\scripts" -Include "*.bat", "*.ps1" -Recurse |
                Copy-Item -Destination $stagePath
            foreach ($dependency in @("..\deps\deps.exe", "..\koneko\koneko.exe")) {
                if (Test-Path -LiteralPath $dependency) {
                    Copy-Item -LiteralPath $dependency -Destination $stagePath
                }
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
