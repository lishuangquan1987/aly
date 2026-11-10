param(
    [string]$Out = "out"
)

# 编译 Java SDK + 测试（目标 Java 8）。
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $files = Get-ChildItem -Path src -Recurse -Filter *.java | ForEach-Object { $_.FullName }
    $argFile = Join-Path $env:TEMP "aly-java-sources.txt"
    $files | Set-Content -Path $argFile -Encoding ASCII
    & javac -encoding UTF-8 --release 8 -Xlint:-options -d $Out "@$argFile"
    if ($LASTEXITCODE -ne 0) {
        throw "javac failed with exit code $LASTEXITCODE"
    }
    Write-Host "Java classes written to $Out"
}
finally {
    Pop-Location
}