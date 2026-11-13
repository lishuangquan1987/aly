$ErrorActionPreference = 'Stop'

# This machine keeps GOROOT pointing at Go 1.10 for the XP client. Derive the
# root from the go.exe actually on PATH (Go 1.27.3 for this module).
$goCmd = Get-Command go -ErrorAction Stop
$goRoot = Split-Path (Split-Path $goCmd.Source -Parent) -Parent
if (Test-Path (Join-Path $goRoot 'src\runtime')) {
    $env:GOROOT = $goRoot
}

$env:GO111MODULE = 'on'
$env:CGO_ENABLED = '0'

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'

function Check-Exit([string]$what) {
    if ($LASTEXITCODE -ne 0) { throw "$what failed (exit $LASTEXITCODE)" }
}

go version
Check-Exit 'go version'
go list std > $null
Check-Exit 'go list std'
go fmt ./...
Check-Exit 'go fmt'
go vet ./...
Check-Exit 'go vet'
go test ./...
Check-Exit 'go test'

$targets = @(
    @{ GOOS = 'windows'; GOARCH = 'amd64'; Name = 'build/windows-amd64/AlyPublish.exe'; Ldflags = @('-H', 'windowsgui') },
    @{ GOOS = 'linux';   GOARCH = 'amd64'; Name = 'build/linux-amd64/AlyPublish'; Ldflags = @() },
    @{ GOOS = 'linux';   GOARCH = 'arm64'; Name = 'build/linux-arm64/AlyPublish'; Ldflags = @() }
)

foreach ($t in $targets) {
    $env:GOOS = $t.GOOS
    $env:GOARCH = $t.GOARCH
    $dir = Split-Path $t.Name
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    if ($t.Ldflags.Count -gt 0) {
        $ld = $t.Ldflags -join ' '
        go build -trimpath -ldflags $ld -o $t.Name .
    } else {
        go build -trimpath -o $t.Name .
    }
    Check-Exit "build $($t.GOOS)/$($t.GOARCH)"
    Get-Item $t.Name | Select-Object FullName, Length
}

$env:GOOS = $oldGOOS
$env:GOARCH = $oldGOARCH
$env:CGO_ENABLED = $oldCGO