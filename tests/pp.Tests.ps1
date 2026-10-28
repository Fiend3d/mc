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
$mockNoImage = '$image = $null'
$mockNoText = '$text = $null'
$mockText = '$text = "line one" + [Environment]::NewLine + "line two"'
$expectedText = "line one" + [Environment]::NewLine + "line two"
$imageLine = '$image = [System.Windows.Forms.Clipboard]::GetImage()'
$textLine = '$text = [System.Windows.Forms.Clipboard]::GetText()'
function Get-PatchedSource([string]$ImageMock, [string]$TextMock) {
    return $source.Replace($imageLine, $ImageMock).Replace($textLine, $TextMock)
}
function New-Script([string]$ImageMock, [string]$TextMock) {
    return [scriptblock]::Create((Get-PatchedSource $ImageMock $TextMock))
}
$script = New-Script $mockImage $mockNoText
function Assert-Image($Path, $Format) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Missing image: $Path" }
    $image = [System.Drawing.Image]::FromFile((Join-Path $PWD.Path $Path))
    try {
        if ($image.Width -ne 3 -or $image.Height -ne 2 -or $image.RawFormat.Guid -ne $Format.Guid) {
            throw "Wrong image contents or format: $Path"
        }
    } finally { $image.Dispose() }
}
function Assert-Text($Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Missing text file: $Path" }
    $bytes = [System.IO.File]::ReadAllBytes((Join-Path $PWD.Path $Path))
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        throw "Text file has a BOM: $Path"
    }
    if ([System.Text.Encoding]::UTF8.GetString($bytes) -ne $expectedText) { throw "Wrong text contents: $Path" }
}
function Invoke-ExpectedError([scriptblock]$Script, [string]$Label, [string[]]$Arguments = @()) {
    $errors = @()
    & $Script @Arguments -ErrorVariable errors 2>$null
    if (-not $errors.Count) { throw "Expected error: $Label" }
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

    # Text clipboard: any non-image extension is a text file, with collisions.
    $textScript = New-Script $mockImage $mockText
    & $textScript 'note.txt' | Out-Null
    Assert-Text 'note.txt'
    & $textScript 'note.txt' | Out-Null
    Assert-Text 'note1.txt'
    New-Item -ItemType Directory -Path 'blocked.txt' | Out-Null
    & $textScript 'blocked.txt' | Out-Null
    Assert-Text 'blocked1.txt'
    foreach ($name in @('NOTES.TXT', 'main.rs', 'notes.md', 'data.json', 'config.toml', 'script.ps1', 'UPPER.RS', 'no.gif')) {
        & $textScript $name | Out-Null
        $path = if ([System.IO.Path]::GetExtension($name)) { $name } else { "$name.txt" }
        Assert-Text $path
    }

    # What is not an image is text: a text clipboard saves .txt even for an image name.
    $textOnly = New-Script $mockNoImage $mockText
    & $textOnly | Out-Null
    Assert-Text 'image.txt'
    & $textOnly 'photo.jpg' | Out-Null
    Assert-Text 'photo.txt'
    & $textOnly 'clip.BMP' | Out-Null
    Assert-Text 'clip.txt'
    & $textOnly 'CHANGELOG' | Out-Null
    Assert-Text 'CHANGELOG.txt'

    $before = @(Get-ChildItem).Count
    foreach ($name in @('', ' ', 'CON', 'NUL.png', 'bad.', 'bad ', '../outside.png', 'bad:name')) {
        Invoke-ExpectedError $script "invalid name $name" @($name)
    }
    foreach ($replacement in @($mockNoImage, 'throw "Clipboard unavailable"')) {
        Invoke-ExpectedError (New-Script $replacement $mockNoText) 'missing image'
    }
    # An explicit .txt name still requires clipboard text.
    Invoke-ExpectedError (New-Script $mockImage $mockNoText) 'explicit .txt without text' @('note.txt')
    Invoke-ExpectedError (New-Script $mockNoImage 'throw "Clipboard unavailable"') 'text clipboard failure'
    $failureImage = '$image = New-Object PSObject; $image | Add-Member ScriptMethod Save { throw "Save failed" }; $image | Add-Member ScriptMethod Dispose {}'
    $errors = @()
    & (New-Script $failureImage $mockNoText) 'failed' -ErrorVariable errors 2>$null
    if (-not $errors.Count -or (Test-Path 'failed.png')) { throw 'Failed save left partial output' }
    if (@(Get-ChildItem).Count -ne $before) { throw 'Error cases created files' }

    # Run the real MTA delegation path, but with synthetic clipboard results in the child.
    $hostPath = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $delegatePath = Join-Path $testDirectory 'delegate.ps1'
    [System.IO.File]::WriteAllText($delegatePath, (Get-PatchedSource $mockImage $mockNoText))
    & $hostPath -NoProfile -MTA -File $delegatePath -Name 'delegated image.png' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'MTA delegation failed' }
    Assert-Image 'delegated image.png' ([System.Drawing.Imaging.ImageFormat]::Png)
    [System.IO.File]::WriteAllText($delegatePath, (Get-PatchedSource $mockNoImage $mockText))
    & $hostPath -NoProfile -MTA -File $delegatePath -Name 'delegated note.jpg' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'MTA text delegation failed' }
    Assert-Text 'delegated note.txt'
    Write-Output 'pp tests passed (image formats, text fallback, collisions, validation, clipboard failures, cleanup, MTA delegation).'
} finally {
    Pop-Location
    $resolvedTestDirectory = [System.IO.Path]::GetFullPath($testDirectory)
    if ([System.IO.Path]::GetDirectoryName($resolvedTestDirectory) -ne [System.IO.Path]::GetTempPath().TrimEnd('\') -or
        -not [System.IO.Path]::GetFileName($resolvedTestDirectory).StartsWith('mc-pp-tests-')) {
        throw 'Unexpected test cleanup path'
    }
    Remove-Item -LiteralPath $resolvedTestDirectory -Recurse -Force
}
