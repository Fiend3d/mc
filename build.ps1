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

go build -ldflags $ldflags
