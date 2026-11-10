---
title: 项目命令 project
description: aly-publish project list/create/update/delete/info 的参数、JSON 输出、名称与标题规则、软删除行为。
---

# 项目命令 project

`project` 管理**服务端**的项目记录：项目是文件与版本日志的容器，终端客户端按项目名拉取更新。它与本地 `.updator/` 配置是两套独立的数据。

| 命令 | 作用 | 是否需要服务端 | 是否需要本地 `.updator/` |
|---|---|---|---|
| `project list` | 列出未删除的项目 | 是 | 否 |
| `project create` | 创建项目，初始版本 V1.0.0 | 是 | 否 |
| `project update` | 更新标题、强制更新与忽略规则 | 是 | 否 |
| `project delete` | 软删除项目 | 是 | 否 |
| `project info` | 查看单个项目详情 | 是 | 否 |

服务端地址来自 `--server` 或当前目录 `shared.json` 的 `server_url`。这些命令不使用全局 `--project`（它们用各自的 `--name`），也不要求 `project.name` 已配置。

## --name、--title 与 --project 的区别

| 参数 | 归属 | 作用 | 是否写本地配置 |
|---|---|---|---|
| `--name` | `project` 子命令自己的参数 | 服务端项目的唯一标识，决定数据目录名 | 否 |
| `--title` | `project create/update` 的参数 | 展示给最终用户的标题 | 否 |
| `--project` | 全局参数 | 覆盖 `shared.json` 的 `project_name`，供 `status`/`push` 等命令使用 | 否 |
| `config init --project` | `config init` 使用全局参数 | 把项目名写入 `shared.json` | **是** |

三者不会自动同步。常见做法是保持 `config init --project` 与 `project create --name` 相同，但 CLI 不会替你校验；不一致时 `status` 会去查一个不存在的项目。

## 名称与标题规则

`--name` 由服务端校验：

- 长度 1–64；
- 不能包含 `< > : " / \ | ? *`；
- 不能是 Windows 保留名（`CON`、`PRN`、`AUX`、`NUL`、`COM1`–`COM9`、`LPT1`–`LPT9`，大小写不敏感）；
- 全局唯一。软删除的项目仍占用名称，重建同名项目会返回“项目名称:xxx已存在”。

`--title` 由服务端校验非空。`project create` 与 `project update` 都把 `--title` 标为必填参数，因此即使只想改强制更新开关，也必须传标题。

## project list

```bash
aly-publish project list [--server <地址>]
```

human 输出：

```text
  ID   NAME     TITLE      VERSION  FORCE UPDATE CREATED
  3    myapp    我的应用   V1.0.1   no           2026-11-03
```

- `CREATED` 只显示日期的前 10 个字符（`yyyy-MM-dd`）。
- 软删除的项目不会出现。

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 3,
      "name": "myapp",
      "title": "我的应用",
      "version": "V1.0.1",
      "force_update": false,
      "ignore_folders": ["logs", "temp"],
      "ignore_files": ["*.log"],
      "created_at": "2026-11-03T10:12:33+08:00",
      "is_deleted": false
    }
  ]
}
```

字段来自服务端 ent 模型，使用 snake_case；`created_at` 是带时区的 RFC3339 时间。

## project create

```bash
aly-publish project create --name <名称> --title <标题> \
  [--force-update] [--ignore-folders <逗号分隔>] [--ignore-files <逗号分隔>]
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--name` | string | **是** | 项目唯一标识 |
| `--title` | string | **是** | 项目标题 |
| `--force-update` | bool | 否 | 项目级强制更新，默认 false |
| `--ignore-folders` | string | 否 | 忽略目录，逗号分隔 |
| `--ignore-files` | string | 否 | 忽略文件，逗号分隔 |

行为：

1. 服务端校验名称与标题。
2. 检查同名项目（包括已软删除的），存在则返回业务失败。
3. 创建 `data/<name>/` 目录。
4. 写入项目记录，版本为 **V1.0.0**，并插入一条内容为“第一次创建”的变更日志。
5. 返回创建后的项目对象。

```bash
aly-publish project create --name myapp --title "我的应用" --ignore-folders "logs,temp" --ignore-files "*.log"
```

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 3,
    "name": "myapp",
    "title": "我的应用",
    "version": "V1.0.0",
    "force_update": false,
    "ignore_folders": ["logs", "temp"],
    "ignore_files": ["*.log"],
    "created_at": "2026-11-03T10:12:33+08:00",
    "is_deleted": false
  }
}
```

human 输出：

```text
项目创建成功: [3] myapp (V1.0.0)
```

注意：

- `project create` 不写 `.updator/shared.json`，也不会复制任何本地文件。
- 初始版本 V1.0.0 占用了版本号。第一次 `push` 必须使用更高版本（例如 V1.0.1），否则会被防倒退校验拒绝。
- `--force-update` 只影响客户端更新时的确认行为，与版本号无关。

## project update

```bash
aly-publish project update --name <名称> --title <标题> \
  [--force-update] [--ignore-folders <逗号分隔>] [--ignore-files <逗号分隔>]
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--name` | string | **是** | 定位项目，不能修改名称本身 |
| `--title` | string | **是** | 新标题 |
| `--force-update` | bool | 否 | 默认 false |
| `--ignore-folders` | string | 否 | 默认空（清空） |
| `--ignore-files` | string | 否 | 默认空（清空） |

这是**整字段覆盖**，不是局部补丁：标题、强制更新、两个忽略数组都会按本次参数写入。未传的数组会被清空，未传的 `--force-update` 会写成 `false`。因此只想改标题时，也应把当前的忽略规则和强制更新状态一并传入，否则会被清空。

```bash
# 同时保留原来的忽略规则与强制更新状态
aly-publish project update --name myapp --title "我的应用 V2" \
  --force-update --ignore-folders "logs,temp" --ignore-files "*.log"
```

成功 JSON：

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

human 输出：

```text
项目更新成功: myapp
```

限制：

- 不能重命名项目。改名需要新建项目，再让本地 `config set project.name` 指向新名称。
- 只能更新未软删除的项目；项目不存在时返回业务失败。
- 不修改版本号，也不影响已有版本日志。
- 修改忽略规则后，服务端会清除该项目的文件列表缓存，下一次 `get_all_files` 按新规则重新收集。

## project delete

```bash
aly-publish project delete --name <名称>
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--name` | string | **是** | 要删除的项目名 |

行为：软删除，仅把 `is_deleted` 置为 `true`。

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

human 输出：

```text
项目已删除: old_project
```

软删除的后果：

- `project list`、`project info` 不再返回该项目；
- 服务端文件与版本日志**不会**被物理删除，也不会级联修改终端上的已有配置；
- 项目名仍被占用。用同名再次 `project create` 会得到“项目名称:old_project已存在”；
- 因为数据仍在，软删除不等于“让终端立刻停止更新”；需要停止分发时，应另行处理服务端数据与终端配置。

## project info

```bash
aly-publish project info --name <名称>
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--name` | string | **是** | 项目名 |

`--id` 参数虽然仍在代码中注册，但被标记为隐藏且不参与查询；`--name` 是必填参数，只有 `--id` 无法通过参数校验。按名称查询即可。

human 输出：

```text
Project: myapp
Title:   我的应用
Version: V1.0.1
Force Update: no
Created: 2026-11-03

Ignore Folders:
  logs
  temp
Ignore Files:
  *.log
```

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 3,
    "name": "myapp",
    "title": "我的应用",
    "version": "V1.0.1",
    "force_update": false,
    "ignore_folders": ["logs", "temp"],
    "ignore_files": ["*.log"],
    "created_at": "2026-11-03T10:12:33+08:00",
    "is_deleted": false
  }
}
```

实现上，`project info` 先拉取全部未删除项目，再在本地按名称精确匹配；项目不存在时返回业务失败（退出码 0）：

```json
{"isSuccess":false,"errorMsg":"项目不存在","data":null}
```

## 下一步

- 本地配置怎么写：[配置命令](/docs/publish/commands-config)
- 开始比对与推送：[发布流程命令](/docs/publish/commands-workflow)
- 回到工具总览：[aly-publish](/docs/publish)