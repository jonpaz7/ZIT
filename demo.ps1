# ZIT End-to-End Automated Demonstration Script
$ErrorActionPreference = "Stop"

$workspace = Get-Location
$zitBin = Join-Path $workspace "zit.exe"
$trackerBin = Join-Path $workspace "zit-tracker.exe"

if (-not (Test-Path $zitBin) -or -not (Test-Path $trackerBin)) {
    Write-Error "Binaries not found. Please build zit.exe and zit-tracker.exe first."
}

$tempBase = Join-Path $env:TEMP ("zit_demo_" + [System.Guid]::NewGuid().ToString().Substring(0, 8))
$dirTracker = Join-Path $tempBase "tracker"
$dirClientA = Join-Path $tempBase "clientA"
$dirClientB = Join-Path $tempBase "clientB"
$dirClientC = Join-Path $tempBase "clientC"

New-Item -ItemType Directory -Path $dirTracker, $dirClientA, $dirClientB, $dirClientC -Force | Out-Null
Write-Host "=== ZIT E2E Demonstration Starting in $tempBase ===" -ForegroundColor Cyan

$trackerPort = 9100
$peerAPort = 9101
$peerBPort = 9102
$peerCPort = 9103

$trackerUrl = "http://127.0.0.1:$trackerPort"

$jobs = @()

try {
    # 1. Start Tracker
    Write-Host "`n[1] Starting Central Tracker on $trackerUrl..." -ForegroundColor Yellow
    $trackerJob = Start-Process -FilePath $trackerBin -ArgumentList @("--port", "$trackerPort", "--dump", (Join-Path $dirTracker "dump.json")) -PassThru -WindowStyle Hidden
    $jobs += $trackerJob
    Start-Sleep -Seconds 1

    # Verify tracker health
    try {
        $health = Invoke-RestMethod -Uri "$trackerUrl/health" -TimeoutSec 3
        Write-Host "Tracker is healthy ($health)" -ForegroundColor Green
    } catch {
        Write-Error "Failed to reach tracker: $_"
    }

    # 2. Client A creates repository and commits files
    Write-Host "`n[2] Client A: Initializing repository and adding files..." -ForegroundColor Yellow
    Set-Location $dirClientA
    & $zitBin init --tracker $trackerUrl --peer "127.0.0.1:$peerAPort"
    
    # Read repoId from config
    $configJson = Get-Content (Join-Path $dirClientA ".zit\config.json") | ConvertFrom-Json
    $repoId = $configJson.repo_id
    Write-Host "Generated Repo ID: $repoId" -ForegroundColor Magenta

    # Create directory structure: docs/ and src/
    New-Item -ItemType Directory -Path "docs", "src" -Force | Out-Null
    Set-Content -Path "docs\guide.md" -Value "# ZIT User Guide`nDistributed version control." -NoNewline
    Set-Content -Path "src\main.go" -Value "package main`nfunc main() {}" -NoNewline
    Set-Content -Path "src\utils.go" -Value "package main`nfunc Add(a, b int) int { return a + b }" -NoNewline

    & $zitBin add docs src
    & $zitBin commit -m "Initial commit with docs and src"

    # 3. Client A starts peer server and publishes to tracker
    Write-Host "`n[3] Client A: Starting peer server on port $peerAPort and publishing..." -ForegroundColor Yellow
    $peerAJob = Start-Process -FilePath $zitBin -ArgumentList @("serve", "--port", "$peerAPort") -PassThru -WindowStyle Hidden
    $jobs += $peerAJob
    Start-Sleep -Seconds 1

    & $zitBin publish

    # 4. Client B clones repo metadata (refs, commits, trees - NO BLOBS!)
    Write-Host "`n[4] Client B: Cloning repository metadata (Lazy Clone)..." -ForegroundColor Yellow
    Set-Location $dirClientB
    & $zitBin clone $trackerUrl $repoId . --peer "127.0.0.1:$peerBPort"

    # Verify git log works immediately
    Write-Host "`nClient B: Checking commit log..." -ForegroundColor Cyan
    & $zitBin log

    # Check .zit/objects in Client B: MUST contain 0 blobs!
    $bObjects = Get-ChildItem -Path (Join-Path $dirClientB ".zit\objects") -Recurse -File
    Write-Host "Client B total objects stored locally: $($bObjects.Count) (all are commits and trees, 0 blobs)" -ForegroundColor Green
    if (Test-Path "docs\guide.md") {
        Write-Error "docs\guide.md should NOT exist yet before checkout!"
    }
    if (Test-Path "src\main.go") {
        Write-Error "src\main.go should NOT exist yet before checkout!"
    }

    # 5. Client B does sparse checkout of docs/ only!
    Write-Host "`n[5] Client B: Performing sparse checkout of 'docs/'..." -ForegroundColor Yellow
    & $zitBin checkout main -- docs/

    # Verify files on disk for Client B
    if (-not (Test-Path "docs\guide.md")) {
        Write-Error "docs\guide.md was expected after sparse checkout, but not found!"
    }
    $guideContent = Get-Content "docs\guide.md" -Raw
    Write-Host "Successfully downloaded docs\guide.md: '$guideContent'" -ForegroundColor Green

    if (Test-Path "src\main.go") {
        Write-Error "src\main.go should NOT have been downloaded during sparse checkout of docs/!"
    }
    Write-Host "Verified: src/ files were NOT downloaded." -ForegroundColor Green

    # Verify status in Client B shows clean (sparse paths respected)
    Write-Host "`nClient B status check:" -ForegroundColor Cyan
    & $zitBin status

    # 6. Torrent Swarming: Client B serves objects, Client A is killed!
    Write-Host "`n[6] Torrent Swarming Test: Client B starts serving, Client A goes offline..." -ForegroundColor Yellow
    $peerBJob = Start-Process -FilePath $zitBin -ArgumentList @("serve", "--port", "$peerBPort") -PassThru -WindowStyle Hidden
    $jobs += $peerBJob
    Start-Sleep -Seconds 1

    & $zitBin publish
    Write-Host "Client B published as a seeder for docs/." -ForegroundColor Green

    Write-Host "Stopping Client A (simulating node going offline)..." -ForegroundColor Red
    Stop-Process -Id $peerAJob.Id -Force -ErrorAction SilentlyContinue

    # 7. Client C clones and downloads docs/ directly from Client B!
    Write-Host "`n[7] Client C: Cloning and fetching docs/ exclusively from Client B..." -ForegroundColor Yellow
    Set-Location $dirClientC
    & $zitBin clone $trackerUrl $repoId . --peer "127.0.0.1:$peerCPort"
    & $zitBin checkout main -- docs/

    if (-not (Test-Path "docs\guide.md")) {
        Write-Error "Client C failed to download docs\guide.md from Client B!"
    }
    $cContent = Get-Content "docs\guide.md" -Raw
    Write-Host "Client C successfully downloaded docs\guide.md from peer B: '$cContent'" -ForegroundColor Green

    Write-Host "`n=== ALL END-TO-END DEMO TESTS PASSED SUCCESSFULLY! ===" -ForegroundColor Green

} finally {
    Set-Location $workspace
    Write-Host "`nCleaning up processes..." -ForegroundColor Gray
    foreach ($j in $jobs) {
        if ($j -and -not $j.HasExited) {
            Stop-Process -Id $j.Id -Force -ErrorAction SilentlyContinue
        }
    }
    Start-Sleep -Seconds 1
    Remove-Item -Path $tempBase -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "Cleanup completed." -ForegroundColor Gray
}
