param(
    [string]$Out = "out"
)

# Build the fake client and run C++ conformance tests.
$root = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
Push-Location $root
try {
    $outDir = Join-Path $PSScriptRoot $Out
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    $fake = Join-Path $outDir "aly-fake.exe"

    Push-Location (Join-Path $root "tools\fake-aly-client")
    try {
        go build -o $fake .
        if ($LASTEXITCODE -ne 0) {
            throw "go build fake client failed"
        }
    }
    finally {
        Pop-Location
    }

    & (Join-Path $outDir "aly-cpp-conformance.exe") $fake (Join-Path $root "spec\fixtures\scenarios")
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
finally {
    Pop-Location
}