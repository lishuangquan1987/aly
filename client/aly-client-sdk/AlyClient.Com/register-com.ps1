param(
    [string]$DllPath = "",
    [switch]$Unregister
)

# Per-user COM registration (no admin). Works for VB6 / EPL (32-bit COM).
$progId = "AlyClient.Com.AlyComClient"
$clsid = "{c1a4e6b2-7f3d-4c8e-9a51-6d2b8f0e4c77}"
if ([string]::IsNullOrWhiteSpace($DllPath)) {
    $DllPath = Join-Path $PSScriptRoot "bin\Release\net40\AlyClient.Com.dll"
}
$dll = (Resolve-Path $DllPath).Path
$codeBase = ([Uri]$dll).AbsoluteUri

if ($Unregister) {
    Remove-Item -Path "HKCU:\Software\Classes\$progId" -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "HKCU:\Software\Classes\CLSID\$clsid" -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "Unregistered $progId"
    exit 0
}

$progKey = "HKCU:\Software\Classes\$progId"
New-Item -Path $progKey -Force | Out-Null
Set-Item -Path $progKey -Value $progId
New-Item -Path "$progKey\CLSID" -Force | Out-Null
Set-Item -Path "$progKey\CLSID" -Value $clsid

$clsidKey = "HKCU:\Software\Classes\CLSID\$clsid"
New-Item -Path $clsidKey -Force | Out-Null
Set-Item -Path $clsidKey -Value $progId

$inproc = "$clsidKey\InprocServer32"
New-Item -Path $inproc -Force | Out-Null
Set-Item -Path $inproc -Value "mscoree.dll"
Set-ItemProperty -Path $inproc -Name "ThreadingModel" -Value "Both"
Set-ItemProperty -Path $inproc -Name "Class" -Value $progId
Set-ItemProperty -Path $inproc -Name "Assembly" -Value "AlyClient.Com, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null"
Set-ItemProperty -Path $inproc -Name "RuntimeVersion" -Value "v4.0.30319"
Set-ItemProperty -Path $inproc -Name "CodeBase" -Value $codeBase

$versionKey = "$inproc\1.0.0.0"
New-Item -Path $versionKey -Force | Out-Null
Set-ItemProperty -Path $versionKey -Name "Class" -Value $progId
Set-ItemProperty -Path $versionKey -Name "Assembly" -Value "AlyClient.Com, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null"
Set-ItemProperty -Path $versionKey -Name "RuntimeVersion" -Value "v4.0.30319"
Set-ItemProperty -Path $versionKey -Name "CodeBase" -Value $codeBase

# 32-bit COM (VB6 / EPL) reads the Wow6432Node view for per-user classes.
$wowProg = "HKCU:\Software\Classes\Wow6432Node\$progId"
New-Item -Path $wowProg -Force | Out-Null
Set-Item -Path $wowProg -Value $progId
New-Item -Path "$wowProg\CLSID" -Force | Out-Null
Set-Item -Path "$wowProg\CLSID" -Value $clsid

$wowClsid = "HKCU:\Software\Classes\Wow6432Node\CLSID\$clsid"
New-Item -Path $wowClsid -Force | Out-Null
Set-Item -Path $wowClsid -Value $progId

$wowInproc = "$wowClsid\InprocServer32"
New-Item -Path $wowInproc -Force | Out-Null
Set-Item -Path $wowInproc -Value "mscoree.dll"
Set-ItemProperty -Path $wowInproc -Name "ThreadingModel" -Value "Both"
Set-ItemProperty -Path $wowInproc -Name "Class" -Value $progId
Set-ItemProperty -Path $wowInproc -Name "Assembly" -Value "AlyClient.Com, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null"
Set-ItemProperty -Path $wowInproc -Name "RuntimeVersion" -Value "v4.0.30319"
Set-ItemProperty -Path $wowInproc -Name "CodeBase" -Value $codeBase

$wowVersion = "$wowInproc\1.0.0.0"
New-Item -Path $wowVersion -Force | Out-Null
Set-ItemProperty -Path $wowVersion -Name "Class" -Value $progId
Set-ItemProperty -Path $wowVersion -Name "Assembly" -Value "AlyClient.Com, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null"
Set-ItemProperty -Path $wowVersion -Name "RuntimeVersion" -Value "v4.0.30319"
Set-ItemProperty -Path $wowVersion -Name "CodeBase" -Value $codeBase

Write-Host "Registered $progId (per-user) -> $dll"