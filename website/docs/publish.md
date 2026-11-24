---
title: 打包与发布
description: aly-publish 的日常发版流程——命令树、status → add → push、版本号规则与退出码语义；第一次跑通看「发布第一个版本」。
---

# 打包与发布

`aly-publish` 在**构建产物目录**里工作：把本地产物与服务端清单比对、把变更放进暂存区，再上传文件并登记一个新版本。服务端保存文件与版本日志，终端上的 [aly-client](/docs/commands) 据此完成更新。

它不负责编译、打包，也不负责在服务端创建项目——后者是 `project create` 的职责。两个常见误解由此而来：`config init` 只写本地配置，`push` 只上传已暂存的文件。

第一次跑通请看[发布第一个版本](/docs/start/publish)；桌面界面见[发布 GUI](/docs/publish/gui)。

## CLI 与 GUI

| 对比项 | `aly-publish`（CLI） | `AlyPublish.exe`（GUI） |
|---|---|---|
| 形态 | 单文件命令行程序 | Go/MyGo native 桌面程序（默认；C#/Avalonia 为历史实现） |
| 适用场景 | CI、脚本、批量发布 | 人工日常发版 |
| 配置 | 直接读写当前目录的 `.updator/` | 调用 CLI，读写同一份配置 |
| 输出 | human 文本或 `--json` | 解析 CLI 的 `--json` 输出并展示进度 |
| 依赖 | 无（CLI 自己就是包内 `publish-cli/<rid>/` 那一份） | 必须能找到同一安装包内的 `publish-cli/<cli_rid>/aly-publish(.exe)` |
| 获取方式 | 随 AlyPublish 安装包分发（`publish-cli/<rid>/`） | 装 AlyPublish 安装包 |

GUI 只是 CLI 的调用方，`.updator/` 配置与暂存区完全一致，两者可以混用。

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
aly-publish add --all        # 暂存新增 / 修改 / 删除
aly-publish push --version V1.0.1 --message "修复登录超时"
```

几点约束：

- `config init` **不会**创建服务端项目；`project create` **不会**写本地配置。两边通过 `project_name` 与 `--name` 保持一致。
- `push` 只能上传暂存区中的文件；暂存区为空时直接失败。
- `add --all` 暂存 `new` / `modified` / `deleted`；本地删除的文件在 `push` 时调用 `file/delete_file` 同步删除，随后从发布清单中消失，客户端据此删除本地旧文件。

## 配置文件

`.updator/` 下的三个文件——`shared.json`（CLI 与客户端共用、随版本上传）、`publish.json`、`staging/staged-files.json`——的字段、位置与生效时机，见[配置文件参考](/docs/client/config)。

`shared.json` 会随版本上传，因此终端上的 `aly-client` 能继承同一份服务端地址、项目名和忽略规则；改规则后发一次版本即可生效。

## 版本号规则

服务端在创建项目时写入初始版本 **V1.0.0**，并同时插入一条内容为“第一次创建”的变更日志。`push` 的新版本必须**严格大于**该项目已发布的最新版本（服务端比较全部未删除日志中的最大版本），否则拒绝：

```json
{"isSuccess":false,"errorMsg":"创建版本失败: POST /api/project/publish_version: 版本 V1.0.0 不大于已发布的最新版本 V1.0.1（防倒退保护；如需回退内容，请发布更高的版本号）（文件已上传，可重试 push）","data":null}
```

由此得到两条实践规则：

1. **首次发布必须从 V1.0.1 起**，V1.0.0 已被项目创建时的初始日志占用。
2. 确实需要回退内容时，发布一个**更高**的版本号，把旧文件重新上传；版本号本身不回退。

版本格式由服务端校验：`^[vV]?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`，长度不超过 50。`V`/`v` 前缀可选；比较时忽略前缀与构建元数据，预发布版本低于同号正式版（`V1.0.1-beta` < `V1.0.1`）。

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

- human 模式下，业务错误写到 stderr，前缀为 `Error:`；
- `--json` 模式下，成功与失败都是单行 JSON；
- `--quiet` 只抑制 human 文本，不抑制 `--json`；
- 退出码 1 只表示命令没有真正执行（参数层面就被拒绝），不代表服务端返回了业务失败。

更新器（`aly-client`）的退出码与进度行语义是另一套，见[命令参考 · 统一约定](/docs/commands#统一约定)。

## 下一步

- 配置细节：[配置命令](/docs/publish/commands-config) · [配置文件参考](/docs/client/config)
- 服务端项目：[项目命令](/docs/publish/commands-project)
- 日常发布：[发布流程命令](/docs/publish/commands-workflow)
- 桌面操作：[发布 GUI](/docs/publish/gui) · 包内结构：[发行包结构与分发](/docs/guide/package)
