# Creates a temporary file with random name
$tempFile = New-TemporaryFile
Write-Host "Temp file: $tempFile"

# Get file info
$tempFile | Get-Item

# Clean up
$tempFile | Remove-Item -Force
