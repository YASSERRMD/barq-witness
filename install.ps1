# barq-witness installer for Windows (PowerShell)
# Usage: iwr -useb https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.ps1 | iex
# Or with a specific version: $env:BARQ_VERSION="v1.1.1"; iwr -useb ... | iex

param(
    [string]$Version = $env:BARQ_VERSION,
    [string]$InstallDir = $env:BARQ_INSTALL_DIR
)

$ErrorActionPreference = "Stop"
$Repo = "yasserrmd/barq-witness"
$Binary = "barq-witness"

function Write-Info  { param($msg) Write-Host "[barq-witness] $msg" -ForegroundColor Green }
function Write-Warn  { param($msg) Write-Host "[barq-witness] WARNING: $msg" -ForegroundColor Yellow }
function Write-Err   { param($msg) Write-Host "[barq-witness] ERROR: $msg" -ForegroundColor Red; exit 1 }

# Detect architecture
$arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else {
    Write-Err "32-bit Windows is not supported. Please check https://github.com/$Repo/releases for available builds."
}

Write-Info "Detected: windows/$arch"

# Resolve latest version if not set
if (-not $Version) {
    Write-Info "Fetching latest release version..."
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        $Version = $release.tag_name
    } catch {
        Write-Err "Could not fetch latest version. Set `$env:BARQ_VERSION manually. Error: $_"
    }
}

Write-Info "Installing barq-witness $Version..."

# Resolve install directory
if (-not $InstallDir) {
    # Try user's local bin first (no admin required), fall back to asking
    $localBin = "$env:LOCALAPPDATA\barq-witness\bin"
    $InstallDir = $localBin
}

# Create install dir if it does not exist
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Write-Info "Created install directory: $InstallDir"
}

# Construct download URL
$AssetName = "$Binary-windows-$arch.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$AssetName"
$OutPath = Join-Path $InstallDir "$Binary.exe"

Write-Info "Downloading $DownloadUrl"
try {
    # Use TLS 1.2+
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $OutPath -UseBasicParsing
} catch {
    Write-Err "Download failed. Check that $Version exists at https://github.com/$Repo/releases. Error: $_"
}

Write-Info "Downloaded to $OutPath"

# Add to PATH for this session
$env:PATH = "$InstallDir;$env:PATH"

# Add to user PATH permanently (no admin required)
$userPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$InstallDir*") {
    [System.Environment]::SetEnvironmentVariable("PATH", "$InstallDir;$userPath", "User")
    Write-Info "Added $InstallDir to your user PATH."
    Write-Warn "Restart your terminal (or run: `$env:PATH = `"$InstallDir;`$env:PATH`") for PATH changes to take effect in new sessions."
}

# Verify
try {
    $installedVersion = & "$OutPath" version 2>&1
    Write-Info "Installation complete: $installedVersion"
} catch {
    Write-Info "Installed to $OutPath"
}

Write-Host ""
Write-Host "Quick start:" -ForegroundColor Cyan
Write-Host "  cd your-project"
Write-Host "  barq-witness init"
Write-Host "  barq-witness report"
Write-Host ""
Write-Host "Docs: https://github.com/$Repo" -ForegroundColor Cyan
Write-Host ""
