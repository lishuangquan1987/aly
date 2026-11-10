# AlySdkFSharpSample

F# 使用示例：直接引用 C# SDK（`AlyClient.CSharpSDK`），不需要单独的模型生成。

- 目标框架：`net472`（FSharp.Core 10 需要 netstandard2.0；.NET Framework 4.7.2+）
- 共享实现：C# SDK 的 `AlyApi` / `AlyUpdateClient`
- 示例覆盖：`check_update` + `download_update`（含进度回调）

> F# 与 VB.NET/C# 共享同一套 CLR 类型；没有额外的协议实现需要维护。

## 构建与运行

```powershell
cd client/aly-client-sdk/AlyClient.FSharpSample
dotnet build -c Release

# 先构建假客户端
cd ../../../tools/fake-aly-client
go build -o $env:TEMP\aly-fake.exe .
cd ../../../client/aly-client-sdk/AlyClient.FSharpSample

.\bin\Release\net472\AlySdkFSharpSample.exe `
  $env:TEMP\aly-fake.exe `
  ..\..\..\spec\fixtures\scenarios\update_available.json
```

输出 `PASS: F# check_update`、`PASS: F# download_update`、`SUMMARY: PASS=2 FAIL=0`。

## 托管生命周期示例

```fsharp
open AlyClient.CSharpSDK

let client = new AlyUpdateClient("C:/App/UpdateFolder/aly-client.exe")
client.StatusChanged.Add(fun (status, message) -> printfn "[%s] %s" status message)
client.RequestDownloadUpdate.Add(fun version ->
    // 非强制更新时询问用户；返回即确认
    ())
client.RequestApplyUpdate.Add(fun version -> ())
client.Cancel()
client.Dispose()
```

## 限制

- F# 示例目标 `net472`；如需 .NET Framework 4.0/XP，请使用旧的 FSharp.Core 并自行验证
  （当前 FSharp.Core 10 不支持 net40）。
- F# 侧没有独立实现；能力与 C# SDK 完全一致。