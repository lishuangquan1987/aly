# 快速开始

三个角色走完一次完整更新：**起服务端** → **发布一个版本** → **客户端检查并应用**。

## 1. 启动服务端

```bash
./aly-server -p 2000
```

首次运行自动创建 SQLite 数据库（`aly.db`），上传的文件按项目存放在 `data/{project_name}/` 下。服务端二进制是静态编译的单文件，没有任何外部依赖，直接拷到服务器上运行即可。

Linux 服务器上：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aly-server-linux-amd64 .
./aly-server-linux-amd64 -p 2000
```

## 2. 发布第一个版本

在**待发布目录**（也就是要分发给用户的产物目录，如 `./dist`）里初始化发布配置：

```bash
cd ./dist

# 初始化项目配置（创建 .updator/ 目录）
aly-publish config init --server http://localhost:2000 --project myapp

# 查看哪些文件有变化
aly-publish status

# 暂存全部变更并推送
aly-publish add --all
aly-publish push --version V1.0.0 --message "首个版本"
```

`push` 会扫描目录、把新增/变更的文件上传到服务端（大文件自动走分片上传），并创建版本变更日志。之后每次发版只需重复 `status → add --all → push` 三条命令。

::: tip 配置随版本分发
发布配置 `.updator/shared.json`（服务端地址、项目名、忽略规则）会随每个版本上传。客户端下载更新时，新配置会替换旧配置——**改服务端地址这类变更会随下一次更新自动生效**。
:::

## 3. 客户端检查并应用更新

在**终端用户机器**上，宿主应用按三步调用 `aly-client`（位于 `UpdateFolder/`）：

```bash
# 第 1 步 — 检查更新
aly-client.exe check_update
# → {"isSuccess":true,"data":{"has_update":true,"need_download_update":true,
#    "new_version":"1.0.1","force_update":false}}

# 第 2 步 — 下载（只下载变更文件，SHA-256 校验）
aly-client.exe download_update

# 第 3 步 — 应用（关闭应用 → 原子替换 → 重启）
aly-client.exe apply_update
```

宿主程序根据 `force_update` 决定交互方式：`true` 时自动下载并提示重启；`false` 时先询问用户。

## 4. 终端机器上的目录布局

```text
PackageFolder/
├── ApplicationFolder/            # 当前生效的应用（最新版本）
│   ├── your_app.exe
│   └── .updator/shared.json     # server_url、project_name、忽略规则
├── ApplicationFolder_V1.0.0/     # 上一版本快照（供回滚）
├── UpdateFolder/
│   ├── aly-client.exe            # 更新器本体
│   ├── aly-client-ui.exe         # 可选 GUI 前端
│   ├── client.json               # main_exe_relative_path、must_close_process_name 等
│   └── version.json              # 版本状态机（自动维护，勿手改）
```

`client.json` 是宿主需要关心的唯一客户端配置：

```json
{
  "main_exe_relative_path": "ApplicationFolder/your_app.exe",
  "must_close_process_name": ["your_app"]
}
```

- `main_exe_relative_path`：主程序相对包根目录的路径，`apply_update` 应用完成后会拉起它
- `must_close_process_name`：应用更新前必须退出的进程名（避免文件占用）

## 5. 用 C# SDK 省掉手工编排

.NET 宿主不必手写上面三步，NuGet 包 `AlyClient.CSharpSDK` 会自动完成检查、下载、应用与自更新的编排：

```csharp
var client = new AlyUpdateClient(updateFolder, mainFolder, ...);
if ((await client.CheckUpdateAsync()).Data.HasUpdate)
{
    await client.DownloadUpdateAsync();
    await client.ApplyUpdateAsync();   // 应用会被关闭、替换并重启
}
```

详见 [C# SDK 集成](/reference/sdk)。

## 6. 发布 GUI（可选）

不想用命令行？`aly-publish-gui`（Avalonia 桌面程序）包装了 publish-cli 的全部能力：选目录、看差异、填版本号、一键发布。

```bash
cd publish/publish-gui && dotnet build
```
