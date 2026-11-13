# AlyClient.TypeScriptSDK

`aly-client` 的 Node.js / Electron 宿主 SDK。零运行时依赖，封装更新器的子进程协议并提供托管更新生命周期。

- 要求：Node.js **>= 18**（Electron 主进程同样可用）
- 更新器：`UpdateFolder/aly-client.exe`（Windows）或 `UpdateFolder/aly-client`（Linux）
- 模型：`src/models.ts` 由 `spec/aly-protocol.schema.json` 生成，**不要手改**

## 安装

当前未发布到 npm。从本仓库安装：

```bash
npm install "<repo>/client/aly-client-sdk/AlyClient.TypeScriptSDK"
```

或把整个目录拷进你的工程，用本地路径依赖。

## 快速开始（托管生命周期）

```ts
import { AlyUpdateClient } from "aly-client-sdk";

const client = new AlyUpdateClient({
  clientPath: "C:/App/UpdateFolder/aly-client.exe", // 默认 <cwd>/../UpdateFolder/aly-client[.exe]
  mustCloseProcessNames: "YourApp",                  // 不带 .exe，多个用逗号分隔
  onStatusChange: (status, message) => {
    console.log(`[${status}] ${message}`);
  },
  onError: (message) => {
    console.error("update error:", message);
  },
  confirmDownload: async (version) => {
    // 非强制更新时询问用户；返回即确认
  },
  confirmApply: async (version) => {
    // 询问用户是否立即重启
  },
});

client.start();

// 退出前停止轮询（不会结束 apply_update 拉起的进程）
await client.dispose();
```

行为与 C# 的 `AlyUpdateClient` 一致：

- 启动时先尝试 `check_self_update` / `self_update`（失败只记录错误，不阻塞主循环）；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 随机抖动），`dispose()` 会中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- `has_update=true` 且 `need_download_update=false`（本地已下载待应用、applying 崩溃恢复、回滚续跑）时**直接调用 `apply_update`**，不下载、不询问（恢复场景自愈）；
- 服务器不可达且本地无在途恢复状态时 `check_update` 返回 `has_update=false`，已下载产物保留；
- `apply_update` 以独立进程拉起，立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

所有函数返回 `AlyResponse<T>`，业务失败**不会 throw**：

```ts
import { checkUpdate, downloadUpdate, applyUpdate } from "aly-client-sdk";

const check = await checkUpdate("/path/to/aly-client");
if (check.isSuccess && check.data.has_update) {
  if (check.data.need_download_update) {
    const downloaded = await downloadUpdate(
      "/path/to/aly-client",
      (progress) => console.log(progress.file, progress.ratio),
    );
    if (!downloaded.isSuccess) {
      throw new Error(downloaded.errorMsg);
    }
  }
  // need_download_update=false：本地已下载待应用 / applying 崩溃恢复 / 回滚续跑，
  // 直接 apply 完成恢复，不下载、不询问。
  await applyUpdate("/path/to/aly-client", { mustCloseProcessNames: "YourApp" });
}
```

| 函数 | 对应命令 | 默认超时 |
|---|---|---|
| `checkUpdate` | `check_update` | 30s |
| `checkSelfUpdate` | `check_self_update` | 30s |
| `selfUpdate` | `self_update` | 30s |
| `listRollbackVersions` | `list_rollback_versions` | 30s |
| `checkDiff` | `check_diff` | 30s |
| `clean` | `clean` | 30s |
| `rollback(client, version)` | `rollback` | 600s |
| `downloadUpdate` | `download_update` | 无总时长；停流看门狗 1800s |
| `applyUpdate` | `apply_update` | 不等待 |

`downloadUpdate` 的进度是**文件计数**粒度（只统计 `DONE` 行），但 `DownloadProgressInfo` 也带 `downloadedBytes` / `totalBytes` / `speedBps` 字节字段。

## 字段命名

生成模型保留协议线格式（snake_case），避免额外映射层：

```ts
check.data.has_update;
check.data.new_version;
check.data.release_notes;
```

## 与 C# SDK 的差异

| 方面 | C# SDK | TypeScript SDK |
|---|---|---|
| 托管循环启动 | 构造函数自动启动 | 显式 `start()` / `dispose()` |
| 事件模型 | `event` + 同步处理器 | 回调函数（可 async） |
| 更新器自更新兜底 | 命令失败时本地复制兜底 | 只委派 `self_update`，不做本地替换 |
| GUI 启动器 | `AlyUpdateUIClient` | 未提供（直接拉起 `aly-client-ui.exe` 亦可） |

托管循环的恢复语义与 C# 一致：`has_update=true` 且 `need_download_update=false` 时直接 `apply_update`，不下载、不询问；差异仅在上表列出的方面。

## 开发与测试

```bash
cd client/aly-client-sdk/AlyClient.TypeScriptSDK
npm ci
npm test        # tsc 构建 + node:test；会用仓库里的 Go 假客户端回放 fixtures
```

测试需要本机有 Go（用于构建 `tools/fake-aly-client`）与仓库的 `spec/fixtures/scenarios`。
修改协议字段时先改 `spec/aly-protocol.schema.json`，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```