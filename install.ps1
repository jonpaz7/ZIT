# ZIT Windows Installer
$ErrorActionPreference = "Stop"
Write-Host "=== Installing ZIT CLI & Tracker ===" -ForegroundColor Cyan

$installDir = Join-Path $env:LOCALAPPDATA "Programs\ZIT\bin"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$workspace = $PSScriptRoot

$zitSrc = Join-Path $workspace "zit.exe"
$trackerSrc = Join-Path $workspace "zit-tracker.exe"

if ((Test-Path $zitSrc) -and (Test-Path $trackerSrc)) {
    Write-Host "Copying prebuilt binaries to $installDir..." -ForegroundColor Yellow
    Copy-Item $zitSrc (Join-Path $installDir "zit.exe") -Force
    Copy-Item $trackerSrc (Join-Path $installDir "zit-tracker.exe") -Force
} else {
    Write-Host "Compiling binaries with Go..." -ForegroundColor Yellow
    & go build -o (Join-Path $installDir "zit.exe") (Join-Path $workspace "cmd\zit")
    & go build -o (Join-Path $installDir "zit-tracker.exe") (Join-Path $workspace "cmd\zit-tracker")
}

# Add to User PATH permanently
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    $newPath = "$userPath;$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$env:Path;$installDir"
    Write-Host "Added $installDir to User PATH." -ForegroundColor Green
} else {
    Write-Host "$installDir is already in User PATH." -ForegroundColor Green
}

Write-Host "`nInstallation Complete!" -ForegroundColor Green
Write-Host "You can now open any new terminal window and simply type:" -ForegroundColor Cyan
Write-Host "  zit --help" -ForegroundColor Yellow
Write-Host "  zit-tracker --help" -ForegroundColor Yellow
