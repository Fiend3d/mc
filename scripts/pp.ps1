# Paste picture: save a clipboard image in the current directory without overwriting.
# Usage: pp [name[.png|.jpg|.jpeg|.bmp]]
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Name = 'image'
)

try {
    if ($PWD.Provider.Name -ne 'FileSystem') {
        throw 'pp requires a filesystem directory.'
    }
    if ([string]::IsNullOrWhiteSpace($Name) -or
        $Name.IndexOfAny([System.IO.Path]::GetInvalidFileNameChars()) -ge 0 -or
        $Name.EndsWith('.') -or $Name.EndsWith(' ') -or
        $Name -match '^(?i:CON|PRN|AUX|NUL|COM[1-9\u00b9\u00b2\u00b3]|LPT[1-9\u00b9\u00b2\u00b3])(?:\.|$)') {
        throw 'Use a valid Windows filename, not a directory path.'
    }

    $extension = [System.IO.Path]::GetExtension($Name)
    if (-not $extension) {
        $extension = '.png'
        $Name += $extension
    }
    $formatName = switch ($extension.ToLowerInvariant()) {
        '.png' { 'Png' }
        '.jpg' { 'Jpeg' }
        '.jpeg' { 'Jpeg' }
        '.bmp' { 'Bmp' }
        default { throw "Unsupported image extension '$extension'. Use .png, .jpg, .jpeg or .bmp." }
    }

    # Windows Forms clipboard APIs require STA. Keep shell state and arguments intact.
    if ([System.Threading.Thread]::CurrentThread.GetApartmentState() -ne 'STA') {
        $windowsPowerShell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
        Push-Location -LiteralPath $PWD.Path
        try {
            & $windowsPowerShell -NoProfile -STA -File $PSCommandPath -Name $Name
        } finally {
            Pop-Location
        }
        return
    }

    Add-Type -AssemblyName System.Windows.Forms -ErrorAction Stop
    Add-Type -AssemblyName System.Drawing -ErrorAction Stop
    $image = [System.Windows.Forms.Clipboard]::GetImage()
    if ($null -eq $image) {
        throw 'The clipboard does not contain an image.'
    }

    $stream = $null
    $createdPath = $null
    $saved = $false
    try {
        $stem = [System.IO.Path]::GetFileNameWithoutExtension($Name)
        $suffix = 0
        while ($null -eq $stream) {
            $candidate = if ($suffix -eq 0) { $Name } else { "$stem$suffix$extension" }
            $path = Join-Path $PWD.Path $candidate
            try {
                $stream = [System.IO.File]::Open($path, [System.IO.FileMode]::CreateNew,
                    [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
                $createdPath = $path
            } catch [System.IO.IOException], [System.UnauthorizedAccessException] {
                if ([System.IO.File]::Exists($path) -or [System.IO.Directory]::Exists($path)) {
                    $suffix++
                } else {
                    throw
                }
            }
        }
        $format = [System.Drawing.Imaging.ImageFormat]::$formatName
        $image.Save($stream, $format)
        $stream.Flush()
        $saved = $true
    } finally {
        if ($null -ne $stream) { $stream.Dispose() }
        $image.Dispose()
        if (-not $saved -and $null -ne $createdPath) {
            Remove-Item -LiteralPath $createdPath -ErrorAction Stop
        }
    }
    Write-Output $createdPath
} catch {
    Write-Error -Message "pp: $($_.Exception.Message)" -ErrorAction Continue
}
