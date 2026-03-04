$commit    = git rev-parse --short HEAD
$buildTime = Get-Date -Format "dd.MM.yyyy HH:mm"
# $version   = git describe --tags --always --dirty 2>$null
$version   = git describe --tags --abbrev=0
if (-not $version) { $version = "dev" }

$ldflags = @(
    "-X 'main.Version=$version'"
    "-X 'main.GitCommit=$commit'"
    "-X 'main.BuildTime=$buildTime'"
) -join " "

go build -ldflags $ldflags
