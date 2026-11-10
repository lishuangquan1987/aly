# AlyClient.RustSDK

`aly-client` 的 Rust 宿主 SDK。模型由 `spec/aly-protocol.schema.json` 生成，
API 与托管生命周期行为对齐 C# / TypeScript / Go / Python / Java SDK。

- 更新器：`UpdateFolder/aly-client.exe`（Windows）/ `UpdateFolder/aly-client`（Linux）
- 依赖：`serde`、`serde_json`
- 模型：`src/models_gen.rs` 由生成器产出，**不要手改**

## 安装

未发布到 crates.io。通过 path 依赖：

```toml
[dependencies]
alyclient = { path = "path/to/aly/client/aly-client-sdk/AlyClient.RustSDK" }
```

或在 SDK 目录本地验证：

```bash
cd client/aly-client-sdk/AlyClient.RustSDK
cargo build
```

## 快速开始（托管生命周期）

```rust
use alyclient::{Api, Status, UpdateClient};
use std::time::Duration;

let api = Api::new("C:/App/UpdateFolder/aly-client.exe");
let mut client = UpdateClient::new(api)
    .on_status(|status, message| println!("[{status}] {message}"))
    .on_error(|message| {
        if !message.is_empty() {
            eprintln!("update error: {message}");
        }
    })
    .confirm_download(|version, token| {
        // 非强制更新时询问用户；返回即确认。
        // 阻塞时应轮询 token，保证 stop 能及时退出。
        let _ = (version, token);
    });

client.start();

// 退出前
let _ = client.stop(Duration::from_secs(5));
```

行为与其它 SDK 一致：

- 启动时先尝试 `check_self_update` / `self_update`；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 抖动），`stop` 可中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- 服务器不可达时 `has_update=false`，已下载产物保留；
- `apply_update` 以独立进程拉起后立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

业务失败不返回 `Err`，而是 `Response { is_success: false, error_msg, .. }`：

```rust
use alyclient::{Api, CloseOptions, DownloadOptions, DEFAULT_DOWNLOAD_STALL_TIMEOUT};

let api = Api::new("/path/to/aly-client");
let check = api.check_update(None);
if check.is_success {
    if let Some(data) = &check.data {
        if data.has_update && data.need_download_update {
            let mut on_progress = |info: alyclient::DownloadProgressInfo| {
                println!("{} {:.0}%", info.file, info.ratio * 100.0);
            };
            let downloaded = api.download_update(DownloadOptions {
                stall_timeout: DEFAULT_DOWNLOAD_STALL_TIMEOUT,
                on_progress: Some(&mut on_progress),
                cancel: None,
            });
            if downloaded.is_success {
                api.apply_update(&CloseOptions::with_names("YourApp"));
            }
        }
    }
}
```

| 方法 | 对应命令 | 默认超时 |
|---|---|---|
| `check_update(cancel)` | `check_update` | 30s |
| `check_self_update(cancel)` | `check_self_update` | 30s |
| `self_update(cancel)` | `self_update` | 30s |
| `list_rollback_versions()` | `list_rollback_versions` | 30s |
| `check_diff()` | `check_diff` | 30s |
| `clean(options)` | `clean` | 30s |
| `rollback(version, options)` | `rollback` | 600s |
| `download_update(options)` | `download_update` | 无总时长；停流看门狗 1800s |
| `apply_update(options)` | `apply_update` | 不等待 |

下载进度是**文件计数**粒度（只统计 `DONE` 行），`DownloadProgressInfo` 同时带字节字段。

## 字段命名

生成模型保持协议 snake_case；可选字段为 `Option<T>`：

```rust
data.has_update;
data.new_version;      // Option<String>
data.force_update;     // Option<bool>
data.release_notes;    // Option<Vec<String>>
```

未知字段被 serde 忽略；未知的状态字符串可正常读取（`status` 是 `String`，
常量在 `DownloadProgress::STATUS_*`）。

## 已知边界

- `Api::default_path()` 以当前可执行文件目录为基准；测试/子进程环境请显式传路径。
- 不提供 `aly-client-ui` 启动器；不做自更新本地替换兜底（只委派 `self_update`）。
- `UpdateClient` 是 `start` 一次的生命周期对象；停止后如需重新开始，请新建实例并重新挂回调。

## 开发与测试

```bash
cd client/aly-client-sdk/AlyClient.RustSDK
cargo test
```

测试用仓库里的 `tools/fake-aly-client` 回放 `spec/fixtures/scenarios`，需要本机有 Go
（测试会自行 `go build` 假客户端）。修改协议字段时先改 schema，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```