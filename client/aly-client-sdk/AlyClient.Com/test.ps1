param(
    [string]$Configuration = "Release"
)

# Build/register the COM wrapper, build the fake client, run 32-bit COM smoke tests.
$repo = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
$dll = Join-Path $PSScriptRoot "bin\$Configuration\net40\AlyClient.Com.dll"
if (-not (Test-Path $dll)) {
    throw "AlyClient.Com.dll not found. Run build.ps1 first."
}

& (Join-Path $PSScriptRoot "register-com.ps1") -DllPath $dll

$outDir = Join-Path $PSScriptRoot "out"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$fake = Join-Path $outDir "aly-fake.exe"
Push-Location (Join-Path $repo "tools\fake-aly-client")
try {
    go build -o $fake .
    if ($LASTEXITCODE -ne 0) {
        throw "go build fake client failed"
    }
}
finally {
    Pop-Location
}

$ps32 = "C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe"
$scenario = Join-Path $repo "spec\fixtures\scenarios\update_available.json"
& $ps32 -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "tests\com-smoke.ps1") `
    -Fake $fake -Scenario $scenario
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}