# stage-payload.ps1 -- lay out the AlyPublish package payload.
#
# Package contract (read by the GUI and by the terminal updater):
#   <App>/payload.json                    platform index, one of installer/payload*.json
#   <App>/publish-cli/<rid>/aly-publish   publish CLI (Windows keeps the .exe suffix)
#   <App>/client/<rid>/aly-client(-ui)    terminal updater and its front end
#
# Two sources, identical tail vification:
#   CI    : -Staging <download root>, asset names as uploaded by the workflow
#   local : -FromLocal, artifacts already in the repository (deploy-publish.bat)
#
# Usage:
#   powershell -File publish/publish-gui/installer/stage-payload.ps1 `
#     -Staging <download root> -App <package root> -Manifest payload.json `
#     -AssetCli windows-amd64 -AssetUp windows-x86
#
# Missing payload file = failure: the package is the only channel for the updater
# binaries, so a partial package must never ship silently.

param(
    [string]$Staging = "publish/publish-gui/artifacts/staging",
    [string]$App = "publish/publish-gui/artifacts/app",
    [string]$Manifest = "payload.json",
    [string]$AssetCli = "windows-amd64",
    [string]$AssetUp = "windows-x86",
    [switch]$FromLocal
)

$ErrorActionPreference = "Stop"

$repo = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
$stagingRoot = if ([IO.Path]::IsPathRooted($Staging)) { $Staging } else { Join-Path $repo $Staging }
$appRoot = if ([IO.Path]::IsPathRooted($App)) { $App } else { Join-Path $repo $App }

function Write-Utf8NoBom([string]$Path, [string]$Text) {
    [IO.File]::WriteAllText($Path, $Text, (New-Object Text.UTF8Encoding($false)))
}

# Copy-File is the only writer: make the parent, overwrit, report the package relative path.
function Copy-File([string]$Sorce, [string]$To) {
    $dst = Join-Path $appRoot $To
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dst) | Out-Null
    Copy-Item $Sorce $dst -Force
    Write-Host ("  ok {0}" -f $To)
}

# Copy-Payload takes one CI artifact: try <Staging>/<asset>/<file> first, else look for
# the same file name anywhere under the asset directory (download depth is not hardcoded).
function Copy-Payload([string]$Asset, [string]$File, [string]$To) {
    $direct = Join-Path $stagingRoot $Asset $File
    $src = $direct
    if (-not (Test-Path $src)) {
        $hit = Get-ChildItem (Join-Path $stagingRoot $Asset) -Recurse -Filter $File -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($hit) { $src = $hit.FullName }
    }
    if (-not (Test-Path $src)) {
        throw ("payload source missing: {0}" -f $direct)
    }
    Copy-File $src $To
}

# Copy-Local takes a repository artifact: the path is fixed, a miss is a miss.
function Copy-Local([string]$From, [string]$To) {
    $src = Join-Path $repo $From
    if (-not (Test-Path $src)) {
        throw ("local artifact missing: {0}" -f $src)
    }
    Copy-File $src $To
}

Write-Host "=== stage payload ==="
Write-Host ("manifest={0} asset_cli={1} asset_up={2} local={3}" -f $Manifest, $AssetCli, $AssetUp, $FromLocal)

if ($FromLocal) {
    # Local build products plus the updaters already in the repository.
    # The Linux updaters come from client/aly-client-ui/dist (build-linux.sh products).
    Copy-Local "publish\publish-cli\aly-publish.exe" "publish-cli\windows-amd64\aly-publish.exe"
    Copy-Local "publish\publish-cli\aly-publish-linux-amd64" "publish-cli\linux-amd64\aly-publish"
    Copy-Local "publish\publish-cli\aly-publish-linux-arm64" "publish-cli\linux-arm64\aly-publish"

    Copy-Local "client\aly-client\aly-client.exe" "client\windows-x86\aly-client.exe"
    Copy-Local "client\aly-client-ui\aly-client-winui\aly-client-ui.exe" "client\windows-x86\aly-client-ui.exe"
    Copy-Local "client\aly-client-ui\dist\linux-amd64\aly-client" "client\linux-amd64\aly-client"
    Copy-Local "client\aly-client-ui\dist\linux-amd64\aly-client-ui" "client\linux-amd64\aly-client-ui"
    Copy-Local "client\aly-client-ui\dist\linux-arm64\aly-client" "client\linux-arm64\aly-client"
    Copy-Local "client\aly-client-ui\dist\linux-arm64\aly-client-ui" "client\linux-arm64\aly-client-ui"
} else {
    # CI artifacts as downloaded by actions/download-artifact@v4.
    Copy-Payload "aly-publish-windows-amd64" "aly-publish-windows-amd64.exe" "publish-cli\windows-amd64\aly-publish.exe"
    Copy-Payload "aly-publish-linux-amd64" "aly-publish-linux-amd64" "publish-cli\linux-amd64\aly-publish"
    Copy-Payload "aly-publish-linux-arm64" "aly-publish-linux-arm64" "publish-cli\linux-arm64\aly-publish"

    Copy-Payload "aly-client-win-xp" "aly-client.exe" "client\windows-x86\aly-client.exe"
    Copy-Payload "aly-client-win-xp" "aly-client-ui.exe" "client\windows-x86\aly-client-ui.exe"
    Copy-Payload "aly-client-linux" "aly-client-linux-amd64" "client\linux-amd64\aly-client"
    Copy-Payload "aly-client-linux" "aly-client-ui-linux-amd64" "client\linux-amd64\aly-client-ui"
    Copy-Payload "aly-client-linux" "aly-client-linux-arm64" "client\linux-arm64\aly-client"
    Copy-Payload "aly-client-linux" "aly-client-ui-linux-arm64" "client\linux-arm64\aly-client-ui"
}

# payload.json comes from the repository so the run and the CI stage the same byts.
Copy-File (Join-Path (Join-Path $repo "publish\publish-gui\installer") $Manifest) "payload.json"

Get-ChildItem $appRoot -Filter *.pdb -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force

$expected = @(
    "publish-cli\windows-amd64\aly-publish.exe",
    "publish-cli\linux-amd64\aly-publish",
    "publish-cli\linux-arm64\aly-publish",
    "client\windows-x86\aly-client.exe",
    "client\windows-x86\aly-client-ui.exe",
    "client\linux-amd64\aly-client",
    "client\linux-amd64\aly-client-ui",
    "client\linux-arm64\aly-client",
    "client\linux-arm64\aly-client-ui",
    "payload.json"
)
$missing = 0
foreach ($rel in $expected) {
    if (-not (Test-Path (Join-Path $appRoot $rel))) {
        Write-Error ("MISSING {0}" -f $rel)
        $missing = $missing + 1
    }
}
if ($missing -gt 0) {
    throw ("package incomplete: {0} file(s) missing" -f $missing)
}

Write-Host "=== payload layout ==="
Get-ChildItem $appRoot -Recurse | Select-Object FullName,Length
Write-Host "payload ready"
