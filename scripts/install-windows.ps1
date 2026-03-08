Param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\PromptPacker"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repo = "immazoni/promptpacker"
$baseUrl = "https://github.com/$repo/releases/latest/download"

switch ($env:PROCESSOR_ARCHITECTURE.ToLowerInvariant()) {
    "amd64" { $arch = "amd64" }
    "arm64" { $arch = "arm64" }
    default {
        Write-Error "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE). Supported: amd64, arm64."
        exit 1
    }
}

$asset = "promptpacker-windows-$arch.exe"
Write-Host "Detected architecture: $arch"
Write-Host "Selected release asset: $asset"

if (-not (Test-Path -LiteralPath $InstallDir)) {
    Write-Host "Creating install directory: $InstallDir"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$exePath = Join-Path $InstallDir "promptpacker.exe"

Write-Host "Downloading latest promptpacker release..."
$downloadUrl = "$baseUrl/$asset"

Invoke-WebRequest -Uri $downloadUrl -OutFile $exePath -UseBasicParsing

Write-Host "Installed promptpacker to $exePath"

# Ensure the install directory is on the user PATH
$currentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathEntries = @()
if ($currentUserPath) {
    $pathEntries = $currentUserPath.Split(";") | Where-Object { $_ -ne "" }
}

if ($pathEntries -notcontains $InstallDir) {
    $newUserPath = if ($currentUserPath) { "$currentUserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    Write-Host "Added '$InstallDir' to your user PATH."
    Write-Host "You may need to restart your terminal or sign out/in for PATH changes to take effect."
}
else {
    Write-Host "'$InstallDir' is already on your user PATH."
}

Write-Host "You can now run 'promptpacker' from any new PowerShell or Command Prompt window."

