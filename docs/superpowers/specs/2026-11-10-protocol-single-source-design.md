# Aly 协议单一真源与 C# 模型生成（设计）

- **日期**：2026-11-10
- **状态**：已与用户确认，待编写实现计划
- **范围**：C# SDK 协议层（schema + host contract + 生成器 + C# 模型迁移）

## 1. 背景与问题

`aly-client` 的命令行协议是子进程 + 单行 JSON：9 条命令统一输出
`{"isSuccess":bool,"errorMsg":string,"data":any}`，`download_update` 先输出多行进度
（同样是信封包装），最后一行才是结果。协议的真实定义散落在三处：

| 位置 | 内容 | 问题 |
|---|---|---|
| `client/aly-client/model/dto.go` | Go 结构体与 json tag（真实发射端） | 改字段不会提醒任何 SDK |
| `client/aly-client-sdk/.../Models/*.cs` | 手写 C# 模型 | 已经漏字段：`failed_files`、`PROGRESS` 状态、`downloaded_bytes/total_bytes/speed_bps`、`error` |
| `website/docs/commands.md` | 面向使用者的命令文档 | 描述性文档，不能机器校验 |

一旦要支持第二种语言的 SDK，每门语言都手抄一遍模型，漂移会成倍放大（`release_notes` 就是
最近一次例子）。因此需要把协议收敛为**语言中立的单一真源**，C# 模型变成生成物。

## 2. 目标与非目标

### 目标

1. `spec/aly-protocol.schema.json` 成为协议字段的唯一真源（信封 + 9 条命令载荷 + 进度行）。
2. `spec/host-contract.md` 成为 schema 表达不了的行为契约（状态机、超时、错误语义、确认事件、
   部署与调用约定），供任何语言的 SDK 实现者阅读。
3. `tools/alyprotocol` 从 schema 生成 C# 模型，并提供 `-check` 模式供 CI 拦截漂移。
4. C# SDK 的**公开 API 与运行时行为不变**；生成物通过双 TFM 编译与现有冒烟/示例工程验证。
5. 为后续 Go / TypeScript SDK 打基础：它们读同一份 schema / host contract。

### 非目标（本轮不做）

- 真实 `aly-client` 输出抓取、假客户端、跨语言一致性用例（下一轮）。
- Go / TypeScript / C++ / Delphi 等 SDK 实现。
- `aly-client version` 握手命令、协议版本运行时协商。
- 把超时常量生成进代码（本轮只在 host contract 中规范化）。
- 修改 `website/docs/commands.md` 的既有命令语义（只加链接与生成说明）。

## 3. 架构总览

```text
                   ┌──────────────────────────────────────┐
                   │ spec/aly-protocol.schema.json (真源) │
                   └──────────────┬──────────┬────────────┘
                                  │          │
             ┌────────────────────┘          └──────────────────┐
             ▼                                                  ▼
  tools/alyprotocol (读取 schema + dto.go)            spec/host-contract.md
             │                                                  │
             ▼                                                  ▼
  C# 生成物 Models/Generated/*.g.cs              各语言 SDK 实现者的行为规范
```

三个方向的一致性由工具保证：

1. **schema → C#**：生成器输出，`-check` 逐字节比对。
2. **schema ↔ dto.go**：漂移校验（字段名、类型、必填/可选），双向报错。
3. **host contract ↔ 各 SDK**：人工评审 + 各 SDK 自己的测试（本轮没有机器校验，风险见 §12）。

## 4. 目录结构

```text
spec/
├── README.md                    # 真源说明、修改流程、版本策略、语言消费方式
├── aly-protocol.schema.json     # 唯一真源
└── host-contract.md             # 行为契约（规范性）
tools/alyprotocol/
├── go.mod                       # module aly/tools/alyprotocol（标准库，零依赖）
├── main.go                      # CLI：-write / -check / -root
├── schema.go                    # 保序解析 + schema 自校验
├── csharp.go                    # C# 渲染
├── gotags.go                    # dto.go 解析与漂移校验
├── csharp.json                  # C# 生成配置（命名空间/可见性/类型覆盖/Go 映射）
└── *_test.go                    # 保序解析、渲染 golden、tag 校验
client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/Models/Generated/
└── *.g.cs                       # 生成物（只由工具写入）
```

## 5. Schema 规范

### 5.1 文件与格式

- 单文件、JSON Schema **draft-07**，`$id` 指向仓库路径；顶层带
  `x-protocol-version: "1.0.0"`（自定义注解，不是校验关键字）。
- 所有类型定义放在 `$defs`，用 `$ref` 引用；不引入 `allOf`/`oneOf` 输出组合（信封与载荷分开表达，
  命令到载荷的映射写在 host contract 命令表里）。
- `description` 用中文，直接生成进 C# XML 文档。

### 5.2 覆盖范围

| `$defs` 名称 | 对应命令/输出 | C# 是否生成 |
|---|---|---|
| `Output` | 所有命令的信封 | 是（`AlyResponse` 的两个属性） |
| `CheckUpdateData` | `check_update` 的 data | 是 |
| `CheckDiffData` | `check_diff` 的 data | 否 |
| `DiffFileItem` | `CheckDiffData.files[]` | 否 |
| `DownloadUpdateData` | `download_update` 失败时的 data | 否 |
| `DownloadProgress` | `download_update` 的进度行 data | 是（`internal`） |
| `RollbackListData` | `list_rollback_versions` 的 data | 是 |
| `RollbackData` | `rollback` 的 data | 是 |
| `SelfUpdateData` | `self_update` 的 data | 是 |
| `CheckSelfUpdateData` | `check_self_update` 的 data | 是 |
| `CleanData` | `clean` 的 data | 否 |

`apply_update` 没有载荷（data 恒为 `null`），只在 host contract 中描述。

### 5.3 严格性与前向兼容

- 所有对象 `additionalProperties: false`。目的：客户端加了字段却没改 schema，`-check` 必须失败。
- 但**宿主/SDK 的反序列化必须忽略未知字段**（容错方向相反）。host contract 与
  `spec/README.md` 都要写死这条，避免后来的语言 SDK 把严格 schema 误当成解析规则。
- 可选字段用 `required` 表达；`$defs` 中不出现 `nullable`（`data:null` 通过命令语义表达）。

### 5.4 真实字段口径（必须与当前 `aly-client` 一致）

- `CheckUpdateData`：`has_update`、`need_download_update`、`current_version` 必填；
  `new_version`、`force_update`（Go 为 `*bool`）、`release_notes` 可选。
- `DownloadProgress`：`index`、`total`、`file`、`status`、`file_size` 必填；
  `error`、`downloaded_bytes`、`total_bytes`、`speed_bps` 可选（`omitempty`）。
  `status` 枚举：`BEGIN` / `START` / `PROGRESS` / `DONE` / `SKIP` / `FAIL`。
- `DownloadUpdateData`：`version` 必填、`failed_files` 可选；只在一部分文件失败时出现，
  成功完成行的 data 是 `null`。
- `CheckDiffData`：`new_version`、`files[]` 必填；`files[]` 项为 `DiffFileItem`
  （`path`、`local_md5`、`local_size`、`local_sha256`、`server_md5`、`server_size`、`server_sha256`）。
- `RollbackListData`：`current_version`、`versions[]`。
- `RollbackData`：`version`。
- `SelfUpdateData`：`updated`、`reason` 必填，`old_path` 可选。
- `CheckSelfUpdateData`：`need_update`。
- `CleanData`：`mode` 必填（`keep` / `all`），`keep` 可选。

### 5.5 协议版本与兼容策略

- `x-protocol-version` 采用 `MAJOR.MINOR.PATCH`：
  - 新增可选字段、新增命令 → **MINOR**
  - 删除字段、改字段名、改类型、改语义 → **MAJOR**
  - 文档/描述修正 → **PATCH**
- 本轮版本 `1.0.0`，不新增 `aly-client version` 命令；握手留到下一轮。
- `spec/README.md` 记录变更规则：改协议必须同时改 schema、跑生成器、更新版本号。

## 6. Host Contract 规范条款

`spec/host-contract.md` 是规范性文档（normative），至少包含：

1. **进程调用约定**：工作目录为 `UpdateFolder`；`UseShellExecute=false`；不弹控制台窗口；
   stdout/stderr 按 UTF-8 解析；不写 stdin。
2. **退出码语义**：`0` 包括业务失败；`1` 未提供/未知子命令或序列化失败；`2` 参数解析失败。
   **只看 `isSuccess`，不看退出码。**
3. **超时表**：
   - 短命令（`check_self_update`、`self_update`、`check_update`、`list_rollback_versions`）：30s
   - 下载：无总时长上限；停流看门狗 1800s（连续无输出即判死，下载中每 3s 至少一行）
   - 回滚：600s
   - `apply_update`：拉起后不等待；调用方 30s 内不要重复拉起
   - 网络失败重试：5s
   - 空闲轮询：默认 300s，最小 60s，叠加 0–60s 随机抖动
4. **状态机**：`None → DiscoveredUpdate → DownloadingUpdate → DownloadedUpdate → ApplyUpdate`
   （每阶段含义、断网时复位 `None`、`Cancel` 语义、下载后待应用状态不因断网丢失）。
5. **确认事件**：仅 `force_update=false` 时触发；同步调用、返回即确认；没有“拒绝”语义，
   只能 `Cancel` 整条流程；`force_update=true` 时两事件不触发。
6. **下载流解析**：完成信号 `data:null`；进度只认 `status=DONE` 且 `total>0` 的行；
   字节字段可选，缺失时退回文件计数进度；`already at latest version` 视为无更新；
   `status` 未知值必须容忍。
7. **自更新**：优先委派 `self_update`；目标缺失或命令失败且 `.aly.lock` 不存在时回退本地替换；
   锁被占用时不绕锁。
8. **部署布局**：`PackageFolder/ApplicationFolder/UpdateFolder/ApplicationFolder_<version>`；
   `client.json` 字段（`main_exe_relative_path`、`must_close_process_name`）；
   `.updator/shared.json` 字段；路径相对谁解析。
9. **锁与并发**：同一时刻一个更新流程；`apply_update` 不要双触发；宿主与 UI 并发安全。
10. **参数约定**：SDK 传 `--must-close-process-name`、`--close-timeout` 的规则与默认值。
11. **错误分类表**：客户端 `errorMsg` → SDK 应映射的行为（可重试 / 视为无更新 / 终止）。

文档中的命令到载荷映射表（`check_update → CheckUpdateData` 等）作为语言实现者的索引。

## 7. 生成器设计

### 7.1 命令与参数

```text
cd tools/alyprotocol
go run . -write            # 生成并写盘
go run . -check            # 只校验，退出码 1 表示漂移
go run . -root <repo>      # 显式指定仓库根（默认从模块目录向上查找）
```

- 独立 Go 模块（`module aly/tools/alyprotocol`），标准库，无第三方依赖。
- 默认路径（相对仓库根）：`spec/aly-protocol.schema.json`、
  `client/aly-client/model/dto.go`、
  `client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/Models/Generated`。
- 配置默认 `tools/alyprotocol/csharp.json`（路径相对模块目录解析）。

### 7.2 执行流程

1. 定位仓库根；读 schema、配置、`dto.go`。
2. 保序解析 schema 的 `properties`（用 `json.Decoder` token 流，不用 map，避免输出顺序抖动）。
3. schema 自校验：`required ⊆ properties`、`$ref` 可解析、类型合法、`enum` 只用于字符串。
4. 漂移校验（§8），失败即退出码 1，不写任何文件。
5. 渲染全部 C# 到内存。
6. `-write`：先算完所有内容，再写文件；清理 `Generated/` 下不在白名单的 `*.g.cs`。
   `-check`：归一化（CRLF→LF、去 UTF-8 BOM）后与磁盘逐字节比较，报告：
   `missing` / `stale` / `unexpected` / `content mismatch`。
7. 输出摘要（生成/删除/一致的文件数），失败信息带文件与字段名。

### 7.3 确定性要求

- 生成文件：UTF-8 无 BOM、LF、无时间戳、无随机顺序；文件头固定为：

```csharp
// <auto-generated>
// 本文件由 tools/alyprotocol 从 spec/aly-protocol.schema.json 生成，请勿手工编辑。
// 重新生成：cd tools/alyprotocol && go run . -write
// </auto-generated>
```

- 文件名 `{TypeName}.g.cs`，一个公开类型一个文件。

### 7.4 `csharp.json` 结构

```json
{
  "types": {
    "Output": {
      "name": "AlyResponse",
      "namespace": "AlyClient.CSharpSDK.Models",
      "visibility": "public",
      "partial": true
    },
    "CheckUpdateData": { "namespace": "AlyClient.CSharpSDK", "visibility": "public" },
    "CheckSelfUpdateData": { "namespace": "AlyClient.CSharpSDK", "visibility": "public" },
    "ListRollbackData": { "namespace": "AlyClient.CSharpSDK", "visibility": "public" },
    "RollbackData": { "namespace": "AlyClient.CSharpSDK", "visibility": "public" },
    "SelfUpdateData": { "namespace": "AlyClient.CSharpSDK.Models", "visibility": "public" },
    "DownloadProgressData": {
      "name": "DownloadProgressData",
      "namespace": "AlyClient.CSharpSDK",
      "visibility": "internal"
    }
  },
  "properties": {
    "DownloadProgressData.file_size": { "type": "double" }
  },
  "go": {
    "structs": {
      "Output": "Output",
      "CheckUpdateData": "CheckUpdateData",
      "CheckDiffData": "CheckDiffData",
      "DiffFileItem": "DiffFileItem",
      "DownloadUpdateData": "DownloadUpdateData",
      "DownloadProgress": "DownloadProgress",
      "RollbackListData": "RollbackListData",
      "RollbackData": "RollbackData",
      "SelfUpdateData": "SelfUpdateData",
      "CheckSelfUpdateData": "CheckSelfUpdateData",
      "CleanData": "CleanData"
    }
  }
}
```

`types` 只列需要生成的 C# 类型（`Output` 映射为 `AlyResponse`）；`go.structs` 列全部需要校验的定义，包含不生成 C# 的 `CheckDiffData` 等。

### 7.5 C# 渲染规则

| JSON Schema | C# |
|---|---|
| `string` | `string` |
| `boolean` | `bool` |
| `integer` | `long`（`file_size` 按配置覆盖为 `double`） |
| `array`（items 为标量） | `T[]` |
| `array`（items 为 `$ref`） | `StructName[]` |
| 对象 | `class`，属性 `{ get; set; }`（C# 5 语法） |
| 可选字段 | 不引入 `Nullable`；只在文档中体现可选性 |
| `enum` | `string`（避免来源不兼容，SDK 自己校验取值） |

- 属性带 `[JsonProperty("<json_name>")]`，并生成 `using Newtonsoft.Json;`。
- `description` 渲染为 `/// <summary>`（XML 转义 `&`、`<`、`>`）。
- `Output` 渲染为 `public partial class AlyResponse`，只有 `IsSuccess`、`ErrorMsg` 两个属性；
  `data` 属性由手写的泛型 `AlyResponse<T>` 承担，生成器跳过。

## 8. 漂移校验规则

对 `csharp.json` 中 `go.structs` 的每个映射（schema def ↔ Go struct）：

1. schema 的每个属性 → Go 必须有同 `json` tag 的字段；缺失报
   `schema property "<def>.<prop>" is not emitted by Go struct <Struct>`。
2. Go 的每个带 `json` tag 字段 → schema 必须有同名属性；缺失报
   `Go field "<Struct>.<Field>" (json:"<tag>") is missing from schema`。
3. 必填/可选一致：
   - `required` 含该属性 → Go 字段不得带 `omitempty`，不得为指针；
   - 不在 `required` → Go 字段必须带 `omitempty`（指针也必须配 `omitempty`，否则会输出 `null`，
     与 schema 的类型约束冲突）。
4. 类型兼容：`boolean↔bool/*bool`、`string↔string`、`integer↔int/int32/int64`、
   `array<string>↔[]string`、`array<$ref>↔[]Struct`、无 `type`（`data`）↔`interface{}`、
   `string enum↔string`。
5. 结构体名必须存在；`Output.Data` 是 `interface{}`，按规则 4 的“无 type”处理。

失败信息必须同时给出 schema 侧与 Go 侧的位置（def 名 / 字段名），便于直接修。

## 9. C# 迁移

| 动作 | 文件 |
|---|---|
| 新增（生成） | `Models/Generated/AlyResponse.g.cs` |
| 新增（生成） | `Models/Generated/{CheckUpdateData,CheckSelfUpdateData,ListRollbackData,RollbackData,SelfUpdateData,DownloadProgressData}.g.cs` |
| 删除 | `Models/CheckUpdateData.cs`、`CheckSelfUpdateData.cs`、`ListRollbackData.cs`、`RollbackData.cs`、`SelfUpdateData.cs`、`DownloadProgressData.cs` |
| 修改 | `Models/AlyResponse.cs`：`public partial class AlyResponse`，保留 `OK/NG` 与泛型 `AlyResponse<T>`，移除两个属性 |

约束：

- 所有公开类型名、命名空间、属性名、可见性保持现状（`DownloadProgressData` 仍为 `internal`）。
- 进度类 `Index` / `Total` 由 `int` 变 `long`（仅 `internal`，SDK 内 `> 0` 比较不受影响）。
- 生成代码必须通过 `net40` 与 `netstandard2.0` 编译；不使用 C# 5 以上语法。

## 10. CI 与文档

- `.github/workflows/release.yml` 的 `build-csharp-sdk` 任务，在 `dotnet pack` 之前增加：
  - `actions/setup-go@v5`，`go-version-file: tools/alyprotocol/go.mod`
  - `working-directory: tools/alyprotocol`，`run: go run . -check`
- 新增 `spec/README.md`：真源说明、修改流程、版本策略、C# 生成命令、未来语言如何消费。
- `website/docs/sdk/api.md` 与 `website/en/docs/sdk/api.md` 各加一句：模型由 schema 生成，
  修改请改 schema 并重新生成；链接到仓库 `spec/`。
- 不改 `AGENTS.md`。

## 11. 验收标准

以下命令全部通过，且人工 diff 审查无公开 API 变化：

1. `cd tools/alyprotocol && go vet ./... && go test ./...`
2. `go run . -write` 后再 `go run . -check`，退出码 0。
3. 故意改 schema 一个字段名（不改 dto.go）→ `-check` 退出码 1，报错含该字段。
4. 故意改 `dto.go` 一个 json tag（不改 schema）→ `-check` 退出码 1，报错含该字段。
5. `dotnet build client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/AlyClient.CSharpSDK.csproj`
   → 0 警告 0 错误（net40 + netstandard2.0）。
6. `dotnet build` `AlySdkSmoke` 与 `samples/MinimalHost` 成功。
7. `git status` 中生成物与生成器输出一致（无手工改动）。
8. 按 AGENTS.md 流程完成代码审查后提交。

## 12. 风险与对策

| 风险 | 对策 |
|---|---|
| 换行/BOM 导致 `-check` 跨平台误报 | 比对前归一化 CRLF 与 BOM；生成物无时间戳 |
| 严格 schema 与客户端演进冲突 | 这是设计目的；CI 失败信息带字段名；契约写明新增字段必须同步 schema |
| 迁移破坏 C# 公开 API | 白名单 + 双 TFM 编译 + smoke/sample 编译 + diff 审查 |
| 生成器失修 | 工具约 300 行、零依赖；单测覆盖保序解析、渲染、tag 校验 |
| host contract 与实现漂移 | 本轮人工评审；下一轮引入真实输出抓取与跨语言一致性用例 |
| 本轮没有运行时协议验证 | 明确列为下一轮；本轮由 Go tag 校验保证字段名/类型/可选性 |

## 13. 后续（不在本轮）

1. 抓取真实 `aly-client` 输出为 fixtures，加一个假客户端，把现有 sd1–sd15 迁移成协议级用例。
2. Go 与 TypeScript SDK：读同一份 schema 与 host contract，生成各自模型。
3. `aly-client version --json` + SDK 协议版本握手。
4. 超时常量与错误文案从 host contract 生成到各 SDK，消除最后一处手工同步。