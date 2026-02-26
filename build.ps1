$commit = git rev-parse --short HEAD
$buildTime = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
$version = git describe --tags --always --dirty 2>$null
if (-not $version) { $version = "dev" }

$ldflags = @(
    "-X main.Version=$version",
    "-X main.GitCommit=$commit",
    "-X main.BuildTime=$buildTime"
) -join " "

go build -ldflags "$ldflags"
