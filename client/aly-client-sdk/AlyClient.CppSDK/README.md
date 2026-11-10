# AlyClient.CppSDK

`aly-client` 的 C++ 宿主 SDK。面向 Windows 桌面（含 XP 遗留环境），
零第三方依赖，使用 Win32 API + C++11。

- 更新器：`UpdateFolder/aly-client.exe`
- 模型：`include/aly/models_gen.h` 由 `spec/aly-protocol.schema.json` 生成，**不要手改**
- 平台：Windows XP SP3+（源码级）；编译工具：MinGW-w64

## XP 说明

代码只使用 XP 已有的 Win32 API（`CreateProcessW`、`CreatePipe`、`CRITICAL_SECTION`、
`CreateThread`、`GetTickCount`），不依赖 `std::thread` / `std::filesystem`。
发布到真实 XP 时请用 **i686 MinGW-w64 + MSVCRT 运行库**构建（UCRT 运行库不支持 XP）：

```bash
i686-w64-mingw32-g++ -std=c++11 -O2 -D_WIN32_WINNT=0x0501 \
  -Iinclude src/json.cpp src/api.cpp src/client.cpp tests/conformance.cpp \
  -o out/aly-cpp-conformance.exe -static -static-libgcc -static-libstdc++
```

本机脚本默认使用已安装的 x64 MinGW 工具链做功能验证。

## 构建与测试

```powershell
cd client/aly-client-sdk/AlyClient.CppSDK
.\build.ps1          # 生成 out\aly-cpp-conformance.exe
.\test.ps1           # 构建假客户端并回放 fixtures
```

或 MinGW bash：

```bash
./build.sh
```

## 快速开始（托管生命周期）

```cpp
#include "aly/client.h"

aly::Api api("C:\\App\\UpdateFolder\\aly-client.exe");
aly::UpdateClient client(api);

client.setOnStatus([](const std::string& status, const std::string& message) {
    std::cout << "[" << status << "] " << message << std::endl;
});
client.setOnError([](const std::string& message) {
    if (!message.empty()) {
        std::cerr << "update error: " << message << std::endl;
    }
});
client.setConfirmDownload([](const std::string& version, const aly::CancelToken& token) {
    // 非强制更新时询问用户；返回即确认。
    // 阻塞时轮询 token，保证 stop 能及时退出。
});

client.start();

// 退出前
client.stop(5000);
```

行为与其它 SDK 一致：

- 启动时先尝试 `check_self_update` / `self_update`；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 抖动），`stop` 可中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- 服务器不可达时 `has_update=false`，已下载产物保留；
- `apply_update` 以独立进程拉起后立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

业务失败不抛异常，返回 `Response<T>{ isSuccess, errorMsg, data }`：

```cpp
aly::Api api("C:/App/UpdateFolder/aly-client.exe");
aly::Response<aly::CheckUpdateData> check = api.checkUpdate();
if (check.isSuccess && check.data.has_value()) {
    const aly::CheckUpdateData& data = check.data.get();
    if (data.has_update && data.need_download_update) {
        aly::DownloadOptions options;
        options.onProgress = [](const aly::DownloadProgressInfo& info) {
            std::cout << info.file << " " << info.ratio << std::endl;
        };
        aly::Response<bool> downloaded = api.downloadUpdate(options);
        if (downloaded.isSuccess) {
            api.applyUpdate(aly::CloseOptions::withNames("YourApp"));
        }
    }
}
```

| 方法 | 对应命令 | 默认超时 |
|---|---|---|
| `checkUpdate(cancel)` | `check_update` | 30s |
| `checkSelfUpdate(cancel)` | `check_self_update` | 30s |
| `selfUpdate(cancel)` | `self_update` | 30s |
| `listRollbackVersions()` | `list_rollback_versions` | 30s |
| `checkDiff()` | `check_diff` | 30s |
| `clean(options)` | `clean` | 30s |
| `rollback(version, options)` | `rollback` | 600s |
| `downloadUpdate(options)` | `download_update` | 无总时长；停流看门狗 1800s |
| `applyUpdate(options)` | `apply_update` | 不等待 |

下载进度是**文件计数**粒度（只统计 `DONE` 行），`DownloadProgressInfo` 同时带字节字段。

## 字段命名

生成模型保持协议 snake_case；可选字段为 `aly::Optional<T>`：

```cpp
data.has_update;
data.new_version.value_or("");
data.force_update.has_value() && data.force_update.get();
data.release_notes.value_or(std::vector<std::string>());
```

未知字段被忽略；未知状态字符串可正常读取（`status` 是 `std::string`）。

## 已知边界

- 仅 Windows；不提供 `aly-client-ui` 启动器。
- 不做自更新本地替换兜底（只委派 `self_update`）。
- `UpdateClient` 的确认回调在循环线程内同步执行，回调必须响应 `CancelToken`，
  否则 `stop` 可能超时。
- `Api::defaultPath()` 以宿主 exe 目录为基准，生产环境建议显式传路径。

## 开发与测试

修改协议字段时先改 schema，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```

`test.ps1` 使用仓库里的 `tools/fake-aly-client` 回放 `spec/fixtures/scenarios`，
需要本机有 Go 与 MinGW g++。