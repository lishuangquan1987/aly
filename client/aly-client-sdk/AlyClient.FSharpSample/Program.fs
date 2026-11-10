open System
open AlyClient.CSharpSDK
open AlyClient.CSharpSDK.Models

[<EntryPoint>]
let main argv =
    if argv.Length < 2 then
        Console.WriteLine("usage: AlySdkFSharpSample.exe <fake-aly-client.exe> <scenario.json>")
        2
    else
        let fakePath = argv.[0]
        let scenarioPath = IO.Path.GetFullPath argv.[1]
        Environment.SetEnvironmentVariable("ALY_FAKE_SCENARIO", scenarioPath)
        let mutable failures = 0

        let check = AlyApi.CheckUpdateAsync(fakePath).GetAwaiter().GetResult()
        if not check.IsSuccess then
            Console.WriteLine("FAIL: check_update - " + check.ErrorMsg)
            failures <- failures + 1
        elif isNull (box check.Data) || not check.Data.HasUpdate then
            Console.WriteLine("FAIL: check_update - has_update=false")
            failures <- failures + 1
        else
            Console.WriteLine("PASS: F# check_update")

        let mutable progressFile = ""
        let mutable progressRatio = 0.0
        let download =
            AlyApi.DownloadUpdateAsync(
                fakePath,
                fun file ratio ->
                    progressFile <- file
                    progressRatio <- ratio).GetAwaiter().GetResult()
        if not download.IsSuccess then
            Console.WriteLine("FAIL: download_update - " + download.ErrorMsg)
            failures <- failures + 1
        elif progressFile <> "app.exe" || abs (progressRatio - 1.0) > 0.0001 then
            Console.WriteLine("FAIL: download_update - progress mismatch")
            failures <- failures + 1
        else
            Console.WriteLine("PASS: F# download_update")

        Console.WriteLine("SUMMARY: PASS=" + string (2 - failures) + " FAIL=" + string failures)
        if failures = 0 then 0 else 1