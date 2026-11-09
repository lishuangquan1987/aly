param(
    [string]$Go = $(if ($env:ALY_GO127) { $env:ALY_GO127 } else { "go" })
)

$ErrorActionPreference = "Stop"
$ClientRoot = $PSScriptRoot
$AlyClientRoot = Join-Path (Split-Path $ClientRoot -Parent) "aly-client"
$UiRoot = Join-Path $ClientRoot "aly-client-linuxui"
$Dist = Join-Path $ClientRoot "dist"

# Native stderr (MyGo progress) is treated as a terminating error under Stop; collect it.
function Invoke-Go {
    param([string[]]$GoArgs)
    $oldEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & $Go @GoArgs 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $oldEap
    foreach ($line in $output) {
        if ($line -is [System.Management.Automation.ErrorRecord]) {
            Write-Host $line.Exception.Message
        } else {
            Write-Host $line
        }
    }
    if ($code -ne 0) {
        throw "go $($GoArgs -join ' ') failed (exit $code)"
    }
}

Invoke-Go -GoArgs @("version")
$goDir = Split-Path $Go -Parent
if ($goDir) { $env:PATH = $goDir + ";" + $env:PATH }

# 1) Linux updater (aly-client, no .exe), Go 1.27, GOPATH mode.
$gp = Join-Path $env:TEMP "aly-linux-gopath"
if (Test-Path $gp) { Remove-Item $gp -Recurse -Force }
New-Item -ItemType Directory -Path (Join-Path $gp "src\aly\client") -Force | Out-Null
Copy-Item $AlyClientRoot (Join-Path $gp "src\aly\client\aly-client") -Recurse -Force

$old = @{
    GOPATH = $env:GOPATH; GO111MODULE = $env:GO111MODULE
    GOOS = $env:GOOS; GOARCH = $env:GOARCH; CGO_ENABLED = $env:CGO_ENABLED
}
try {
    $env:GOPATH = $gp
    $env:GO111MODULE = "off"
    $env:GOOS = "linux"
    $env:CGO_ENABLED = "0"
    foreach ($arch in @("amd64", "arm64")) {
        $env:GOARCH = $arch
        $outDir = Join-Path $Dist "linux-$arch"
        New-Item -ItemType Directory -Path $outDir -Force | Out-Null
        Invoke-Go -GoArgs @("build", "-trimpath", "-o", (Join-Path $outDir "aly-client"), "aly/client/aly-client")
    }
} finally {
    foreach ($k in $old.Keys) {
        if ($null -eq $old[$k]) { Remove-Item "Env:$k" -ErrorAction SilentlyContinue }
        else { Set-Item "Env:$k" $old[$k] }
    }
}

# Native stderr (MyGo progress) is treated as a terminating error under Stop; collect it.
Push-Location $UiRoot
try {
    Invoke-Go -GoArgs @("tool", "mygo", "vet")
    Invoke-Go -GoArgs @("test", "./...")
    Invoke-Go -GoArgs @("tool", "mygo", "build", "-platform", "linux/amd64,linux/arm64")
    foreach ($arch in @("amd64", "arm64")) {
        $src = Join-Path $UiRoot "build\linux-$arch\aly-client-ui"
        if (-not (Test-Path $src)) { throw "missing $src" }
        $outDir = Join-Path $Dist "linux-$arch"
        New-Item -ItemType Directory -Path $outDir -Force | Out-Null
        Copy-Item $src (Join-Path $outDir "aly-client-ui") -Force
    }
} finally {
    Pop-Location
}

Write-Host "Built:"
Get-ChildItem $Dist -Recurse -File | Select-Object FullName, Length
