# Zip Bomb Detector - Analyzes archives and safely extracts potential zip bombs
# Usage: zz archive1.zip archive2.7z ...
# Requires: 7-Zip in PATH

function GetArchiveContent {
    param(
        [Parameter(Mandatory=$true)]
        [string]$ArchivePath
    )
    
    if (-not (Test-Path $ArchivePath)) {
        Write-Error "Archive not found: $ArchivePath"
        return
    }
    
    $output = 7z l -ba $ArchivePath
    
    $output | ForEach-Object {
        $parts = $_ -split '\s+'
        if ($parts.Count -ge 6) {
            $date = $parts[0]
            $time = $parts[1]
            $attr = $parts[2]
            $size = $parts[3]
            $compressed = $parts[4]
            $name = $parts[5..($parts.Count-1)] -join " "
            
            [PSCustomObject]@{
                Date = $date
                Time = $time
                Attr = $attr
                Size = $size
                Compressed = $compressed
                Name = $name
            }
        }
    }
}

foreach ($arg in $args) {
    Write-Host "Processing " -NoNewline
    Write-Host "$arg" -NoNewline -ForegroundColor Green
    Write-Host "..."
    $content = GetArchiveContent -ArchivePath $arg
    Write-Host "Number of files: $($content.Count)" -ForegroundColor Blue
    if ($content.Count -eq 1) {
        Write-Host "$arg is not a zip bomb" -ForegroundColor Green
        7z x $arg
    } else {
        $bomb = $true
        if ($content[0].Attr -eq "D....") {
            $bomb = $false
            foreach ($item in $content) {
                if (-not $item.Name.StartsWith(($content[0].Name))) {
                    $bomb = $true
                    break
                }
            }
        }
        if ($bomb) {
            Write-Host "$arg is a zip bomb" -ForegroundColor Red
            $file = Get-Item $arg
            $outDir = [System.IO.Path]::Combine($file.Directory, $file.BaseName)
            7z x $arg -o"$outDir"
        } else {
            Write-Host "$arg is not a zip bomb" -ForegroundColor Green
            7z x $arg
        }
    }
}