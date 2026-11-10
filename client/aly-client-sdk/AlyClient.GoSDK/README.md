# AlyClient.GoSDK

`aly-client` 的 Go 宿主 SDK。零第三方依赖，封装更新器子进程协议，并提供托管更新生命周期。

- 要求：Go **>= 1.21**
- 更新器：`UpdateFolder/aly-client.exe`（Windows）/ `UpdateFolder/aly-client`（Linux）
- 模型：`models_gen.go` 由 `spec/aly-protocol.schema.json` 生成，**不要手改**

## 安装

仓库尚未给该模块打 tag，可直接用伪版本，或在你的 `go.mod` 里用本地路径：

```go
require github.com/lishuangquan1987/aly/client/aly-client-sdk/AlyClient.GoSDK v0.0.0

replace github.com/lishuangquan1987/aly/client/aly-client-sdk/AlyClient.GoSDK => "../aly/client/aly-client-sdk/AlyClient.GoSDK"
```

## 快速开始（托管生命周期）

```go
package main

import (
	"context"
	"fmt"
	"log"

	alyclient "github.com/lishuangquan1987/aly/client/aly-client-sdk/AlyClient.GoSDK"
)

func main() {
	client := alyclient.NewUpdateClient("C:/App/UpdateFolder/aly-client.exe")
	client.MustCloseProcessNames = "YourApp"
	client.OnStatusChange = func(status alyclient.Status, message string) {
		log.Printf("[%s] %s", status, message)
	}
	client.OnError = func(message string) {
		if message != "" {
			log.Printf("update error: %s", message)
		}
	}
	client.ConfirmDownload = func(ctx context.Context, version string) {
		// 非强制更新时询问用户；函数返回即确认。阻塞时应响应 ctx。
	}
	client.ConfirmApply = func(ctx context.Context, version string) {
		// 询问是否重启
	}

	client.Start()
	defer client.Stop() // 停止轮询；不会结束 apply_update 拉起的进程

	select {} // 你的主程序
}
```

行为与 C# / TypeScript SDK 一致：

- 启动时先尝试 `check_self_update` / `self_update`；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 抖动），`Stop` 可中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- 服务器不可达时 `has_update=false`，已下载产物保留；
- `ApplyUpdate` 以独立进程拉起后立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

业务失败不返回 `error`，而是 `Response.IsSuccess=false` + `ErrorMsg`：

```go
api := alyclient.NewAPI("/path/to/aly-client")
ctx := context.Background()

check := api.CheckUpdate(ctx)
if check.IsSuccess && check.Data.HasUpdate && check.Data.NeedDownloadUpdate {
	downloaded := api.DownloadUpdate(ctx, alyclient.DownloadOptions{
		OnProgress: func(info alyclient.DownloadProgressInfo) {
			fmt.Println(info.File, info.Ratio)
		},
	})
	if downloaded.IsSuccess {
		api.ApplyUpdate(alyclient.CloseOptions{MustCloseProcessNames: "YourApp"})
	}
}
```

| 方法 | 对应命令 | 默认超时 |
|---|---|---|
| `CheckUpdate` | `check_update` | 30s |
| `CheckSelfUpdate` | `check_self_update` | 30s |
| `SelfUpdate` | `self_update` | 30s |
| `ListRollbackVersions` | `list_rollback_versions` | 30s |
| `CheckDiff` | `check_diff` | 30s |
| `Clean` | `clean` | 30s |
| `Rollback` | `rollback` | 600s |
| `DownloadUpdate` | `download_update` | 无总时长；停流看门狗 1800s |
| `ApplyUpdate` | `apply_update` | 不等待 |

下载进度是**文件计数**粒度（只统计 `DONE` 行），`DownloadProgressInfo` 同时带字节字段。

## 字段命名

生成模型使用 `json` tag 保留协议线格式，Go 字段为 PascalCase：

```go
check.Data.HasUpdate     // json:"has_update"
check.Data.NewVersion    // json:"new_version"
check.Data.ReleaseNotes  // json:"release_notes"
```

可选字段缺省时是 Go 零值（`false` / `""` / `nil`）；未知字段由 `encoding/json` 忽略。

## 与 C# SDK 的差异

| 方面 | C# SDK | Go SDK |
|---|---|---|
| 托管循环启动 | 构造函数自动启动 | `Start()` / `Stop()` |
| 事件模型 | `event` | 回调函数 |
| 确认回调 | 同步 | 接收 `context.Context`，应响应取消 |
| 更新器自更新兜底 | 命令失败时本地复制兜底 | 只委派 `self_update`，不做本地替换 |
| GUI 启动器 | `AlyUpdateUIClient` | 未提供 |

## 开发与测试

```bash
cd client/aly-client-sdk/AlyClient.GoSDK
go vet ./...
go test ./... -count=1
```

测试用仓库里的 `tools/fake-aly-client` 回放 `spec/fixtures/scenarios`，需要本机有 Go。
修改协议字段时先改 schema，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```