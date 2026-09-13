# Standalone tests: powershell -NoProfile -STA -File tests/pp.Tests.ps1
# Use synthetic clipboard results; never read or modify the user's clipboard.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$sourcePath = Join-Path (Split-Path $PSScriptRoot -Parent) 'scripts\pp.ps1'
$source = Get-Content -LiteralPath $sourcePath -Raw
$parseErrors = $null
[System.Management.Automation.Language.Parser]::ParseFile($sourcePath, [ref]$null, [ref]$parseErrors) | Out-Null
if ($parseErrors.Count) { throw ($parseErrors | Out-String) }

$testDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ('mc-pp-tests-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testDirectory | Out-Null
$mockImage = '$image = New-Object System.Drawing.Bitmap 3, 2'
$script = [scriptblock]::Create($source.Replace('$image = [System.Windows.Forms.Clipboard]::GetImage()', $mockImage))
function Assert-Image($Path, $Format) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Missing image: $Path" }
    $image = [System.Drawing.Image]::FromFile((Join-Path $PWD.Path $Path))
    try {
        if ($image.Width -ne 3 -or $image.Height -ne 2 -or $image.RawFormat.Guid -ne $Format.Guid) {
            throw "Wrong image contents or format: $Path"
        }
    } finally { $image.Dispose() }
}
Push-Location -LiteralPath $testDirectory
try {
    & $script | Out-Null
    Assert-Image 'image.png' ([System.Drawing.Imaging.ImageFormat]::Png)
    $original = [System.IO.File]::ReadAllBytes((Join-Path $testDirectory 'image.png'))
    New-Item -ItemType Directory -Path 'image1.png' | Out-Null
    & $script | Out-Null
    Assert-Image 'image2.png' ([System.Drawing.Imaging.ImageFormat]::Png)
    if ([Convert]::ToBase64String($original) -ne [Convert]::ToBase64String(
        [System.IO.File]::ReadAllBytes((Join-Path $testDirectory 'image.png')))) { throw 'Existing image overwritten' }
    foreach ($name in @('my screenshot', 'photo.jpg', 'photo.jpeg', 'drawing.BMP', 'custom.PNG')) {
        & $script $name | Out-Null
        $path = if ([System.IO.Path]::GetExtension($name)) { $name } else { "$name.png" }
        $format = switch ([System.IO.Path]::GetExtension($path).ToLowerInvariant()) {
            '.png' { [System.Drawing.Imaging.ImageFormat]::Png }
            '.jpg' { [System.Drawing.Imaging.ImageFormat]::Jpeg }
            '.jpeg' { [System.Drawing.Imaging.ImageFormat]::Jpeg }
            '.bmp' { [System.Drawing.Imaging.ImageFormat]::Bmp }
        }
        Assert-Image $path $format
    }
    & $script 'photo.jpg' | Out-Null
    Assert-Image 'photo1.jpg' ([System.Drawing.Imaging.ImageFormat]::Jpeg)
    $before = @(Get-ChildItem).Count
    foreach ($name in @('', ' ', 'CON', 'NUL.png', 'bad.', 'bad ', '../outside.png', 'bad:name', 'no.gif')) {
        $errors = @()
        & $script -Name $name -ErrorVariable errors 2>$null
        if (-not $errors.Count) { throw "Expected invalid-name error: $name" }
    }
    foreach ($replacement in @('$image = $null', 'throw "Clipboard unavailable"')) {
        $noImage = [scriptblock]::Create($source.Replace('$image = [System.Windows.Forms.Clipboard]::GetImage()', $replacement))
        $errors = @()
        & $noImage -ErrorVariable errors 2>$null
        if (-not $errors.Count) { throw 'Expected clipboard error' }
    }
    $failureImage = '$image = New-Object PSObject; $image | Add-Member ScriptMethod Save { throw "Save failed" }; $image | Add-Member ScriptMethod Dispose {}'
    $failure = [scriptblock]::Create($source.Replace('$image = [System.Windows.Forms.Clipboard]::GetImage()', $failureImage))
    $errors = @()
    & $failure 'failed' -ErrorVariable errors 2>$null
    if (-not $errors.Count -or (Test-Path 'failed.png')) { throw 'Failed save left partial output' }
    if (@(Get-ChildItem).Count -ne $before) { throw 'Error cases created files' }

    # Run the real MTA delegation path, but with a synthetic image in the child.
    $delegatePath = Join-Path $testDirectory 'delegate.ps1'
    [System.IO.File]::WriteAllText($delegatePath, $source.Replace('$image = [System.Windows.Forms.Clipboard]::GetImage()', $mockImage))
    $hostPath = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    & $hostPath -NoProfile -MTA -File $delegatePath -Name 'delegated image.png' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'MTA delegation failed' }
    Assert-Image 'delegated image.png' ([System.Drawing.Imaging.ImageFormat]::Png)
    Write-Output 'pp tests passed (formats, collisions, validation, clipboard failures, cleanup, MTA delegation).'
} finally {
    Pop-Location
    $resolvedTestDirectory = [System.IO.Path]::GetFullPath($testDirectory)
    if ([System.IO.Path]::GetDirectoryName($resolvedTestDirectory) -ne [System.IO.Path]::GetTempPath().TrimEnd('\') -or
        -not [System.IO.Path]::GetFileName($resolvedTestDirectory).StartsWith('mc-pp-tests-')) {
        throw 'Unexpected test cleanup path'
    }
    Remove-Item -LiteralPath $resolvedTestDirectory -Recurse -Force
}
