Option Explicit

Dim fake, scenario, shell, client
If WScript.Arguments.Count < 2 Then
    WScript.Echo "usage: vbscript-smoke.vbs <fake-aly-client.exe> <scenario.json>"
    WScript.Quit 2
End If
fake = WScript.Arguments(0)
scenario = WScript.Arguments(1)

Set shell = CreateObject("WScript.Shell")
shell.Environment("PROCESS")("ALY_FAKE_SCENARIO") = scenario

Set client = CreateObject("AlyClient.Com.AlyComClient")
client.UpdaterPath = fake

If Not client.CheckUpdate() Then
    WScript.Echo "FAIL: check_update - " & client.ErrorMsg
    WScript.Quit 1
End If
If Not client.HasUpdate Then
    WScript.Echo "FAIL: check_update - has_update=false"
    WScript.Quit 1
End If
WScript.Echo "PASS: VBScript(COM) check_update"

If Not client.DownloadUpdate() Then
    WScript.Echo "FAIL: download_update - " & client.ErrorMsg
    WScript.Quit 1
End If
If client.ProgressFile <> "app.exe" Then
    WScript.Echo "FAIL: download_update - progress=" & client.ProgressFile
    WScript.Quit 1
End If
WScript.Echo "PASS: VBScript(COM) download_update"
WScript.Echo "SUMMARY: PASS=2 FAIL=0"