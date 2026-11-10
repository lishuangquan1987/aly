param(
    [string]$Version = "1.0.0",
    [string]$Out = "artifacts/sdks",
    [switch]$SkipCSharp
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$sdkRoot = Join-Path $repo "client\aly-client-sdk"
$outDir = if ([IO.Path]::IsPathRooted($Out)) { $Out } else { Join-Path $repo $Out }
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$work = Join-Path $env:TEMP ("aly-sdk-package-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $work | Out-Null

function New-Stage([string]$Name) {
    $path = Join-Path $work $Name
    New-Item -ItemType Directory -Force -Path $path | Out-Null
    return $path
}

function Invoke-Native([string]$Exe, [string[]]$Arguments, [string]$WorkingDirectory) {
    Push-Location $WorkingDirectory
    $oldErrorAction = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $Exe @Arguments 2>&1 | ForEach-Object { Write-Host $_ }
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $oldErrorAction
        Pop-Location
    }
    if ($code -ne 0) {
        throw "$Exe failed with exit code $code"
    }
}

function New-Zip([string]$Stage, [string]$Name) {
    $dest = Join-Path $outDir $Name
    if (Test-Path $dest) {
        Remove-Item $dest -Force
    }
    Compress-Archive -Path (Join-Path $Stage "*") -DestinationPath $dest -Force
    Write-Host "Packaged $Name"
}

function Write-Utf8NoBom([string]$Path, [string]$Text) {
    [IO.File]::WriteAllText($Path, $Text, (New-Object Text.UTF8Encoding($false)))
}

function Set-PackageVersion([string]$Path, [string]$Old, [string]$New) {
    $text = Get-Content $Path -Raw -Encoding UTF8
    $text = $text.Replace($Old, $New)
    Write-Utf8NoBom $Path $text
}

# C# (nupkg + README)
if (-not $SkipCSharp) {
    $stage = New-Stage "csharp"
    $proj = Join-Path $sdkRoot "AlyClient.CSharpSDK\AlyClient.CSharpSDK\AlyClient.CSharpSDK.csproj"
    Invoke-Native "dotnet" @("pack", $proj, "-c", "Release", "-o", $stage, "-p:Version=$Version") $repo
    Copy-Item (Join-Path $sdkRoot "AlyClient.CSharpSDK\AlyClient.CSharpSDK\README.md") $stage -Force
    New-Zip $stage "AlyClient.CSharpSDK-$Version.zip"
}

# TypeScript (dist + package metadata + sources)
$tsSdk = Join-Path $sdkRoot "AlyClient.TypeScriptSDK"
Invoke-Native "npm" @("ci") $tsSdk
Invoke-Native "npm" @("run", "build") $tsSdk
$stage = New-Stage "typescript"
Copy-Item (Join-Path $tsSdk "dist") (Join-Path $stage "dist") -Recurse -Force
Copy-Item (Join-Path $tsSdk "src") (Join-Path $stage "src") -Recurse -Force
Copy-Item (Join-Path $tsSdk "test") (Join-Path $stage "test") -Recurse -Force
Copy-Item (Join-Path $tsSdk "package.json") $stage -Force
Copy-Item (Join-Path $tsSdk "README.md") $stage -Force
Set-PackageVersion (Join-Path $stage "package.json") '"version": "1.0.0"' ('"version": "' + $Version + '"')
New-Zip $stage "AlyClient.TypeScriptSDK-$Version.zip"

# Go (module source)
$goSdk = Join-Path $sdkRoot "AlyClient.GoSDK"
Get-ChildItem $goSdk -Filter "*.go" | Copy-Item -Destination (New-Stage "go") -Force
Copy-Item (Join-Path $goSdk "go.mod") (Join-Path $work "go") -Force
Copy-Item (Join-Path $goSdk "README.md") (Join-Path $work "go") -Force
New-Zip (Join-Path $work "go") "AlyClient.GoSDK-$Version.zip"

# Python (wheel + source)
$pySdk = Join-Path $sdkRoot "AlyClient.PythonSDK"
$pySrc = New-Stage "python-src"
Copy-Item (Join-Path $pySdk "alyclient") (Join-Path $pySrc "alyclient") -Recurse -Force
Copy-Item (Join-Path $pySdk "pyproject.toml") $pySrc -Force
Copy-Item (Join-Path $pySdk "README.md") $pySrc -Force
Set-PackageVersion (Join-Path $pySrc "pyproject.toml") 'version = "1.0.0"' ('version = "' + $Version + '"')
$stage = New-Stage "python"
Invoke-Native "python" @("-m", "pip", "wheel", $pySrc, "--no-deps", "-w", (Join-Path $stage "dist")) $repo
Copy-Item (Join-Path $pySrc "alyclient") (Join-Path $stage "alyclient") -Recurse -Force
Copy-Item (Join-Path $pySrc "pyproject.toml") $stage -Force
Copy-Item (Join-Path $pySrc "README.md") $stage -Force
New-Zip $stage "AlyClient.PythonSDK-$Version.zip"

# Java (jar + source)
$javaSdk = Join-Path $sdkRoot "AlyClient.JavaSDK"
$stage = New-Stage "java"
$classes = Join-Path $stage "classes"
New-Item -ItemType Directory -Force -Path $classes | Out-Null
$javaFiles = Get-ChildItem (Join-Path $javaSdk "src") -Recurse -Filter "*.java" | ForEach-Object { $_.FullName }
Invoke-Native "javac" (@("-encoding", "UTF-8", "--release", "8", "-d", $classes) + $javaFiles) $repo
$manifest = Join-Path $stage "MANIFEST.MF"
Write-Utf8NoBom $manifest ("Manifest-Version: 1.0`r`nImplementation-Title: AlyClient.JavaSDK`r`nImplementation-Version: " + $Version + "`r`n")
Invoke-Native "jar" @("cfm", (Join-Path $stage "AlyClient.JavaSDK-$Version.jar"), $manifest, "-C", $classes, ".") $repo
Remove-Item $classes -Recurse -Force
Remove-Item $manifest -Force
Copy-Item (Join-Path $javaSdk "src") (Join-Path $stage "src") -Recurse -Force
Copy-Item (Join-Path $javaSdk "README.md") $stage -Force
New-Zip $stage "AlyClient.JavaSDK-$Version.zip"

# Rust (.crate + source)
$rustSdk = Join-Path $sdkRoot "AlyClient.RustSDK"
$rustSrc = New-Stage "rust-src"
Copy-Item (Join-Path $rustSdk "src") (Join-Path $rustSrc "src") -Recurse -Force
Copy-Item (Join-Path $rustSdk "Cargo.toml") $rustSrc -Force
Copy-Item (Join-Path $rustSdk "Cargo.lock") $rustSrc -Force
Copy-Item (Join-Path $rustSdk "README.md") $rustSrc -Force
Set-PackageVersion (Join-Path $rustSrc "Cargo.toml") 'version = "1.0.0"' ('version = "' + $Version + '"')
Invoke-Native "cargo" @("package", "--allow-dirty", "--no-verify") $rustSrc
$crate = Get-ChildItem (Join-Path $rustSrc "target\package") -Filter "*.crate" | Select-Object -First 1
if (-not $crate) {
    throw "cargo package did not produce a .crate"
}
$stage = New-Stage "rust"
Copy-Item $crate.FullName $stage -Force
Copy-Item (Join-Path $rustSrc "src") (Join-Path $stage "src") -Recurse -Force
Copy-Item (Join-Path $rustSrc "Cargo.toml") $stage -Force
Copy-Item (Join-Path $rustSrc "Cargo.lock") $stage -Force
Copy-Item (Join-Path $rustSrc "README.md") $stage -Force
New-Zip $stage "AlyClient.RustSDK-$Version.zip"

# C++ (compiled check + source)
$cppSdk = Join-Path $sdkRoot "AlyClient.CppSDK"
Invoke-Native "powershell" @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $cppSdk "build.ps1")) $cppSdk
$stage = New-Stage "cpp"
Copy-Item (Join-Path $cppSdk "include") (Join-Path $stage "include") -Recurse -Force
Copy-Item (Join-Path $cppSdk "src") (Join-Path $stage "src") -Recurse -Force
Copy-Item (Join-Path $cppSdk "tests") (Join-Path $stage "tests") -Recurse -Force
Copy-Item (Join-Path $cppSdk "build.ps1") $stage -Force
Copy-Item (Join-Path $cppSdk "build.sh") $stage -Force
Copy-Item (Join-Path $cppSdk "README.md") $stage -Force
New-Zip $stage "AlyClient.CppSDK-$Version.zip"

# Pascal (compiled check + source)
$pasSdk = Join-Path $sdkRoot "AlyClient.PascalSDK"
Invoke-Native "powershell" @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $pasSdk "build.ps1")) $pasSdk
$stage = New-Stage "pascal"
Copy-Item (Join-Path $pasSdk "src") (Join-Path $stage "src") -Recurse -Force
Copy-Item (Join-Path $pasSdk "tests") (Join-Path $stage "tests") -Recurse -Force
Copy-Item (Join-Path $pasSdk "build.ps1") $stage -Force
Copy-Item (Join-Path $pasSdk "test.ps1") $stage -Force
Copy-Item (Join-Path $pasSdk "README.md") $stage -Force
New-Zip $stage "AlyClient.PascalSDK-$Version.zip"

# VB.NET (source + compiled output)
$vbSdk = Join-Path $sdkRoot "AlyClient.VbNetSample"
Invoke-Native "dotnet" @("build", (Join-Path $vbSdk "AlySdkVbSample.vbproj"), "-c", "Release", "-p:Version=$Version") $repo
$stage = New-Stage "vbnet"
Copy-Item (Join-Path $vbSdk "Program.vb") $stage -Force
Copy-Item (Join-Path $vbSdk "AlySdkVbSample.vbproj") $stage -Force
Copy-Item (Join-Path $vbSdk "README.md") $stage -Force
Copy-Item (Join-Path $vbSdk "bin\Release\net40") (Join-Path $stage "bin\net40") -Recurse -Force
New-Zip $stage "AlyClient.VbNetSample-$Version.zip"

# F# (source + compiled output)
$fsSdk = Join-Path $sdkRoot "AlyClient.FSharpSample"
Invoke-Native "dotnet" @("build", (Join-Path $fsSdk "AlySdkFSharpSample.fsproj"), "-c", "Release", "-p:Version=$Version") $repo
$stage = New-Stage "fsharp"
Copy-Item (Join-Path $fsSdk "Program.fs") $stage -Force
Copy-Item (Join-Path $fsSdk "AlySdkFSharpSample.fsproj") $stage -Force
Copy-Item (Join-Path $fsSdk "README.md") $stage -Force
Copy-Item (Join-Path $fsSdk "bin\Release\net472") (Join-Path $stage "bin\net472") -Recurse -Force
New-Zip $stage "AlyClient.FSharpSample-$Version.zip"

# COM (x86 DLL + registration scripts)
$comSdk = Join-Path $sdkRoot "AlyClient.Com"
Invoke-Native "dotnet" @("build", (Join-Path $comSdk "AlyClient.Com.csproj"), "-c", "Release") $repo
$stage = New-Stage "com"
Copy-Item (Join-Path $comSdk "bin\Release\net40\*") $stage -Force
Copy-Item (Join-Path $comSdk "README.md") $stage -Force
Copy-Item (Join-Path $comSdk "register-com.ps1") $stage -Force
Copy-Item (Join-Path $comSdk "tests") (Join-Path $stage "tests") -Recurse -Force
New-Zip $stage "AlyClient.Com-$Version.zip"

Remove-Item $work -Recurse -Force
Write-Host "SDK zips written to $outDir"
Get-ChildItem $outDir | Select-Object Name, Length