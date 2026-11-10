param(
    [string]$Out = "out"
)

# Build the Pascal SDK and conformance tests (Free Pascal / Delphi mode).
function Find-Fpc {
    $command = Get-Command fpc -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    $candidates = @(
        (Join-Path $env:USERPROFILE "fpc\3.2.2\bin\i386-win32\fpc.exe"),
        "C:\FPC\3.2.2\bin\i386-win32\fpc.exe",
        "C:\lazarus\fpc\3.2.2\bin\i386-win32\fpc.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return $candidate
        }
    }
    throw "fpc not found. Install Free Pascal 3.2.2 first."
}

$root = $PSScriptRoot
Push-Location $root
try {
    $outDir = Join-Path $root $Out
    $unitDir = Join-Path $outDir "units"
    New-Item -ItemType Directory -Force -Path $unitDir | Out-Null
    $fpc = Find-Fpc
    & $fpc -Mdelphi -Fu"src" -FU"$unitDir" -FE"$outDir" -O2 "tests\conformance.pas"
    if ($LASTEXITCODE -ne 0) {
        throw "fpc failed with exit code $LASTEXITCODE"
    }
    Write-Host "Built $Out\conformance.exe"
}
finally {
    Pop-Location
}