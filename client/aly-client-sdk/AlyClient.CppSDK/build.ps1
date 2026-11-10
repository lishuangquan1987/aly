param(
    [string]$Out = "out"
)

# Build C++ SDK and conformance tests (Windows / MinGW-w64, C++11).
function Find-Gpp {
    $command = Get-Command g++ -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    $candidates = @(
        (Join-Path $env:USERPROFILE "scoop\apps\mingw-winlibs\current\bin\g++.exe"),
        "C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin\g++.exe",
        "C:\msys64\mingw64\bin\g++.exe",
        "C:\tools\mingw64\bin\g++.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return $candidate
        }
    }
    throw "g++ not found. Install MinGW-w64 (e.g. scoop install mingw-winlibs) first."
}

$root = $PSScriptRoot
Push-Location $root
try {
    $outDir = Join-Path $root $Out
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    $gpp = Find-Gpp
    & $gpp -std=c++11 -O2 -Wall -Wextra -D_WIN32_WINNT=0x0501 `
        -Iinclude src\json.cpp src\api.cpp src\client.cpp tests\conformance.cpp `
        -o (Join-Path $outDir "aly-cpp-conformance.exe") `
        -static -static-libgcc -static-libstdc++
    if ($LASTEXITCODE -ne 0) {
        throw "g++ failed with exit code $LASTEXITCODE"
    }
    Write-Host "Built $Out\aly-cpp-conformance.exe"
}
finally {
    Pop-Location
}