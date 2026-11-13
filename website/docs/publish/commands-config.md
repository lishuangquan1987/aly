---
title: 配置命令 config
description: aly-publish config init/set/set-array/get/list/path 的用法、参数、JSON 输出、行为边界与限制。
---

# 配置命令 config

`config` 管理当前工作目录下的 `.updator/`：

| 文件 | 内容 | 是否上传 |
|---|---|---|
| `.updator/shared.json` | `server_url`、`project_name`、`ignore_folders/files`、`un_copy_folders/files` | 是（随版本分发） |
| `.updator/publish.json` | CLI 专有配置，当前为空对象 `{}` | 否 |

所有命令都从**当前工作目录**读取配置，没有指定目录的参数；换目录请先 `cd`。`--server`、`--project` 全局参数会覆盖本次调用读到的值，但不会写回文件。

::: tip JSON 示例格式
CLI 实际输出为单行 JSON。下文较长的示例为便于阅读做了缩进，字段与取值不变。
:::

## 可读写的键

| 键 | 类型 | 可用命令 | 说明 |
|---|---|---|---|
| `server.url` | string | `set`、`get`、`list` | 服务端地址 |
| `project.name` | string | `set`、`get`、`list` | 服务端项目名 |
| `ignore.folders` | string[] | `set-array`、`list` | 扫描时跳过的目录（相对项目根） |
| `ignore.files` | string[] | `set-array`、`list` | 扫描时跳过的文件（支持 glob） |
| `un_copy.folders` | string[] | `set-array`、`list` | 客户端应用更新时不复制的目录 |
| `un_copy.files` | string[] | `set-array`、`list` | 客户端应用更新时不复制的文件 |

`set` / `get` 只认两个标量键。数组必须走 `set-array`；`get` 无法读取数组，`set` 也无法整体替换数组。

## config init

创建 `.updator/` 并写入 `shared.json` 与 `publish.json`。

```bash
aly-publish config init --project <项目名> [--server <地址>] \
  [--ignore-folders <逗号分隔>] [--ignore-files <逗号分隔>] \
  [--un-copy-folders <逗号分隔>] [--un-copy-files <逗号分隔>]
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--project` | string | **是** | 写入 `project_name` |
| `--server` | string | 否 | 写入 `server_url`；也可之后用 `config set server.url` 设置 |
| `--ignore-folders` | string | 否 | 忽略目录，逗号分隔 |
| `--ignore-files` | string | 否 | 忽略文件，逗号分隔 |
| `--un-copy-folders` | string | 否 | 不复制目录，逗号分隔 |
| `--un-copy-files` | string | 否 | 不复制文件，逗号分隔 |

行为：

1. 先加载当前目录已有的 `shared.json`（若存在），再用命令行参数覆盖对应字段。
2. 写入 `shared.json`；未提供的字段保留原值，首次初始化则为空字符串/空数组。
3. 写入 `publish.json`，内容固定为 `{}`。
4. 两个文件都采用“临时文件 + rename”的原子写，崩溃不会留下半截 JSON。

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "publish_path": "C:\\build\\dist\\.updator\\publish.json",
    "shared_path": "C:\\build\\dist\\.updator\\shared.json"
  }
}
```

human 输出：

```text
配置已保存到: C:\build\dist\.updator
```

限制与注意：

- `--project` 是必需项（或工作区既有的 `shared.json` 里已有 `project_name`）：两者都没有时命令**直接失败**并返回 `config init 必须通过 --project 指定项目名称`，不会写出空配置（2026-11-12 修复 M14）。
- `config init` 只写本地文件，不校验项目名是否合法、也不创建服务端项目。名称规则由服务端在 `project create` / `push` 时检查。
- 在已初始化的目录重复执行不会清空未指定的字段；要清空数组请用 `config set-array <key> --clear`，要清空标量可用 `config set <key> ""`。CLI 没有 `unset` 命令。
- `publish.json` 当前没有可写字段，写入 `{}` 属正常现象。

## config set

设置单个标量配置项。

```bash
aly-publish config set <key> <value>
```

| 参数 | 说明 |
|---|---|
| `<key>` | 仅支持 `server.url`、`project.name` |
| `<value>` | 原样写入的字符串；含空格时用引号包住 |

```bash
aly-publish config set server.url http://192.168.1.100:2000
aly-publish config set project.name myapp
```

成功输出（human）：

```text
配置已更新 server.url = http://192.168.1.100:2000
```

JSON：

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

行为与限制：

- 读取现有配置后写回 `shared.json` 与 `publish.json`；目录不存在时会自动创建 `.updator/`，但更推荐先 `config init`。
- 未知键返回业务失败（退出码仍为 0）：

  ```json
  {"isSuccess":false,"errorMsg":"unknown config key 'output.format'","data":null}
  ```

- 没有 `output.format` 配置项；输出格式由每次调用的 `--json` 决定。
- `server.url` 只写 `scheme://host:port`，不要带路径前缀。API 客户端会拼接 `/api/...`，带路径会导致请求落到错误地址；末尾的 `/` 会被自动去掉。
- `set` 不校验服务端是否可达，也不校验项目是否存在。

## config set-array

对数组配置执行追加、移除或清空。

```bash
aly-publish config set-array <key> --add <item> | --remove <item> | --clear
```

| 参数 | 类型 | 说明 |
|---|---|---|
| `<key>` | string | `ignore.folders`、`ignore.files`、`un_copy.folders`、`un_copy.files` |
| `--add` | string | 追加一个条目 |
| `--remove` | string | 移除所有完全相同的条目 |
| `--clear` | bool | 清空整个数组 |

```bash
aly-publish config set-array ignore.folders --add logs
aly-publish config set-array ignore.folders --add temp
aly-publish config set-array ignore.folders --remove temp
aly-publish config set-array ignore.files --clear
```

执行顺序固定为 `--clear` → `--add` → `--remove`。三个参数可以同时出现，但通常只用一个。

成功输出（human）：

```text
配置已更新 ignore.folders
```

JSON：

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

行为与限制：

- **一次只能加/删一个条目**。`--add` 是普通 string 参数，不是可重复参数；重复写 `--add a --add b` 只有最后一个 `b` 生效。批量添加请分多次调用。
- 追加**不去重**；同一项执行两次 `--add` 会出现两个相同条目。`--remove` 会移除所有完全匹配的条目。
- 匹配是字符串比较，不支持 glob；`*.log` 只是作为模式字符串存入，真正的通配匹配发生在扫描/客户端使用时。
- 数组的当前值请用 `config list` 查看；`config get ignore.folders` 返回空字符串，不能用来读数组。
- 未知键返回 `unknown config key '...'` 的业务失败。

## config get

读取一个标量配置项。

```bash
aly-publish config get <key>
```

```bash
aly-publish config get server.url
# http://192.168.1.100:2000
```

JSON：

```json
{"isSuccess":true,"errorMsg":"","data":{"server.url":"http://192.168.1.100:2000"}}
```

行为与限制：

- 只支持 `server.url` 与 `project.name`。
- 未知键或数组键**不会报错**：返回空字符串，且 `isSuccess` 为 `true`。这是最容易误判的地方——空值可能表示“没配置”，也可能表示“键不存在”。

  ```json
  {"isSuccess":true,"errorMsg":"","data":{"ignore.folders":""}}
  ```

- 命令输出会应用本次的 `--server` / `--project` 覆盖值；例如 `config get server.url --server http://10.0.0.1:2000` 返回临时值，不写回文件。

## config list

列出全部配置。

```bash
aly-publish config list
```

human 输出（注意各行的空格数并不完全对齐）：

```text
server.url      = http://192.168.1.100:2000
project.name    = myapp
ignore.folders   = [logs temp]
ignore.files     = [*.log]
un_copy.folders  = [docs]
un_copy.files    = [*.md]
```

JSON：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "publish": {},
    "shared": {
      "server_url": "http://192.168.1.100:2000",
      "project_name": "myapp",
      "ignore_folders": ["logs", "temp"],
      "ignore_files": ["*.log"],
      "un_copy_folders": ["docs"],
      "un_copy_files": ["*.md"]
    }
  }
}
```

注意 JSON 中是 `shared` 的原始字段名（`server_url`、`project_name` 等），与 human 输出的点号键名不同。

限制：

- 一次输出全部字段，没有过滤参数。
- `publish` 当前恒为 `{}`，因为 `publish.json` 还没有字段。
- 不包含暂存区内容；暂存区用 `aly-publish staged` 查看。
- 与 `config get` 一样，输出反映本次 `--server` / `--project` 的覆盖值。

## config path

显示 `.updator/` 目录路径（不是某个文件）。

```bash
aly-publish config path
# C:\build\dist\.updator
```

JSON：

```json
{"isSuccess":true,"errorMsg":"","data":{"path":"C:\\build\\dist\\.updator"}}
```

路径由当前工作目录拼出，因此结果取决于执行命令时所在的目录。

## 限制速查

| 限制 | 说明 |
|---|---|
| `set` / `get` 只支持两个标量键 | `server.url`、`project.name` |
| 数组只能通过 `set-array` 修改 | `get` 读数组返回空字符串 |
| `set-array` 每次只处理一个条目 | 重复 `--add` 只保留最后一个 |
| `init` 必须提供项目名 | 缺 `--project` 且既有配置也没有 `project_name` 时直接报错（不再静默写空配置） |
| 配置始终属于当前目录 | 没有 `--path` / `--cwd` |
| 没有 `output.format` | 输出格式只由 `--json` 控制 |
| 没有 `unset` / 删除键命令 | 需要清空时用 `set-array --clear` 或手动编辑 JSON |

## 下一步

- 创建与管理服务端项目：[项目命令](/docs/publish/commands-project)
- 配置好之后开始比对与推送：[发布流程命令](/docs/publish/commands-workflow)
- 回到工具总览：[aly-publish](/docs/publish)