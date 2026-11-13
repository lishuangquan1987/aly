---
title: 发布流程命令
description: status、diff、add、reset、staged、push、log、server info 的行为、JSON 契约、push 进度流与失败语义。
---

# 发布流程命令

这组命令覆盖一次发布的完整链路：比对差异 → 暂存变更 → 上传并登记版本。除特别说明外，示例省略全局参数 `--server`、`--project`、`--json`、`--quiet`。

| 命令 | 作用 | 需要服务端 | 需要 `project.name` | 需要暂存区 |
|---|---|---|---|---|
| `status` | 本地与服务端差异总览，并同步暂存区 | 是 | 是 | 否 |
| `diff` | 逐文件输出本地/服务端哈希与大小 | 是 | 是 | 否 |
| `add` | 暂存变更 | `--all` 需要；指定文件不需要 | 是 | 否 |
| `reset` | 取消暂存 | 否 | 是 | 是 |
| `staged` | 查看暂存区 | 否 | 是 | 否 |
| `push` | 上传暂存文件并登记版本 | 是 | 是 | 是（非空） |
| `log` | 查看版本日志 | 是 | 是（服务端拒绝空名称） | 否 |
| `server info` | 查看服务端系统信息 | 是 | 否 | 否 |

## 暂存区模型

暂存区是本地文件 `.updator/staging/staged-files.json`，不会上传。每条记录包含相对路径、加入时的状态、本地 MD5 与大小：

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

`add` 写入，`reset` 删除，`push` 读取并在成功后清空。`status` 会把暂存区合并进差异结果，并自动清理“已经上传成功”的条目。

## status

```bash
aly-publish status
```

流程：

1. 递归扫描当前目录，跳过 `.updator/staging/`、`.updator/publish.json` 及忽略规则命中的路径；`.updator/shared.json` 会被扫描（它需要随版本上传）。
2. 为每个文件计算 MD5 + SHA256。
3. 调用 `GET /api/file/get_all_files/{projectName}` 拉取服务端清单。
4. 按相对路径与 MD5 分类：`new`、`modified`、`deleted`、`unchanged`。
5. **自动同步暂存区**：若某个已暂存文件在服务端存在且 MD5 相同，视为此前已上传成功，从暂存区移除。human 模式下会打印：

   ```text
   已同步暂存区：移除了 2 个已上传成功的文件
           DemoApp.dll
           assets/theme.pak
   ```

   这一步用于处理“上传完成但版本未登记 / 进程中断”的现场，重跑 `status` 即可恢复一致。
6. 把剩余暂存条目放入 `staged`，并从 `unstaged` 中移除。

human 输出：

```text
Changes staged for commit:
  (use "aly-publish reset <file>..." to unstage)

        modified:    DemoApp.dll
        new:         .updator/shared.json

Changes not staged for commit:
  (use "aly-publish add <file>..." to stage)

        deleted:     old.txt
        modified:    sub/b.bin

Unchanged files:
        a.txt
```

`deleted` 出现在 `unstaged` 中；`add --all` **会**暂存删除项，`push` 时调用 `file/delete_file` 同步删除。

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "staged": [
      {
        "relativePath": "DemoApp.dll",
        "status": "modified",
        "localMd5": "96c15c2bb2921193bf290df8cd85e2ba",
        "localSize": 90052
      }
    ],
    "unstaged": [
      {
        "relativePath": ".updator/shared.json",
        "status": "new",
        "localMd5": "97a209b3de6950d03a6469dea098fd7d",
        "localSize": 234
      },
      {
        "relativePath": "old.txt",
        "status": "deleted",
        "localMd5": "",
        "localSize": 0,
        "serverMd5": "cccccccccccccccccccccccccccccccc",
        "serverSize": 3
      }
    ],
    "unchanged": [
      {
        "relativePath": "a.txt",
        "status": "unchanged",
        "localMd5": "5d41402abc4b2a76b9719d911017c592",
        "localSize": 5,
        "serverMd5": "5d41402abc4b2a76b9719d911017c592",
        "serverSize": 5
      }
    ]
  }
}
```

注意：

- 空分类输出为 `null` 而不是 `[]`（Go 的 nil slice）。
- 暂存条目没有 `serverMd5` / `serverSize` 字段；它们来自 `add` 时的本地快照。
- `status` 失败（服务端不可达等）返回 `isSuccess: false`，退出码仍为 0。
- `--quiet` 可抑制 human 文本；`--json` 仍完整输出。

## diff

```bash
aly-publish diff [--file <相对路径>]
```

| 参数 | 说明 |
|---|---|
| `--file` | 只输出指定相对路径的文件；按完整相对路径精确匹配 |

输出示例：

```text
--- sub/b.bin ---
- local:  sub/b.bin  modified  11 bytes  md5:96c15c2bb2921193bf290df8cd85e2ba
+ server: sub/b.bin  modified  11 bytes  md5:00000000000000000000000000000000

--- a.txt ---
- local:  a.txt  unchanged  5 bytes  md5:5d41402abc4b2a76b9719d911017c592
+ server: a.txt  unchanged  5 bytes  md5:5d41402abc4b2a76b9719d911017c592
```

行为与限制：

- 输出暂存、未暂存、未变化的**全部**文件，不做过滤；`--file` 是唯一筛选条件。
- **没有 `--json` 分支**。即使加 `--json`，仍然输出上面的文本，不会产生 JSON；需要结构化数据请用 `status --json`。
- 与 `status` 不同，`diff` **不会**同步暂存区。
- 新增文件的 `+ server` 行没有真实数据，显示 `0 bytes` 与空 MD5；这是预期输出。

## add

```bash
aly-publish add --all
aly-publish add <file1> <file2> ...
```

| 模式 | 需要服务端 | 行为 |
|---|---|---|
| `--all` | 是 | 先执行一次差异扫描，只把 `new` 与 `modified` 加入暂存区 |
| 指定文件 | 否 | 直接计算本地文件的 MD5 与大小后加入暂存区 |

示例：

```bash
aly-publish add --all
aly-publish add DemoApp.dll config/settings.json
```

JSON：

```json
{"isSuccess":true,"errorMsg":"","data":{"added":2}}
```

行为与限制：

- `--all` 会暂存 `new` / `modified` / `deleted`（已暂存的不重复暂存）；`deleted` 由 `push` 调用 `file/delete_file` 同步删除。
- 指定文件模式**不查询服务端**，因此不知道文件是新增还是修改；暂存条目的状态一律记为 `modified`。真正的 `new`/`modified` 判定仍以 `status`/`push` 时的服务端清单为准。
- 指定文件必须存在于项目目录中；路径不存在会返回 `hash <path>: ...` 形式的业务失败。
- 重复 `add` 同一路径是幂等的（已存在的条目会被跳过），但 JSON 中的 `added` 统计的是**本次请求的文件数**，不是实际新增的条目数。
- 不带参数且不带 `--all` 时只打印用法，属于业务失败（退出码 0）：

  ```json
  {"isSuccess":false,"errorMsg":"Usage: aly-publish add [--all | <file>...]","data":null}
  ```

## reset

```bash
aly-publish reset --all
aly-publish reset <file1> <file2> ...
```

- `--all` 清空整个暂存区，JSON 的 `data` 为 `null`。
- 指定文件按相对路径移除；不存在的条目会被忽略。

```json
{"isSuccess":true,"errorMsg":"","data":{"removed":2}}
```

human 输出：

```text
已从暂存区移除2 个文件
```

`reset` 只改本地暂存区，不需要服务端。不带参数且不带 `--all` 时打印用法并返回业务失败。

## staged

```bash
aly-publish staged
```

human 输出：

```text
Staged files:
  [new] .updator/shared.json  234 bytes
  [modified] sub/b.bin  11 bytes
```

暂存区为空时输出 `暂存区为空`。

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "relativePath": ".updator/shared.json",
      "status": "new",
      "localMd5": "97a209b3de6950d03a6469dea098fd7d",
      "localSize": 234
    },
    {
      "relativePath": "sub/b.bin",
      "status": "modified",
      "localMd5": "96c15c2bb2921193bf290df8cd85e2ba",
      "localSize": 11
    }
  ]
}
```

条目按 `relativePath` 排序。暂存区为空时 `data` 为 `null`。

## push

```bash
aly-publish push --version <版本号> --message <说明> [--message <说明> ...] \
  [--dry-run] [--force] [--after-apply-update-script <脚本>] [--set-force-update]
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--version` | string | **是** | 新版本号，须严格大于服务端最新版本 |
| `--message` | string[] | **是**（至少一条） | 变更说明，可重复指定 |
| `--dry-run` | bool | 否 | 只列出将上传的文件与版本，不实际推送 |
| `--force` | bool | 否 | 跳过暂存文件的 MD5 复核 |
| `--after-apply-update-script` | string | 否 | 更新后执行的脚本，路径相对于应用目录 |
| `--set-force-update` | bool | 否 | 推送成功后把项目标记为强制更新 |

### 前置条件与流程

1. 必须能解析出服务端地址与 `project.name`。
2. 暂存区必须非空；为空时返回“暂存区为空，请先 add 文件”。
3. 至少一条 `--message`；缺失时返回“至少需要一条 `--message`”。
4. 默认重新计算暂存文件的 MD5，与 `add` 时记录的值比较。不一致说明文件在暂存后被改动：

   ```json
   {"isSuccess":false,"errorMsg":"以下文件在 add 后被修改，请重新 add: [DemoApp.dll]","data":null}
   ```

   `--force` 跳过这一步，直接上传当前文件。
5. 并发上传。并发数为 CPU 数，限制在 3–16 之间。单文件不超过 1 MiB 时直传；大于 1 MiB 时按 1 MiB 分片上传，分片完成后通知服务端合并。单文件/单分片最多尝试 5 次（首次 + 4 次重试，退避 400/800/1600/3200 ms）。
6. 上传全部成功后调用 `POST /api/project/publish_version` 登记版本，时间为本地 `yyyy-MM-dd HH:mm:ss`。
7. 登记成功后清空暂存区，并按 `--set-force-update` 的值更新项目强制更新标记。

### JSON 进度流

`push --json` 不是单行输出，而是逐行 JSON：

1. 所有文件的 `START` 行（按暂存顺序，先全部输出）；
2. 每个文件的 `DONE` 或 `FAIL` 行（按原顺序）；
3. 一行最终结果（成功时 `data` 含 `version` 与 `files`；失败时 `isSuccess: false`）；
4. 一行结束标记：`{"isSuccess":true,"errorMsg":"","data":null}`。

成功示例（2 个文件）：

```json
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"START","file_size":234}}
{"isSuccess":true,"errorMsg":"","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"START","file_size":90052}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"DONE","file_size":234}}
{"isSuccess":true,"errorMsg":"","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"DONE","file_size":90052}}
{"isSuccess":true,"errorMsg":"","data":{"files":"2","version":"V1.0.2"}}
{"isSuccess":true,"errorMsg":"","data":null}
```

进度数据的字段是 `index`、`total`、`file`、`status`、`file_size`、`error`（失败时才有）。`data.files` 在最终结果中是**字符串**。

上传失败示例：

```json
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"START","file_size":234}}
{"isSuccess":true,"errorMsg":"","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"START","file_size":90052}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"DONE","file_size":234}}
{"isSuccess":false,"errorMsg":"DemoApp.dll: 磁盘已满","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"FAIL","file_size":90052,"error":"磁盘已满"}}
{"isSuccess":false,"errorMsg":"上传失败 [DemoApp.dll]: 磁盘已满","data":null}
{"isSuccess":true,"errorMsg":"","data":null}
```

消费方的终止条件应是“`data` 为 `null` 的 `isSuccess: true` 行”，而不是简单读到 EOF。

前置校验失败（缺少配置、暂存区为空、缺少 `--message`、MD5 冲突）不会进入进度流：只输出**一行**失败 JSON，既没有 START/DONE/FAIL，也没有结束标记。只有开始上传后，才会出现完整进度流并以结束标记收尾。

### 失败语义

- 上传阶段有任一文件失败：其余文件仍会完成上传，随后输出失败结果与结束标记；暂存区**不清空**，修复后可直接重试。
- 文件全部上传成功但版本登记失败（例如版本不单调）：
  `errorMsg` 形如“创建版本失败: POST /api/project/publish_version: 版本 ...（文件已上传，可重试 push）”。服务端文件已就位，但版本未登记；可用更高版本号重试。
- 所有业务失败退出码都是 0，必须检查 `isSuccess`。

### dry-run

`--dry-run` 在 MD5 复核之后、上传之前返回，输出人类可读的计划：

```text
[DRY RUN] 将上传以下文件：
  .updator/shared.json
  DemoApp.dll
[DRY RUN] 将创建版本 V1.0.3 (1 条日志)
[DRY RUN] 未实际推送任何内容。
```

限制：

- dry-run **不产生** JSON 进度流。即使加 `--json`，输出仍是上面的文本；加 `--quiet` 则完全没有输出。
- 不校验版本是否大于服务端最新版本（这一步在服务端登记时才做）。
- 不清空暂存区。

### 强制更新参数

`--set-force-update` 表示“推送成功后把项目标记为强制更新”，客户端检查更新时不再询问直接更新。

::: warning 当前实现的行为
成功推送后，CLI 总会调用 `set_force_update`，传入的值就是 `--set-force-update` 的当前值。也就是说，不传该参数的成功推送会把项目的强制更新标记置为 `false`。如果项目原本处于强制更新状态且需要保持，请显式传入 `--set-force-update`。
:::

### 示例

```bash
# 普通发布
aly-publish push --version V1.0.2 --message "修复登录超时" --message "优化大文件下载速度"

# 先演练
aly-publish push --version V1.0.3 --message "重构上传" --dry-run

# 脚本 + 强制更新
aly-publish push --version V1.0.4 --message "升级数据库" \
  --after-apply-update-script update.bat --set-force-update
```

## log

```bash
aly-publish log [--limit <条数>]
```

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `--limit` | int | 20 | 只显示最近 N 条 |

human 输出：

```text
V1.0.1 (2026-11-10 10:45:42)
  • 修复登录超时
  • 优化大文件下载速度
V1.0.0 (2026-11-03 10:12:33)
  • 第一次创建
```

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 7,
      "project_id": 3,
      "version": "V1.0.1",
      "logs": ["修复登录超时", "优化大文件下载速度"],
      "time": "2026-11-10 10:45:42",
      "created_at": "2026-11-10T10:45:42+08:00",
      "is_deleted": false,
      "after_apply_update_script": ""
    }
  ]
}
```

说明：

- 服务端按日志 ID 倒序返回（最新在前），CLI 截取前 `--limit` 条。
- 项目名来自 `project.name` 或 `--project`。CLI 只强校验服务端地址，项目名为空时服务端会拒绝并返回业务失败。
- `--limit 0` 返回空数组；不要传负数。

## server info

```bash
aly-publish server info
```

human 输出：

```text
OS:              windows
Platform:        Microsoft Windows 11 Pro
Architecture:    amd64
Go Version:      go1.25.0
CPU:             AMD Ryzen 7 5800X
Cores:           16 @ 3800 MHz
Disk:            120.5 GiB used / 460.7 GiB total (26.2%)
```

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "os": "windows",
      "platform": "Microsoft Windows 11 Pro",
      "goARCH": "amd64",
      "version": "go1.25.0",
      "numCPU": 16,
      "cpuName": "AMD Ryzen 7 5800X",
      "cpuMhz": 3800,
      "diskUsed": 120.5,
      "diskFree": 340.2,
      "diskTotal": 460.7,
      "diskUsedPercent": 26.2
    }
  ]
}
```

- 该命令只需要服务端地址，不需要项目配置。
- 服务端返回空数据时，human 输出 `无服务器信息`，JSON 的 `data` 为 `null` 或空数组（取决于服务端响应）。
- 磁盘容量的单位是 GiB，保留一位小数。

## 下一步

- 配置项含义：[配置命令](/docs/publish/commands-config)
- 服务端项目准备：[项目命令](/docs/publish/commands-project)
- 图形化操作：[发布 GUI](/docs/publish/gui)
- 回到工具总览：[aly-publish](/docs/publish)