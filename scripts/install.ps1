# Sabdopalon one-click installer (Windows PowerShell).
#
# Usage (PowerShell):
#   irm https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/install.ps1 | iex
#
# Downloads the Windows bundle, extracts it to %USERPROFILE%\sabdopalon,
# adds it to the user PATH (persistent), and starts the setup wizard.
# Idempotent: re-running upgrades in place.
$ErrorActionPreference = "Stop"

$Repo = "danyakmallun9999/sabdopalon"
$Version = if ($env:SABDOPALON_VERSION) { $env:SABDOPALON_VERSION } else { "latest" }
$InstallDir = if ($env:SABDOPALON_DIR) { $env:SABDOPALON_DIR } else { Join-Path $HOME "sabdopalon" }

$Asset = "sabdopalon-windows-x86_64.exe.zip"
$Url = "https://github.com/$Repo/releases/$Version/download/$Asset"

Write-Host "Sabdopalon installer (Windows)" -ForegroundColor Cyan
Write-Host "Install folder: $InstallDir"

# --- download ---------------------------------------------------------------
$Tmp = Join-Path $env:TEMP "sabdopalon-install"
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
$Zip = Join-Path $Tmp $Asset

Write-Host "Downloading $Asset ..."
Invoke-WebRequest -Uri $Url -OutFile $Zip -UseBasicParsing

# --- extract ----------------------------------------------------------------
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Expand-Archive -Path $Zip -DestinationPath $InstallDir -Force

$Binary = Join-Path $InstallDir "sabdopalon.exe"
if (-not (Test-Path $Binary)) {
    Write-Error "sabdopalon.exe not found in the bundle — aborting."
    exit 1
}

# --- add to user PATH (persistent) -------------------------------------------
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = if ([string]::IsNullOrEmpty($UserPath)) { $InstallDir } else { "$UserPath;$InstallDir" }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    Write-Host "Added $InstallDir to your user PATH (new terminals will pick it up)." -ForegroundColor Yellow
}

Write-Host "Installed to $InstallDir" -ForegroundColor Green
Write-Host "Running the setup wizard (your first-run configuration)..."
& $Binary setup
