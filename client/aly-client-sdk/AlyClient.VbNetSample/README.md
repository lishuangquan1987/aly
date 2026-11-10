# AlySdkVbSample

VB.NET 使用示例：直接引用 C# SDK（`AlyClient.CSharpSDK`），不需要单独的模型生成。

- 目标框架：`net40`（.NET Framework 4.0，兼容 Windows XP）
- 共享实现：C# SDK 的 `AlyApi` / `AlyUpdateClient`
- 示例覆盖：`check_update` + `download_update`（含进度回调）

## 构建与运行

```powershell
cd client/aly-client-sdk/AlyClient.VbNetSample
dotnet build -c Release

# 先构建假客户端
cd ../../../tools/fake-aly-client
go build -o $env:TEMP\aly-fake.exe .
cd ../../../client/aly-client-sdk/AlyClient.VbNetSample

.\bin\Release\net40\AlySdkVbSample.exe `
  $env:TEMP\aly-fake.exe `
  ..\..\..\spec\fixtures\scenarios\update_available.json
```

输出 `PASS: VB.NET check_update`、`PASS: VB.NET download_update`、`SUMMARY: PASS=2 FAIL=0`。

## 托管生命周期示例

```vb
Imports AlyClient.CSharpSDK

Dim client = New AlyUpdateClient("C:\App\UpdateFolder\aly-client.exe")
AddHandler client.StatusChanged, Sub(status, message)
                                       Console.WriteLine($"[{status}] {message}")
                                   End Sub
AddHandler client.RequestDownloadUpdate, Sub(version)
                                             ' 非强制更新时询问用户；返回即确认
                                         End Sub
AddHandler client.RequestApplyUpdate, Sub(version)
                                          ' 同上
                                      End Sub
client.Cancel()   ' 退出前停止
client.Dispose()
```

VB.NET 与 C# 共享同一套行为：自更新、强制更新不确认、离线静默、`apply_update`
独立进程拉起。详细语义见 C# SDK README。

## 限制

- VB.NET 侧没有独立实现；能力与 C# SDK 完全一致。
- `net40` 示例使用 `Task.Result`（.NET 4.0 无 `GetAwaiter` 扩展）。
- `AlyUpdateUIClient`（GUI 启动器）同样可直接在 VB.NET 中调用。