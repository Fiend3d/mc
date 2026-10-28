# Paste picture or text: save a clipboard image or text file in the current directory without overwriting.
# Usage: pp [name[.png|.jpg|.jpeg|.bmp|.txt]]
# What is not an image is text: without an explicit .txt name, an image is saved as an
# image and anything else on the clipboard is saved as text (.txt).
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Name = 'image'
)

function New-UniqueFile {
    param(
        [string]$FileName,
        [string]$Directory
    )
    $stem = [System.IO.Path]::GetFileNameWithoutExtension($FileName)
    $extension = [System.IO.Path]::GetExtension($FileName)
    $suffix = 0
    while ($true) {
        $candidate = if ($suffix -eq 0) { $FileName } else { "$stem$suffix$extension" }
        $path = Join-Path $Directory $candidate
        try {
            $stream = [System.IO.File]::Open($path, [System.IO.FileMode]::CreateNew,
                [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
            return [pscustomobject]@{ Stream = $stream; Path = $path }
        } catch [System.IO.IOException], [System.UnauthorizedAccessException] {
            if ([System.IO.File]::Exists($path) -or [System.IO.Directory]::Exists($path)) {
                $suffix++
            } else {
                throw
            }
        }
    }
}

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
        '.txt' { 'Text' }
        default { throw "Unsupported extension '$extension'. Use .png, .jpg, .jpeg, .bmp or .txt." }
    }
    # An explicit .txt name asks for text; otherwise the clipboard decides and text falls
    # back to .txt so a text clipboard is never rejected.
    $isText = $formatName -eq 'Text'

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
    $image = $null
    $text = $null
    if ($isText) {
        $text = [System.Windows.Forms.Clipboard]::GetText()
        if ([string]::IsNullOrEmpty($text)) {
            throw 'The clipboard does not contain text.'
        }
    } else {
        Add-Type -AssemblyName System.Drawing -ErrorAction Stop
        $image = [System.Windows.Forms.Clipboard]::GetImage()
        if ($null -eq $image) {
            $text = [System.Windows.Forms.Clipboard]::GetText()
            if ([string]::IsNullOrEmpty($text)) {
                throw 'The clipboard contains neither an image nor text.'
            }
            $isText = $true
            $Name = [System.IO.Path]::GetFileNameWithoutExtension($Name) + '.txt'
        }
    }

    $created = $null
    $saved = $false
    try {
        $created = New-UniqueFile -FileName $Name -Directory $PWD.Path
        if ($isText) {
            $writer = New-Object System.IO.StreamWriter($created.Stream, (New-Object System.Text.UTF8Encoding($false)))
            $writer.Write($text)
            $writer.Flush()
            $writer.Dispose()
            $created.Stream = $null
        } else {
            $format = [System.Drawing.Imaging.ImageFormat]::$formatName
            $image.Save($created.Stream, $format)
            $created.Stream.Flush()
        }
        $saved = $true
    } finally {
        if ($null -ne $created -and $null -ne $created.Stream) { $created.Stream.Dispose() }
        if ($null -ne $image) { $image.Dispose() }
        if (-not $saved -and $null -ne $created -and $null -ne $created.Path) {
            Remove-Item -LiteralPath $created.Path -ErrorAction Stop
        }
    }
    Write-Output $created.Path
} catch {
    Write-Error -Message "pp: $($_.Exception.Message)" -ErrorAction Continue
}
