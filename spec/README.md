# spec/ — aly 客户端协议单一真源

本目录定义 `aly-client` 与宿主之间的 JSON 协议。任何语言的 SDK 都必须以这里为准，
**不要**从 C# / Go 代码里手抄字段。

| 文件 | 作用 | 规范性 |
|---|---|---|
| `aly-protocol.schema.json` | 字段真源：信封、8 条命令的载荷、下载进度行与嵌套文件项 | 是（机器可校验） |
| `host-contract.md` | schema 表达不了的行为：调用约定、超时、状态机、确认语义、错误分类 | 是（人读） |
| `README.md` | 修改流程与兼容策略 | 本文件 |

## 修改协议的正确流程

1. 改 `aly-protocol.schema.json`（新增/删除/改名字段），必要时同步 `host-contract.md`。
2. 改 `client/aly-client/model/dto.go`，让客户端真的输出/不再输出该字段。
3. 重新生成各语言模型（当前只有 C#）：

   ```bash
   cd tools/alyprotocol
   go run . -write
   go run . -check
   ```

4. 编译对应 SDK（C#：`dotnet build` 双 TFM）。
5. 提交：schema、dto.go、生成物、受影响文档必须在同一个 commit 里。

`go run . -check` 会做三件事，任何一件失败都退出码 1：

- 校验 schema 自身（`required` 必须存在、`$ref` 必须可解析、`additionalProperties:false`、命名规范）；
- 校验 schema ↔ `dto.go` 的字段名、类型、必填/可选（双向）；
- 比对生成物与磁盘（CRLF/BOM 归一化后逐字节）。

CI 在打包 C# SDK 之前执行同一个 `-check`，漂移会直接红灯。

## 兼容策略

`x-protocol-version` 采用 `MAJOR.MINOR.PATCH`：

| 变更 | 版本位 | 例子 |
|---|---|---|
| 新增可选字段、新增命令 | MINOR | 新增 `release_notes` |
| 删除字段、改名、改类型、改语义 | MAJOR | `data` 从对象改成数组 |
| 只改描述/文档 | PATCH | 修错别字 |

方向性约定：

- **客户端严格**：schema 对客户端输出做 `additionalProperties:false`，加字段必须同步 schema。
- **宿主宽松**：SDK 反序列化必须忽略未知字段；未知 `status`、未知 `data` 形状都不得崩溃。
- 目前没有运行时版本握手；SDK 靠 `self_update` 保证更新器副本兼容。握手命令列入后续工作。

## 给新语言 SDK 的接入顺序

1. 读 `aly-protocol.schema.json`，用你语言生态的 JSON Schema → 类型生成器生成模型，
   或按 `$defs` 手写但必须用 `-check` 之外的等价测试锁定字段（字段名/类型/可选性）。
2. 读 `host-contract.md`，实现进程调用、超时、状态机、确认事件与错误分类。
3. 复用 `client/aly-client-sdk/AlyClient.CSharpSDK` 的行为作为参考实现；不要发明新的语义。
4. 部署更新器与 `client.json` 的方式由各生态自己决定（NuGet targets / npm postinstall /
   Go embed 等），参考 C# 的 `AlyClient.CSharpSDK.targets`。

## 常见错误

- 只改 schema 不改 `dto.go`：`-check` 报“schema property is not emitted by Go struct”。
- 只改 `dto.go` 不改 schema：`-check` 报“Go field is missing from schema”。
- 手改 `Models/Generated/*.g.cs`：下次 `-write` 会被覆盖，`-check` 会失败。
- 把严格 schema 当成 SDK 的解析规则：宿主必须容忍未知字段。