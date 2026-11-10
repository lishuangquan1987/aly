# AlyClient.Com

VB6 / 易语言可用的 **COM 可见封装**，内部复用 `AlyClient.CSharpSDK`。

- ProgID：`AlyClient.Com.AlyComClient`
- 目标：`net40` / **x86**（VB6、易语言都是 32 位进程）
- 平台：Windows XP+（需 .NET Framework 4.0）
- 结果：方法统一返回 `Boolean`，详细信息通过只读属性读取

## 构建

```powershell
cd client/aly-client-sdk/AlyClient.Com
.\build.ps1
```

输出：`bin\Release\net40\AlyClient.Com.dll`（连同 C# SDK 与 Newtonsoft.Json）。

## 注册

### 方式一：当前用户（免管理员，推荐本地/测试）

```powershell
.\register-com.ps1
# 卸载
.\register-com.ps1 -Unregister
```

脚本同时写 `HKCU\Software\Classes` 与 `Wow6432Node`，32 位 COM 可直接
`CreateObject`。不需要 TLB（使用后期绑定）。

### 方式二：机器级 RegAsm（管理员）

```cmd
C:\Windows\Microsoft.NET\Framework\v4.0.30319\RegAsm.exe /codebase ^
  /tlb:AlyClient.Com.tlb bin\Release\net40\AlyClient.Com.dll
```

VB6 使用早期绑定（引用 TLB）或后期绑定均可；易语言建议后期绑定。

## 验证

```powershell
.\test.ps1
```

脚本会注册 COM、构建假客户端，并用 **32 位 PowerShell** 与
**32 位 VBScript（VB6 同款后期绑定）** 运行 smoke：

- PowerShell：[tests/com-smoke.ps1](tests/com-smoke.ps1)（check/download/list/rollback/self-update）
- VBScript：[tests/vbscript-smoke.vbs](tests/vbscript-smoke.vbs)（check/download）

## VB6 示例

```vb
Dim u As Object
Set u = CreateObject("AlyClient.Com.AlyComClient")

u.UpdaterPath = "C:\App\UpdateFolder\aly-client.exe"

If u.CheckUpdate() Then
    If u.HasUpdate And u.NeedDownloadUpdate Then
        If u.DownloadUpdate() Then
            ' 可选：u.ProgressFile / u.ProgressRatio
            u.ApplyUpdate "YourApp", 30
        Else
            MsgBox u.ErrorMsg
        End If
    End If
Else
    MsgBox u.ErrorMsg
End If

Set u = Nothing
```

## 易语言示例（COM 后期绑定）

```e
.局部变量 更新器, 对象
更新器.创建 (“AlyClient.Com.AlyComClient”, )
更新器.写属性 (“UpdaterPath”, “C:\App\UpdateFolder\aly-client.exe”)

.如果真 (更新器.调用方法 (“CheckUpdate”) ＝ 真)
    .如果真 (更新器.读属性 (“HasUpdate”) ＝ 真 且 更新器.读属性 (“NeedDownloadUpdate”) ＝ 真)
        .如果真 (更新器.调用方法 (“DownloadUpdate”) ＝ 真)
            更新器.调用方法 (“ApplyUpdate”, “YourApp”, 30)
        .如果真结束
    .如果真结束
.如果真结束
```

> 不同易语言版本的 COM 方法名可能略有差异（`方法` / `调用方法`），
> 属性与 ProgID 不变。

## 方法

| 方法 | 对应命令 | 结果属性 |
|---|---|---|
| `CheckUpdate()` | `check_update` | `HasUpdate` / `NeedDownloadUpdate` / `ForceUpdate` / `CurrentVersion` / `NewVersion` / `ReleaseNotes` |
| `CheckSelfUpdate()` | `check_self_update` | `NeedSelfUpdate` |
| `SelfUpdate()` | `self_update` | `Updated` / `SelfUpdateReason` / `OldPath` |
| `DownloadUpdate()` | `download_update` | `ProgressFile` / `ProgressRatio`（阻塞到完成） |
| `ApplyUpdate(names, timeout)` | `apply_update` | 拉起后立即返回 |
| `ListRollbackVersions()` | `list_rollback_versions` | `RollbackVersions`（换行分隔） |
| `Rollback(version, names, timeout)` | `rollback` | `RolledBackVersion` |

所有方法成功/失败看返回值和 `IsSuccess`，失败原因在 `ErrorMsg`。

## 限制

- **x86 only**：DLL 为 32 位；64 位宿主不能加载。
- `DownloadUpdate` 是同步阻塞调用，VB6 UI 线程会卡住；大更新建议在单独线程调用。
- 没有逐文件进度回调（COM 事件未封装），只暴露最后一次 `ProgressFile/Ratio`。
- C# SDK 当前未提供 `clean` / `check_diff` 的公开方法，COM 也未暴露。
- 需要 .NET Framework 4.0；XP 需先安装 .NET 4.0 与对应更新器。