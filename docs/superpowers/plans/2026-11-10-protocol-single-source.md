# 协议单一真源与 C# 模型生成 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 把 `aly-client` 的 JSON 协议收敛到 `spec/aly-protocol.schema.json` 单一真源，用 `tools/alyprotocol` 生成 C# 模型，并让 CI 能拦截 schema / Go 客户端 / C# 生成物三向漂移。

**架构：** schema（语言中立）为真源；Go 工具读取 schema 与 `model/dto.go`，做双向漂移校验后渲染 C#；host contract 以规范文档承载 schema 表达不了的状态机、超时与错误语义。C# SDK 公开 API 不变。

**技术栈：** JSON Schema draft-07、Go 1.21+（标准库，零依赖）、.NET Framework 4.0 / netstandard2.0、Newtonsoft.Json、GitHub Actions。

**规格：** `docs/superpowers/specs/2026-11-10-protocol-single-source-design.md`

**执行方式：** 当前会话内联执行（TDD：先写失败测试，再实现；每个任务结束跑对应验证）。

---

## 文件结构

```text
spec/
├── README.md                          # 真源说明、修改流程、版本策略
├── aly-protocol.schema.json           # 协议真源（信封 + 11 个 $defs）
└── host-contract.md                   # 宿主行为规范（规范性文档）
tools/alyprotocol/
├── go.mod                             # module aly/tools/alyprotocol，go 1.21
├── main.go                            # CLI：-write / -check / -root
├── config.go                          # csharp.json 读取
├── schema.go                          # 保序解析 + 自校验
├── csharp.go                          # C# 渲染
├── gotags.go                          # dto.go 解析 + 漂移校验
├── csharp.json                        # 生成配置
├── ordered_test.go / schema_test.go
├── csharp_test.go / gotags_test.go
└── main_test.go                       # 对真实仓库做 -check 一致性测试
client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/Models/Generated/
└── *.g.cs                             # 生成物
```

---

### 任务 1：写 spec（schema + host contract + README）

**文件：**
- 创建：`spec/aly-protocol.schema.json`
- 创建：`spec/host-contract.md`
- 创建：`spec/README.md`

- [ ] **步骤 1：写 `spec/aly-protocol.schema.json`**

内容要求（与 `client/aly-client/model/dto.go` 逐字段对齐，字段表见规格 §5.4）：

- 顶层：`$schema`（draft-07）、`$id`、`title`、`description`、`x-protocol-version: "1.0.0"`。
- `$defs` 依次：`Output`、`CheckUpdateData`、`CheckDiffData`、`DiffFileItem`、
  `DownloadUpdateData`、`DownloadProgress`、`RollbackListData`、`RollbackData`、
  `SelfUpdateData`、`CheckSelfUpdateData`、`CleanData`。
- 每个对象 `additionalProperties: false`，`required` 严格对齐 Go 的 `omitempty`/指针；
  `status`、`mode` 用 `enum`；`Output.data` 不写 `type`（允许 null / 任意载荷）。
- `description` 用中文，一句话说清语义；不写实现细节（在 host contract 里写）。

- [ ] **步骤 2：写 `spec/host-contract.md`**

按规格 §6 的 11 条组织成规范性章节，每节给出可执行的判定规则与默认值，
并附命令到载荷映射表（`check_update → CheckUpdateData` 等）。必须包含：

- 进程调用约定（工作目录、无窗口、UTF-8、无 stdin）
- 退出码语义（只看 `isSuccess`）
- 超时表（30s / 1800s / 600s / 30s / 5s / 300s+抖动）
- 状态机与断网复位、`Cancel`
- 确认事件语义（同步、无拒绝）
- 下载流（完成信号 `data:null`、进度行也是信封、只认 `DONE`、`already at latest version`）
- 自更新委派与锁
- 部署布局、`client.json` / `shared.json`
- 锁与并发、参数约定、错误分类表

- [ ] **步骤 3：写 `spec/README.md`**

说明：真源是谁、改字段的完整流程、兼容策略（MINOR/MAJOR）、C# 生成命令、
后续语言如何消费（读 schema + contract，不手抄字段）。

- [ ] **步骤 4：校验 JSON 语法**

运行：

```powershell
python -c "import json;json.load(open('spec/aly-protocol.schema.json',encoding='utf-8'));print('json ok')"
```

预期：`json ok`。

- [ ] **步骤 5：Commit**

```bash
git add spec
git commit -m "feat(spec): 协议单一真源 schema 与宿主行为契约"
```

---

### 任务 2：工具骨架 + 保序解析

**文件：**
- 创建：`tools/alyprotocol/go.mod`
- 创建：`tools/alyprotocol/schema.go`
- 测试：`tools/alyprotocol/schema_test.go`

- [ ] **步骤 1：建模块**

```text
module aly/tools/alyprotocol

go 1.21
```

- [ ] **步骤 2：写失败测试**（`schema_test.go`）

```go
package main

import "testing"

func TestParseSchemaKeepsDefinitionAndPropertyOrder(t *testing.T) {
	raw := []byte(`{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "x-protocol-version": "1.0.0",
	  "$defs": {
	    "Second": {"type": "object", "required": ["b"], "additionalProperties": false,
	      "properties": {"b": {"type": "string"}, "a": {"type": "string"}}},
	    "First": {"type": "object", "required": [], "additionalProperties": false,
	      "properties": {"x": {"type": "integer"}}}
	  }
	}`)
	s, err := ParseSchema(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.DefNames(); len(got) != 2 || got[0] != "Second" || got[1] != "First" {
		t.Fatalf("def order = %v", got)
	}
	if got := s.Def("Second").PropertyNames(); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("property order = %v", got)
	}
	if s.ProtocolVersion != "1.0.0" {
		t.Fatalf("version = %q", s.ProtocolVersion)
	}
}

func TestValidateRejectsRequiredNotInProperties(t *testing.T) {
	raw := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0",
	  "$defs":{"X":{"type":"object","required":["nope"],"additionalProperties":false,
	  "properties":{"a":{"type":"string"}}}}}`)
	if _, err := ParseSchema(raw); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **步骤 3：运行确认失败**

```powershell
cd tools/alyprotocol; go test ./...
```

预期：FAIL，`ParseSchema` 未定义。

- [ ] **步骤 4：实现 `schema.go`**

要点：

- `ParseSchema([]byte)` → `*Schema`，用 `json.Decoder` 交替 `Token()` 与 `Decode()`
  保序解析 `$defs` 与每个 `properties`（禁止用 map 决定顺序）。
- 结构体：`Schema{ProtocolVersion string; Defs []*Object; byName map[string]*Object}`、
  `Object{Name, Description string; Properties []*Property; Required []string}`、
  `Property{Name, Description, Type, Ref, ItemsType, ItemsRef string; Enum []string}`。
- 校验：版本号 `\d+\.\d+\.\d+`；每个对象 `additionalProperties` 必须为 `false`；
  `required ⊆ properties` 且不重复；属性名匹配 `^[a-z0-9_]+$`；
  `$ref` 必须指向存在的 def；`enum` 只能出现在 `type=string` 上。
- 提供 `Def(name)`、`DefNames()`、`Object.Property(name)`、`Object.PropertyNames()`。

- [ ] **步骤 5：运行确认通过**

```powershell
cd tools/alyprotocol; go test ./...
```

预期：PASS。

- [ ] **步骤 6：Commit**

```bash
git add tools/alyprotocol
git commit -m "feat(tools): alyprotocol 保序解析协议 schema"
```

---

### 任务 3：C# 渲染

**文件：**
- 创建：`tools/alyprotocol/config.go`、`tools/alyprotocol/csharp.go`、`tools/alyprotocol/csharp.json`
- 测试：`tools/alyprotocol/csharp_test.go`

- [ ] **步骤 1：写失败测试**

断言：

1. `Output` → `AlyResponse`，`public partial class`，只生成 `IsSuccess`/`ErrorMsg`；
2. `DownloadProgress.file_size` 按覆盖渲染为 `double`，`index` 为 `long`；
3. 属性顺序与 schema 一致；
4. 文件头包含 `<auto-generated>` 与重新生成命令；
5. 可选字段不出现 `?`（C# 5 兼容）；
6. `internal` 可见性正确。

- [ ] **步骤 2：实现 `config.go`**

```go
type Config struct {
	Types      map[string]TypeConfig   `json:"types"`
	Properties map[string]PropOverride `json:"properties"`
	Go         struct {
		Structs map[string]string `json:"structs"`
	} `json:"go"`
}
type TypeConfig struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Visibility string `json:"visibility"`
	Partial    bool   `json:"partial"`
}
type PropOverride struct{ Type string `json:"type"` }
```

缺省：`Name` 为空取 def 名、`Visibility` 为空取 `public`；未知字段名报错。

- [ ] **步骤 3：实现 `csharp.go`**

- `Render(schema, cfg) ([]GeneratedFile, error)`，`GeneratedFile{Name string; Content []byte}`。
- 类型映射：`string→string`、`boolean→bool`、`integer→long`、数组标量→`T[]`、
  数组 `$ref`→`Name[]`、`$ref` 对象→`Name`、无 `type`→只在 `Output.data` 出现（跳过）。
- 只生成 `cfg.Types` 中列出的 def；`Output` 跳过 `data`；`partial` 时加 `partial`。
- 生成文件 UTF-8 无 BOM、LF、无时间戳；头部固定：

```csharp
// <auto-generated>
// 本文件由 tools/alyprotocol 从 spec/aly-protocol.schema.json 生成，请勿手工编辑。
// 重新生成：cd tools/alyprotocol && go run . -write
// </auto-generated>
```

- XML 转义 `& < >`；`description` 按行渲染为 `/// `。

- [ ] **步骤 4：写 `csharp.json`**

按规格 §7.4 的完整内容（7 个生成类型 + 11 个 Go 映射 + `file_size` 覆盖）。

- [ ] **步骤 5：运行测试**

```powershell
cd tools/alyprotocol; go test ./...
```

预期：PASS。

- [ ] **步骤 6：Commit**

```bash
git add tools/alyprotocol
git commit -m "feat(tools): 从 schema 渲染 C# 模型"
```

---

### 任务 4：`dto.go` 漂移校验

**文件：**
- 创建：`tools/alyprotocol/gotags.go`
- 测试：`tools/alyprotocol/gotags_test.go`

- [ ] **步骤 1：写失败测试**

用内联 Go 源码字符串覆盖：`omitempty`、指针、`[]string`、`interface{}`、
`json:"-"`；断言 4 类错误各自命中：

- schema 有、Go 无
- Go 有、schema 无
- 必填字段带 `omitempty`
- 类型不匹配

- [ ] **步骤 2：实现 `gotags.go`**

- `ParseGoStructs([]byte) (map[string]*GoStruct, error)`：`go/parser`（无类型检查），
  遍历 `*ast.StructType`；字段类型字符串支持 `Ident`、`*StarExpr`、`[]T`、`interface{}`。
- `Verify(schema, cfg, structs) []error`：按规格 §8 的 5 条规则双向校验；
  错误按 `def/prop` 排序，信息包含 def 名、Go 结构体名与字段名。

- [ ] **步骤 3：运行测试**

```powershell
cd tools/alyprotocol; go test ./...
```

预期：PASS。

- [ ] **步骤 4：Commit**

```bash
git add tools/alyprotocol
git commit -m "feat(tools): 校验 schema 与 Go 客户端字段漂移"
```

---

### 任务 5：CLI 与仓库一致性测试

**文件：**
- 创建：`tools/alyprotocol/main.go`
- 测试：`tools/alyprotocol/main_test.go`

- [ ] **步骤 1：写失败测试**

`main_test.go` 从包目录用 `root = "../.."` 加载真实 spec / `dto.go` / 配置，
断言：漂移校验为零错误、渲染结果与 `Models/Generated` 一致（归一化 CRLF/BOM）。

- [ ] **步骤 2：实现 `main.go`**

- 参数：`-root`、`-write`、`-check`；两者都未给时打印用法并退出码 2。
- 从 cwd 向上查找 `spec/aly-protocol.schema.json` 确定根；`-root` 优先。
- 顺序：读 schema/config/dto → 自校验 → 漂移校验 → 渲染 →
  `-check` 比对（missing/stale/unexpected/mismatch）或 `-write` 写盘并清理多余 `*.g.cs`。
- 比对归一化：CRLF→LF、去 UTF-8 BOM。
- 失败信息带文件路径；成功打印各类文件计数。

- [ ] **步骤 3：运行**

```powershell
cd tools/alyprotocol; go run . -check
```

预期：退出码 1（生成物尚不存在），报 `missing`。

- [ ] **步骤 4：Commit**

```bash
git add tools/alyprotocol
git commit -m "feat(tools): alyprotocol CLI 与仓库一致性检查"
```

---

### 任务 6：生成 C# 模型并迁移 SDK

**文件：**
- 生成：`Models/Generated/*.g.cs`（7 个）
- 删除：`Models/{CheckUpdateData,CheckSelfUpdateData,ListRollbackData,RollbackData,SelfUpdateData,DownloadProgressData}.cs`
- 修改：`Models/AlyResponse.cs`

- [ ] **步骤 1：生成**

```powershell
cd tools/alyprotocol; go run . -write; go run . -check
```

预期：`-check` 退出码 0。

- [ ] **步骤 2：迁移 `AlyResponse.cs`**

改成：

```csharp
using System;

namespace AlyClient.CSharpSDK.Models
{
    public partial class AlyResponse
    {
        public static AlyResponse OK() { /* 原有实现 */ }
        public static AlyResponse NG(string errMsg) { /* 原有实现 */ }
        public static AlyResponse NG(Exception e) { /* 原有实现 */ }
    }

    public class AlyResponse<T> : AlyResponse
    {
        [Newtonsoft.Json.JsonProperty("data")]
        public T Data { get; set; }
        public static AlyResponse<T> OK(T data) { /* 原有实现 */ }
        public static new AlyResponse<T> NG(string errMsg) { /* 原有实现 */ }
        public static new AlyResponse<T> NG(Exception e) { /* 原有实现 */ }
    }
}
```

`IsSuccess` / `ErrorMsg` 移入 `Models/Generated/AlyResponse.g.cs`。

- [ ] **步骤 3：编译双 TFM**

```powershell
dotnet build client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/AlyClient.CSharpSDK.csproj -v q --nologo
```

预期：0 警告 0 错误。

- [ ] **步骤 4：编译 smoke 与 sample**

```powershell
dotnet build client/aly-client-sdk/AlyClient.CSharpSDK/AlySdkSmoke/AlySdkSmoke.csproj -v q --nologo
dotnet build client/aly-client-sdk/samples/MinimalHost/MinimalHost.csproj -v q --nologo
```

预期：均成功。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client-sdk
git commit -m "refactor(sdk): C# 模型改为从协议 schema 生成"
```

---

### 任务 7：CI 与文档

**文件：**
- 修改：`.github/workflows/release.yml`（`build-csharp-sdk` 任务）
- 修改：`website/docs/sdk/api.md`、`website/en/docs/sdk/api.md`

- [ ] **步骤 1：CI 加校验**

在 `dotnet pack` 之前：

```yaml
      - name: Setup Go (protocol tool)
        uses: actions/setup-go@v5
        with:
          go-version-file: tools/alyprotocol/go.mod

      - name: Verify protocol models are generated
        working-directory: tools/alyprotocol
        run: go run . -check
```

- [ ] **步骤 2：文档加一句**

中文与英文 SDK API 页各加一段：模型由 `spec/aly-protocol.schema.json` 生成，
改字段请改 schema 并运行生成器，链接到仓库 `spec/README.md`。

- [ ] **步骤 3：提交**

```bash
git add .github/workflows/release.yml website
git commit -m "ci(sdk): 校验协议生成物；docs: 说明模型来源"
```

---

### 任务 8：最终验证与审查

- [ ] **步骤 1：跑完整验证矩阵**

```powershell
cd tools/alyprotocol
go vet ./...
go test ./...
go run . -check
cd ../..
dotnet build client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/AlyClient.CSharpSDK.csproj -v q --nologo
dotnet build client/aly-client-sdk/AlyClient.CSharpSDK/AlySdkSmoke/AlySdkSmoke.csproj -v q --nologo
dotnet build client/aly-client-sdk/samples/MinimalHost/MinimalHost.csproj -v q --nologo
```

预期：全部 0 退出码。

- [ ] **步骤 2：反向测试漂移检测**

临时改 schema 一个属性名 → `go run . -check` 必须退出码 1 并指出该字段；
`git checkout -- spec/aly-protocol.schema.json` 恢复。
再临时改 `dto.go` 一个 json tag → 同样必须红灯；恢复。
把两条输出记录下来（用于最终汇报）。

- [ ] **步骤 3：审查 diff**

重点：公开类型/命名空间/可见性无变化；生成文件无手工改动；
`AlyResponse` 仍是 `partial` + 泛型 + 工厂；`DownloadProgressData` 仍 `internal`。

- [ ] **步骤 4：修复审查问题并重新验证**

- [ ] **步骤 5：最终提交**

```bash
git add -A
git commit -m "chore(sdk): 协议生成落地后的审查修复"
```