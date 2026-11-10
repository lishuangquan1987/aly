param(
    [string]$Fake,
    [string]$Scenario
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($Fake) -or [string]::IsNullOrWhiteSpace($Scenario)) {
    throw "usage: com-smoke.ps1 -Fake <fake-aly-client.exe> -Scenario <scenario.json>"
}

$env:ALY_FAKE_SCENARIO = [IO.Path]::GetFullPath($Scenario)
$client = New-Object -ComObject AlyClient.Com.AlyComClient
$client.UpdaterPath = $Fake

if (-not $client.CheckUpdate()) {
    throw "check_update failed: $($client.ErrorMsg)"
}
if (-not $client.HasUpdate) {
    throw "check_update: has_update=false"
}
if ($client.NewVersion -ne "1.0.1") {
    throw "check_update: new_version=$($client.NewVersion)"
}
Write-Host "PASS: COM check_update"

if (-not $client.DownloadUpdate()) {
    throw "download_update failed: $($client.ErrorMsg)"
}
if ($client.ProgressFile -ne "app.exe" -or [Math]::Abs($client.ProgressRatio - 1.0) -gt 0.0001) {
    throw "download_update: progress mismatch"
}
Write-Host "PASS: COM download_update"

$rollbackScenario = Join-Path ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Scenario))) "rollback.json"
$env:ALY_FAKE_SCENARIO = $rollbackScenario
$client2 = New-Object -ComObject AlyClient.Com.AlyComClient
$client2.UpdaterPath = $Fake
if (-not $client2.ListRollbackVersions()) {
    throw "list_rollback_versions failed: $($client2.ErrorMsg)"
}
if ($client2.RollbackVersions -notmatch "1.0.0") {
    throw "list_rollback_versions: $($client2.RollbackVersions)"
}
Write-Host "PASS: COM list_rollback_versions"

if (-not $client2.Rollback("1.0.0", "", 30)) {
    throw "rollback failed: $($client2.ErrorMsg)"
}
if ($client2.RolledBackVersion -ne "1.0.0") {
    throw "rollback: version=$($client2.RolledBackVersion)"
}
Write-Host "PASS: COM rollback"

$selfScenario = Join-Path ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Scenario))) "self_update.json"
$env:ALY_FAKE_SCENARIO = $selfScenario
$client3 = New-Object -ComObject AlyClient.Com.AlyComClient
$client3.UpdaterPath = $Fake
if (-not $client3.CheckSelfUpdate() -or -not $client3.NeedSelfUpdate) {
    throw "check_self_update failed: $($client3.ErrorMsg)"
}
if (-not $client3.SelfUpdate() -or -not $client3.Updated) {
    throw "self_update failed: $($client3.ErrorMsg)"
}
Write-Host "PASS: COM self_update"

Write-Host "SUMMARY: PASS=5 FAIL=0"