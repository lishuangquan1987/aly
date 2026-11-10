---
title: 发布工具 aly-publish
description: aly-publish 的职责、CLI 与 GUI 的分工、全局参数、命令树、典型发布流程、配置文件与退出码语义。
---

# 发布工具 aly-publish

`aly-publish` 是发布侧命令行工具（Go + cobra）。它在一个**构建产物目录**中工作：把本地产物与服务端清单比对、把变更放入暂存区，再上传文件并登记一个新版本。文件与版本日志保存在服务端，终端上的 [aly-client](/docs/commands) 据此完成更新。

它不负责编译、打包，也不负责在服务端创建项目——后者是 `project create` 的职责。两个常见误解由此而来：`config init` 只写本地配置，`push` 只上传已暂存的文件。

## CLI 与 GUI

| 对比项 | `aly-publish`（CLI） | `AlyPublish.exe`（GUI） |
|---|---|---|
| 形态 | 单文件命令行程序 | Avalonia 桌面程序 |
| 适用场景 | CI、脚本、批量发布 | 人工日常发版 |
| 配置 | 直接读写当前目录的 `.updator/` | 调用 CLI，读写同一份配置 |
| 输出 | human 文本或 `--json` | 解析 CLI 的 `--json` 输出并展示进度 |
| 依赖 | 无 | 必须与 `aly-publish.exe` 放在同一目录 |
| 发布资产 | Windows / Linux / macOS 预构建 | 仓库发布工作流未构建，需从源码构建 |

GUI 的细节见[发布 GUI](/docs/publish/gui)。两者可以混用：GUI 只是 CLI 的调用方，`.updator/` 配置与暂存区完全一致。

## 全局参数

以下参数对所有子命令生效，写在子命令前或后都可以：

| 参数 | 类型 | 说明 |
|---|---|---|
| `--server` | string | 覆盖 `shared.json` 的 `server_url`，仅本次调用有效 |
| `--project` | string | 覆盖 `shared.json` 的 `project_name`，仅本次调用有效 |
| `--json` | bool | 输出单行 JSON；`push` 输出逐行进度 JSON |
| `--quiet` | bool | 抑制 human 文本；不影响 `--json` 的结构化输出 |

::: tip 工作目录决定配置
与 git 一样，`aly-publish` 始终读取**当前工作目录**下的 `.updator/`，没有 `--cwd` 之类的参数。发布前先 `cd` 到构建产物目录（例如 `dist/`）。
:::

## 命令树

```text
aly-publish
├── config                  # 本地配置（.updator/）
│   ├── init                #   初始化 shared.json + publish.json
│   ├── set                 #   设置 server.url / project.name
│   ├── set-array           #   增删 ignore / un_copy 数组
│   ├── get                 #   读取单个标量配置
│   ├── list                #   列出全部配置
│   └── path                #   显示 .updator/ 路径
├── project                 # 服务端项目生命周期
│   ├── list                #   列出未删除的项目
│   ├── create              #   创建项目（初始版本 V1.0.0）
│   ├── update              #   更新标题、强制更新与忽略规则
│   ├── delete              #   软删除项目
│   └── info                #   查看项目详情
├── status                  # 本地与服务端差异总览（并同步暂存区）
├── diff                    # 逐文件对比（仅人类可读输出）
├── add                     # 暂存变更（--all 或指定路径）
├── reset                   # 取消暂存
├── staged                  # 查看暂存区
├── push                    # 上传暂存文件并登记版本
├── log                     # 查看版本变更日志
└── server info             # 查看服务端系统信息
```

| 命令 | 作用 | 参考 |
|---|---|---|
| `config init/set/set-array/get/list/path` | 读写 `.updator/` 下的配置 | [配置命令](/docs/publish/commands-config) |
| `project list/create/update/delete/info` | 管理服务端项目 | [项目命令](/docs/publish/commands-project) |
| `status/diff/add/reset/staged/push/log/server info` | 比对、暂存、推送与日志 | [发布流程命令](/docs/publish/commands-workflow) |

当前版本没有 `push-all`、`publish`、`watch` 子命令；一次完整发布由 `status` → `add` → `push` 组合完成。

## 典型发布流程

```bash
# 1. 进入构建产物目录（.updator/ 会建在这里）
cd dist

# 2. 一次性初始化本地配置；--project 指向服务端项目名
aly-publish config init --project myapp --server http://192.168.1.100:2000

# 3. 服务端首次发布前注册项目（只需一次，可在任意目录执行）
aly-publish project create --name myapp --title "我的应用"

# 4. 日常发版三步
aly-publish status           # 这次改了什么
aly-publish add --all        # 暂存新增与修改（不含删除）
aly-publish push --version V1.0.1 --message "修复登录超时"
```

几点约束：

- `config init` **不会**创建服务端项目；`project create` **不会**写本地配置。两边通过 `project_name` 与 `--name` 保持一致。
- `push` 只能上传暂存区中的文件。暂存区为空时直接失败。
- `add --all` 只暂存 `new` 与 `modified`；本地删除的文件不会进入暂存区，由服务端在发布后固化的清单驱动客户端删除。

## 配置文件

| 文件 | 归属 | 是否上传 | 说明 |
|---|---|---|---|
| `.updator/shared.json` | CLI 与客户端共用 | **是** | 服务端地址、项目名、忽略规则、不复制规则 |
| `.updator/publish.json` | CLI 专有 | 否 | 当前为空对象，预留给 CLI 专属配置 |
| `.updator/staging/staged-files.json` | CLI 专有 | 否 | 暂存区条目，`add` 写入，`push` 读取 |

`shared.json` 会作为普通文件随版本上传，因此终端上的 `aly-client` 能继承同一份服务端地址、项目名和忽略规则：

```json
{
  "server_url": "http://192.168.1.100:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp"],
  "ignore_files": ["*.log", ".DS_Store"],
  "un_copy_folders": ["docs"],
  "un_copy_files": ["*.md"]
}
```

`publish.json` 目前固定写入 `{}`：

```json
{}
```

`staged-files.json` 是暂存条目的数组，记录加入暂存区时的 MD5 与大小：

```json
[
  {
    "relativePath": "DemoApp.dll",
    "status": "modified",
    "localMd5": "96c15c2bb2921193bf290df8cd85e2ba",
    "localSize": 90052
  }
]
```

扫描发布目录时，`.updator/publish.json` 与 `.updator/staging/` 总被跳过，`.updator/shared.json` 则会被上传。`un_copy_folders` / `un_copy_files` 只影响客户端应用更新时的复制行为，不参与发布扫描。

## 退出码与输出契约

| 场景 | 退出码 | 判断依据 |
|---|---|---|
| 命令成功 | 0 | `isSuccess` 为 `true` |
| 业务失败：配置缺失、网络错误、服务端拒绝、暂存区为空、版本冲突 | **0** | `isSuccess` 为 `false`，`errorMsg` 给出原因 |
| 用法错误：未知命令/参数、缺少命令级必填参数、参数类型错误 | 1 | cobra 打印 `Error: ...` 与 usage |

**业务失败也返回 0**，这是脚本接入时最容易踩的坑。请解析输出中的 `isSuccess`，不要只看退出码：

```json
{"isSuccess":false,"errorMsg":"暂存区为空，请先 add 文件","data":null}
```

- human 模式下，业务错误写到 stderr，前缀为 `Error:`。
- `--json` 模式下，错误 JSON 写到 stdout，成功与失败都是单行 JSON。
- `--quiet` 只抑制 human 文本，不抑制 `--json`。
- 退出码 1 只表示命令没有真正执行（参数层面就被拒绝），不代表服务端返回了业务失败。

## 版本号规则

服务端在创建项目时写入初始版本 **V1.0.0**，并同时插入一条内容为“第一次创建”的变更日志。`push` 的新版本必须**严格大于**该项目已发布的最新版本（服务端比较全部未删除日志中的最大版本），否则拒绝：

```json
{"isSuccess":false,"errorMsg":"创建版本失败: POST /api/project/publish_version: 版本 V1.0.0 不大于已发布的最新版本 V1.0.1（防倒退保护；如需回退内容，请发布更高的版本号）（文件已上传，可重试 push）","data":null}
```

由此得到两条实践规则：

1. **首次发布必须从 V1.0.1 起**，V1.0.0 已被项目创建时的初始日志占用。
2. 确实需要回退内容时，发布一个**更高**的版本号，把旧文件重新上传；版本号本身不回退。

版本格式由服务端校验：`^[vV]?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`，长度不超过 50。`V`/`v` 前缀可选；比较时忽略前缀与构建元数据，预发布版本低于同号正式版（`V1.0.1-beta` < `V1.0.1`）。

## 下一步

- 第一次使用：从[下载与安装](/docs/download)取得 CLI，按[快速开始](/docs/getting-started)跑通服务端与首个版本。
- 配置细节：[配置命令](/docs/publish/commands-config)
- 服务端项目：[项目命令](/docs/publish/commands-project)
- 日常发布：[发布流程命令](/docs/publish/commands-workflow)
- 桌面操作：[发布 GUI](/docs/publish/gui)