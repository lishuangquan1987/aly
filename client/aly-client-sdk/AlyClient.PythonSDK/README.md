# AlyClient.PythonSDK

`aly-client` 的 Python 宿主 SDK。零第三方依赖，封装更新器子进程协议，并提供托管更新生命周期。

- 要求：Python **>= 3.9**
- 更新器：`UpdateFolder/aly-client.exe`（Windows）/ `UpdateFolder/aly-client`（Linux）
- 模型：`alyclient/models_gen.py` 由 `spec/aly-protocol.schema.json` 生成，**不要手改**

## 安装

未发布到 PyPI。从本仓库安装：

```bash
pip install -e "<repo>/client/aly-client-sdk/AlyClient.PythonSDK"
```

或把 `alyclient/` 目录拷进你的工程。

## 快速开始（托管生命周期）

```python
import threading

from alyclient import UpdateClient

client = UpdateClient("C:/App/UpdateFolder/aly-client.exe")
client.must_close_process_names = "YourApp"
client.on_status_change = lambda status, message: print(f"[{status}] {message}")
client.on_error = lambda message: print("update error:", message) if message else None

def confirm_download(version, stop_event):
    # 非强制更新时询问用户；函数返回即确认。
    # 阻塞时请等待 stop_event，保证 stop() 能及时退出。
    ...

client.confirm_download = confirm_download
client.start()

# 退出前
client.stop()
```

行为与 C# / TypeScript / Go SDK 一致：

- 启动时先尝试 `check_self_update` / `self_update`；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 抖动），`stop()` 可中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- 服务器不可达时 `has_update=false`，已下载产物保留；
- `apply_update` 以独立进程拉起后立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

业务失败不抛异常，返回 `Response.is_success=False` + `error_msg`：

```python
from alyclient import API, CloseOptions

api = API("/path/to/aly-client")
check = api.check_update()
if check.is_success and check.data.has_update and check.data.need_download_update:
    downloaded = api.download_update(
        on_progress=lambda info: print(info.file, info.ratio),
    )
    if downloaded.is_success:
        api.apply_update(CloseOptions(must_close_process_names="YourApp"))
```

| 方法 | 对应命令 | 默认超时 |
|---|---|---|
| `check_update()` | `check_update` | 30s |
| `check_self_update()` | `check_self_update` | 30s |
| `self_update()` | `self_update` | 30s |
| `list_rollback_versions()` | `list_rollback_versions` | 30s |
| `check_diff()` | `check_diff` | 30s |
| `clean()` | `clean` | 30s |
| `rollback(version)` | `rollback` | 600s |
| `download_update()` | `download_update` | 无总时长；停流看门狗 1800s |
| `apply_update()` | `apply_update` | 不等待 |

下载进度是**文件计数**粒度（只统计 `DONE` 行），`DownloadProgressInfo` 同时带字节字段。

## 字段命名

生成模型直接使用协议线格式（snake_case）：

```python
check.data.has_update
check.data.new_version
check.data.release_notes
```

可选字段缺省时为 `None`；未知字段被 `from_dict` 忽略。

## 与 C# SDK 的差异

| 方面 | C# SDK | Python SDK |
|---|---|---|
| 托管循环启动 | 构造函数自动启动 | `start()` / `stop()` |
| 事件模型 | `event` | 回调函数 |
| 确认回调 | 同步 | 同步，接收 `threading.Event`（应响应取消） |
| 更新器自更新兜底 | 命令失败时本地复制兜底 | 只委派 `self_update`，不做本地替换 |
| GUI 启动器 | `AlyUpdateUIClient` | 未提供 |

## 开发与测试

```bash
cd client/aly-client-sdk/AlyClient.PythonSDK
python -m unittest discover -s tests -v
```

测试用仓库里的 `tools/fake-aly-client` 回放 `spec/fixtures/scenarios`，需要本机有 Go。
修改协议字段时先改 schema，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```