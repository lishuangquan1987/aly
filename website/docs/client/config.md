---
title: 客户端配置参考
description: client.json、.updator/shared.json、version.json、publish.json 与暂存文件的字段、位置、生效时机和优先级。
---

# 客户端配置参考

客户端涉及三份核心配置，另有两份发布侧本地文件容易混淆：

| 文件 | 位置 | 读写方 | 随版本分发 |
|---|---|---|---|
| `client.json` | `UpdateFolder/client.json` | 更新器读取；部署时手写 | 否 |
| `shared.json` | `MainFolder/.updator/shared.json` | `aly-publish` 与 `aly-client` 共用 | 是，上传到服务端并随更新下载 |
| `version.json` | `UpdateFolder/version.json` | 更新器读写 | 否，本机状态 |
| `publish.json` | 发布目录 `.updator/publish.json` | `aly-publish` 专有 | 否，本地不上传 |
| `staged-files.json` | 发布目录 `.updator/staging/staged-files.json` | `aly-publish` 专有 | 否，本地暂存 |

`UpdateFolder` 是更新器所在目录，`MainFolder` 是当前运行目录（由 `client.json` 推导）。部署布局见[更新器概览](/docs/client/)。

## client.json

### 位置与作用

位于 `UpdateFolder/client.json`，与 `aly-client` 可执行文件同级。更新器每个命令都先读取它，用它定位主程序目录和需要关闭的进程。

### 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `main_exe_relative_path` | string | 是 | — | 主程序可执行文件相对 `UpdateFolder` 的路径 |
| `must_close_process_name` | string[] | 否 | `[]` | 更新/回滚前需要关闭的进程名，通常不带 `.exe` |

`main_exe_relative_path` 的规则：

- 基准目录是 **`UpdateFolder`**，不是 `PackageFolder`，也不是当前工作目录。
- 更新器取该路径的父目录作为 `MainFolder`；文件名本身只用于启动主程序。
- 推荐写成 `../ApplicationFolder/YourApp.exe`，对应 `MainFolder = PackageFolder/ApplicationFolder`。
- Linux 上会把反斜杠归一为正斜杠，因此 Windows 打包工具写出的 `..\ApplicationFolder\YourApp.exe` 可以直接复用。
- 该字段必须指向真实的主程序文件；为空会导致 `MainFolder` 推导错误，更新器不会替你猜测或纠正。

`must_close_process_name` 的规则：

- 填进程名，不填完整路径；Windows 上带不带 `.exe` 都能匹配（大小写不敏感）。
- `apply_update` 会先发 `WM_CLOSE` 请求正常退出，短暂等待后强杀存活进程；`rollback` 同样先优雅关闭。
- 未列入的目录占用者由 rename 失败后的探测逻辑处理，届时可能被直接强杀。宿主应把自身进程名写进这里，保证有机会保存数据、停止监听。

### 示例

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp", "YourApp.PluginHost"]
}
```

### 边界与错误

- `client.json` 缺失或 JSON 损坏时，所有更新器命令在加载配置阶段失败，返回 `load config: ...`。
- 未知字段会被忽略，允许部署工具附加注释性字段（不保证未来语义）。
- GUI 只读取 `main_exe_relative_path` 来推导主程序目录，并据此读取 `shared.json` 的 `project_name` 作为标题。
- `--main-exe-path` 可临时覆盖路径；`--must-close-process-name` 会**追加**到列表，而不是替换。

## .updator/shared.json

### 位置与作用

位于 `MainFolder/.updator/shared.json`，由 `aly-publish` 在发布目录中维护并随版本上传。客户端下载更新后，新版本的 `shared.json` 会替换旧文件，因此规则变更可以随版本自动下发。

客户端在下载/检查阶段读取**当前主目录**的 `shared.json`；应用更新后，新主目录里的 `shared.json` 才生效。崩溃恢复时若主目录已被改名走，客户端会尝试从版本目录中读取可用的 `shared.json`。

### 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `server_url` | string | 联网命令必填 | 空 | 服务端地址，如 `http://10.0.0.1:2000` |
| `project_name` | string | 联网命令必填 | 空 | 服务端项目名，必须与发布时一致 |
| `ignore_folders` | string[] | 否 | `[]` | 发布扫描时忽略的文件夹 |
| `ignore_files` | string[] | 否 | `[]` | 发布扫描时忽略的文件 |
| `un_copy_folders` | string[] | 否 | `[]` | 客户端物化时“不迁移”的文件夹 |
| `un_copy_files` | string[] | 否 | `[]` | 客户端物化时“不迁移”的文件 |

### ignore_* 与 un_copy_* 的区别

这是两套作用点完全不同的规则：

| 规则 | 作用端 | 作用时机 | 影响服务端文件清单 | 影响客户端本地文件 |
|---|---|---|---|---|
| `ignore_folders` / `ignore_files` | `aly-publish` | 发布扫描 | 是，匹配的文件不会上传 | 间接：清单里没有，客户端就不会下载 |
| `un_copy_folders` / `un_copy_files` | `aly-client` | `download_update` 物化收尾 | 否 | 是，当前主目录中的匹配文件不会复制进待应用版本目录 |

`ignore_*` **不在客户端物化阶段生效**。如果某个文件已经被服务端收录，客户端仍会下载它，即使它同时出现在 `ignore_*` 里。反过来，`ignore_*` 也不是客户端的删除或保留规则：本地已存在的同名文件是否进入新版本目录，取决于物化时的 `un_copy_*` 以及它是否在新版本包中。

`un_copy_*` 只影响“把当前主目录中未变化文件复制进新版本目录”这一步；新版本包中确实存在的文件仍会正常下载。典型用途是把运行期数据、日志、缓存留在旧目录，不让它们进入新版本快照：

```json
{
  "un_copy_folders": ["logs", "cache"],
  "un_copy_files": ["*.log", "config/local.json"]
}
```

匹配语义：

- `ignore_folders` 是相对路径前缀匹配：`logs` 会跳过 `logs` 及其子目录；不按 `**` 展开。
- `ignore_files` 支持相对路径精确匹配、`filepath.Match` glob（`*` 不跨 `/`）、按文件名匹配，以及 `*` 开头的后缀匹配。
- `un_copy_folders` 支持相对路径精确匹配、glob 与 `**` 任意深度（例如 `**/node_modules`）。
- `un_copy_files` 支持按文件名或相对路径匹配，支持 `*`、`?` 等 glob。

::: warning
不要把 `.updator` 加进 `ignore_folders`：`shared.json` 需要随版本上传，否则新客户端拿不到服务器地址与项目名，配置更新也无法下发。`.updator/publish.json` 与 `.updator/staging/` 由发布工具自动排除，无需手工忽略。
:::

### 示例

```json
{
  "server_url": "http://10.0.0.1:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp"],
  "ignore_files": ["*.log", "*.tmp"],
  "un_copy_folders": ["logs", "cache"],
  "un_copy_files": ["*.log"]
}
```

### 边界与错误

- `server_url` 或 `project_name` 为空时，`check_update`、`check_diff`、`download_update` 分别返回 `no server url configured` / `no project name configured`。
- `shared.json` 缺失或损坏时，更新器在加载配置阶段失败；仅当 `MainFolder` 确实不存在（崩溃现场）时，才会回退到版本目录中查找 `shared.json`。
- `--url` / `--project-name` 只对 `check_update`、`check_diff`、`download_update` 生效，且只影响本次调用，不写回文件。
- `apply_update`、`rollback`、`list_rollback_versions`、`clean`、`check_self_update`、`self_update` 不需要服务端配置。

## version.json

### 位置与作用

位于 `UpdateFolder/version.json`，记录本机版本状态机。它不是发布包的一部分，不应随应用分发；由更新器写入，宿主只读。

不要手工编辑。状态与磁盘内容错配时，`apply_update` / `rollback` 会拒绝替换；只有 `download_update` 能把未知状态重写为 `downloaded` 完成修复。

### 字段

| 字段 | 类型 | 出现/默认 | 说明 |
|---|---|---|---|
| `version` | string | 通常存在 | `applied` 时为当前已应用版本；`downloaded` / `applying` 时为待应用或正在应用的版本 |
| `version_previous` | string | 可空 | 替换前 `MainFolder` 的真实内容版本，也是可回滚快照的目录后缀；首次更新为空 |
| `version_status` | string | 可空 | `applied` / `downloaded` / `applying`；空串或文件缺失等价于 `applied` |
| `apply_stage` | string | 可空 | 仅 apply 在途时写 `activating`，作为“激活改名已开始”的崩溃恢复证据；终态清空 |
| `downloaded_file_count` | int | `0` | 下载产物中的常规文件数（不含 `.part` / `.tmp`），用于 apply 识别残缺现场；旧文件为 0 表示未知，退化为“目录非空”弱校验 |
| `rollback_previous` | string | 可空 | 回滚开始时 `MainFolder` 的真实内容版本；仅回滚在途（`applying`）期间存在 |
| `rollback_target` | string | 可空 | 本次回滚的目标版本；仅回滚在途期间存在，供崩溃恢复续跑 |
| `apply_fail_count` | int | `0` | 同一待应用版本的连续 apply 失败次数；达到 3 次后 `check_update` 不再上报待应用更新 |
| `after_apply_update_script` | string | 可空 | 后置脚本相对 `MainFolder` 的路径；`download_update` 时从服务端变更日志同步，apply/rollback 成功后执行 |

### version_status 取值

| 值 | 含义 | 主目录内容 | 下一步 |
|---|---|---|---|
| `""`（缺失/空） | 没有版本记录，按 `applied` 处理 | 未知/首次部署 | `check_update` 重新检查；`apply_update` 返回无待应用更新 |
| `applied` | 没有待应用更新 | `version` 对应内容 | 正常检查新版本 |
| `downloaded` | 新版本已下载，等待应用 | `version_previous` 对应内容 | `apply_update`；产物丢失时 `download_update` 补齐 |
| `applying` | apply 或 rollback 在途 | 取决于崩溃点 | `check_update` / `apply_update` 走崩溃恢复 |

回滚在途时，`version` 仍是回滚前版本，目标版本记录在 `rollback_target`；回滚成功后才把 `version` 写为目标版本。

### 示例

下载完成、等待应用：

```json
{
  "version": "1.0.1",
  "version_previous": "1.0.0",
  "version_status": "downloaded",
  "downloaded_file_count": 128,
  "after_apply_update_script": "scripts/after_update.bat"
}
```

回滚在途：

```json
{
  "version": "1.0.1",
  "version_previous": "1.0.0",
  "version_status": "applying",
  "rollback_previous": "1.0.1",
  "rollback_target": "0.9.0"
}
```

apply 激活改名阶段（崩溃恢复证据）：

```json
{
  "version": "1.0.1",
  "version_previous": "1.0.0",
  "version_status": "applying",
  "apply_stage": "activating",
  "downloaded_file_count": 128
}
```

### 写入与恢复

- 写入是原子的：先写 `version.json.tmp`、`fsync`，再 rename 覆盖。
- 每次写入前，若当前文件可解析，会把它保存为 `version.json.bak`（last-known-good）。
- 读取时若 JSON 损坏，先尝试从 `.bak` 恢复；仍失败则把损坏文件改名为 `.corrupt` 并重置为空状态（等价首次部署），避免更新器永久瘫痪。
- 后置脚本调用结束后写 `MainFolder/.updator/after_apply_{version}.done` 标记（实现不校验脚本退出码），崩溃恢复据此跳过已调用过的脚本；脚本应按幂等编写，不能把“有标记”理解为“脚本成功”。

## .updator/publish.json

发布目录中的 `aly-publish` 专有配置，当前是空对象占位：

```json
{}
```

| 特性 | 说明 |
|---|---|
| 位置 | 发布目录 `.updator/publish.json` |
| 读写方 | `aly-publish`（含 publish-gui 通过子进程调用） |
| 是否上传 | 否，发布工具自动排除 |
| 字段 | 当前无正式字段，保留给 CLI 专有设置 |

不要把它放到 `UpdateFolder`，客户端不读取它。

## .updator/staging/staged-files.json

发布目录中的本地暂存区，由 `aly-publish add` 写入、`push` 成功后清空：

```json
[
  {
    "relativePath": "app.exe",
    "status": "modified",
    "localMd5": "abc123",
    "localSize": 5242880
  }
]
```

| 特性 | 说明 |
|---|---|
| 位置 | 发布目录 `.updator/staging/staged-files.json` |
| 读写方 | `aly-publish` 专有 |
| 是否上传 | 否，扫描时整个 `staging/` 目录被跳过 |
| 用途 | 记录待发布文件及其状态，供 `push` 使用 |

客户端不读取该文件。不要手工编辑；需要调整暂存内容时用 `aly-publish add` / `reset`。

## 配置优先级

客户端没有“多个配置文件互相覆盖同一字段”的复杂合并，而是按固定顺序加载并按需覆盖：

| 配置项 | 来源顺序（后者覆盖前者） |
|---|---|
| 主程序路径 | `client.json` 的 `main_exe_relative_path` → `--main-exe-path` |
| 需关闭进程 | `client.json` 的 `must_close_process_name` → `--must-close-process-name`（**追加**） |
| 服务端地址 | `shared.json` 的 `server_url` → `--url`（仅 check/diff/download） |
| 项目名 | `shared.json` 的 `project_name` → `--project-name`（仅 check/diff/download） |
| 忽略/不复制规则 | 仅 `shared.json`，无命令行覆盖 |
| 版本状态 | 仅 `version.json` |

加载顺序：

1. 从 `UpdateFolder` 读取 `client.json`；
2. 合并 `--main-exe-path`，推导 `MainFolder`；
3. 读取 `MainFolder/.updator/shared.json`；
4. 用 `--url` / `--project-name` 覆盖本次调用的服务端配置；
5. 追加 `--must-close-process-name`。

发布侧的配置优先级（CLI 参数优先于 `.updator/shared.json` / `publish.json`）由 `aly-publish` 决定，见[发布工具](/docs/publish)。