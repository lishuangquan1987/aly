# AlyClient.PascalSDK

`aly-client` 的 Object Pascal / Delphi 宿主 SDK（Free Pascal 3.2.2，Delphi mode）。

- 更新器：`UpdateFolder/aly-client.exe`
- 目标：Windows XP+（FPC i386-win32，Win32 API）
- 模型：`src/models_gen.pas` 由 `spec/aly-protocol.schema.json` 生成，**不要手改**
- 依赖：FPC RTL（`fpjson`、`Process`、`Classes`），无第三方包

## 与 Delphi 的关系

源码使用 `{$mode delphi}`，可直接被 FPC 编译；**原生 Delphi 需要替换 `TProcess`
进程封装**（FPC 提供的 `Process` 单元不属于 Delphi RTL），协议/模型/生命周期代码可复用。
本仓库的 CI 与本地验证使用 Free Pascal。

## 构建与测试

先安装 Free Pascal 3.2.2（Windows i386 安装包）。然后：

```powershell
cd client/aly-client-sdk/AlyClient.PascalSDK
.\build.ps1          # 生成 out\conformance.exe
.\test.ps1           # 构建假客户端并回放 fixtures
```

`build.ps1` 会自动查找 `fpc.exe`，也可把 FPC 加入 PATH。

## 快速开始（托管生命周期）

```pascal
uses alyclient;

type
  TAppCallbacks = class
    procedure OnStatus(const Status, Message: string);
    procedure OnConfirmDownload(const Version: string; Token: TCancelToken);
  end;

procedure TAppCallbacks.OnStatus(const Status, Message: string);
begin
  WriteLn('[', Status, '] ', Message);
end;

procedure TAppCallbacks.OnConfirmDownload(const Version: string; Token: TCancelToken);
begin
  // 非强制更新时询问用户；返回即确认。
  // 阻塞时轮询 Token，保证 Stop 能及时退出。
end;

var
  Api: TApi;
  Client: TUpdateClient;
  Callbacks: TAppCallbacks;
begin
  Api := TApi.Create('C:\App\UpdateFolder\aly-client.exe');
  Callbacks := TAppCallbacks.Create;
  Client := TUpdateClient.Create(Api);
  try
    Client.SetOnStatus(Callbacks.OnStatus);
    Client.SetConfirmDownload(Callbacks.OnConfirmDownload);
    Client.Start;
    // ...
    Client.Stop(5000);
  finally
    Client.Free;
    Callbacks.Free;
    Api.Free;
  end;
end.
```

行为与其它 SDK 一致：

- 启动时先尝试 `check_self_update` / `self_update`；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 抖动），`Stop` 可中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- 服务器不可达时 `has_update=false`，已下载产物保留；
- `apply_update` 以独立进程拉起后立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

业务失败不抛异常，返回 `IsSuccess=False` + `ErrorMsg`：

```pascal
var
  Api: TApi;
  Check: TCheckUpdateResponse;
  Options: TDownloadOptions;
  Sink: TProgressSink;
  Downloaded: TBasicResponse;
  CloseOptions: TCloseOptions;
begin
  Api := TApi.Create('C:/App/UpdateFolder/aly-client.exe');
  Check := Api.CheckUpdate;
  if Check.IsSuccess and Check.HasData and Check.Data.has_update
     and Check.Data.need_download_update then
  begin
    Options := DefaultDownloadOptions;
    Options.OnProgress := Sink.OnProgress;
    Downloaded := Api.DownloadUpdate(Options);
    if Downloaded.IsSuccess then
    begin
      CloseOptions := DefaultCloseOptions;
      CloseOptions.MustCloseProcessNames := 'YourApp';
      Api.ApplyUpdate(CloseOptions);
    end;
  end;
end;
```

| 方法 | 对应命令 | 默认超时 |
|---|---|---|
| `CheckUpdate` / `CheckSelfUpdate` | `check_update` / `check_self_update` | 30s |
| `SelfUpdate` | `self_update` | 30s |
| `ListRollbackVersions` / `CheckDiff` | 同名命令 | 30s |
| `Clean(Options)` | `clean` | 30s |
| `Rollback(Version, Options)` | `rollback` | 600s |
| `DownloadUpdate(Options)` | `download_update` | 无总时长；停流看门狗 1800s |
| `ApplyUpdate(Options)` | `apply_update` | 不等待 |

下载进度只统计 `DONE` 行；`TDownloadProgressInfo` 带字节字段。

## 字段命名

生成模型保持协议 snake_case；Pascal 保留字 `file` 映射为 `fileName`：

```pascal
Check.Data.has_update;
Check.Data.new_version;       // 缺省为空串
Check.Data.force_update;      // 缺省为 False
Check.Data.release_notes;     // 缺省为空数组
Progress.fileName;
```

可选字段没有 presence 标志：缺省时字符串为空、数字为 0、布尔为 False、数组为空。
未知 JSON 字段被忽略；未知状态字符串可正常读取（`status` 是 `string`）。

## 已知边界

- 仅 Windows；不提供 `aly-client-ui` 启动器。
- 不做自更新本地替换兜底（只委派 `self_update`）。
- 原生 Delphi 需替换 `TProcess` 封装；FPC 版本已完整测试。
- `TUpdateClient` 的确认回调在循环线程内同步执行，回调必须响应 `TCancelToken`，
  否则 `Stop` 可能超时。
- 子进程继承当前进程环境变量（SDK 不提供 per-call env）。

## 开发与测试

修改协议字段时先改 schema，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```

`test.ps1` 使用仓库里的 `tools/fake-aly-client` 回放 `spec/fixtures/scenarios`，
需要本机有 Go 与 FPC。