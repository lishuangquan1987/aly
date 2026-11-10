Imports System
Imports System.Threading
Imports AlyClient.CSharpSDK
Imports AlyClient.CSharpSDK.Models

Module Program
    Function Main(args As String()) As Integer
        If args.Length < 2 Then
            Console.WriteLine("usage: AlySdkVbSample.exe <fake-aly-client.exe> <scenario.json>")
            Return 2
        End If

        Dim fakePath = args(0)
        Dim scenarioPath = IO.Path.GetFullPath(args(1))
        Environment.SetEnvironmentVariable("ALY_FAKE_SCENARIO", scenarioPath)
        Dim failures As Integer = 0

        Dim check = AlyApi.CheckUpdateAsync(fakePath).Result
        If Not check.IsSuccess Then
            Console.WriteLine("FAIL: check_update - " & check.ErrorMsg)
            failures += 1
        ElseIf check.Data Is Nothing OrElse Not check.Data.HasUpdate Then
            Console.WriteLine("FAIL: check_update - has_update=false")
            failures += 1
        Else
            Console.WriteLine("PASS: VB.NET check_update")
        End If

        Dim progressFile As String = ""
        Dim progressRatio As Double = 0
        Dim download = AlyApi.DownloadUpdateAsync(
            fakePath,
            Sub(file As String, ratio As Double)
                progressFile = file
                progressRatio = ratio
            End Sub).Result
        If Not download.IsSuccess Then
            Console.WriteLine("FAIL: download_update - " & download.ErrorMsg)
            failures += 1
        ElseIf progressFile <> "app.exe" OrElse Math.Abs(progressRatio - 1.0) > 0.0001 Then
            Console.WriteLine("FAIL: download_update - progress mismatch")
            failures += 1
        Else
            Console.WriteLine("PASS: VB.NET download_update")
        End If

        Console.WriteLine("SUMMARY: PASS=" & (2 - failures).ToString() & " FAIL=" & failures.ToString())
        If failures = 0 Then Return 0
        Return 1
    End Function
End Module