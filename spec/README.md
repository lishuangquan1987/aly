# spec/ — aly 客户端协议单一真源

本目录定义 `aly-client` 与宿主之间的 JSON 协议。任何语言的 SDK 都必须以这里为准，
**不要**从 C# / Go 代码里手抄字段。

| 文件 | 作用 | 规范性 |
|---|---|---|
| `aly-protocol.schema.json` | 字段真源：信封、8 条命令的载荷、下载进度行与嵌套文件项 | 是（机器可校验） |
| `fixtures/scenarios/*.json` | 可回放的命令输出样例（跨语言 SDK 的一致性测试向量） | 是（`-check` 逐行校验） |
| `host-contract.md` | schema 表达不了的行为：调用约定、超时、状态机、确认语义、错误分类 | 是（人读） |
| `README.md` | 修改流程与兼容策略 | 本文件 |

## 修改协议的正确流程

1. 改 `aly-protocol.schema.json`（新增/删除/改名字段），必要时同步 `host-contract.md`。
2. 改 `client/aly-client/model/dto.go`，让客户端真的输出/不再输出该字段。
3. 重新生成各语言模型（当前 C#、TypeScript、Go、Python、Java、Rust）：

   ```bash
   cd tools/alyprotocol
   go run . -write
   go run . -check
   ```

   新增字段或命令时，同步更新 `spec/fixtures/scenarios/` 里的样例——`-check` 会校验它们。

4. 编译对应 SDK（C#：`dotnet build` 双 TFM；TypeScript：`npm test`；Go：`go test ./...`；Python：`python -m unittest discover -s tests`；Java：`bash scripts/build.sh out` 后运行一致性用例；Rust：`cargo test`）。
5. 提交：schema、dto.go、fixtures、生成物、受影响文档必须在同一个 commit 里。

`go run . -check` 会做四件事，任何一件失败都退出码 1：

- 校验 schema 自身（`required` 必须存在、`$ref` 必须可解析、`additionalProperties:false`、命名规范）；
- 校验 schema ↔ `dto.go` 的字段名、类型、必填/可选（双向）；
- 逐行校验 `fixtures/scenarios/*.json`，并要求 9 条命令都有样例；
- 比对生成物与磁盘（CRLF/BOM 归一化后逐字节）。

CI 在打包 C# SDK、运行 TypeScript/Go/Python/Java/Rust SDK 测试之前执行同一个 `-check`，漂移会直接红灯。

## fixtures：可回放的协议样例

`spec/fixtures/scenarios/*.json` 按命令保存输出形态，既是对 schema 的样例，也是假客户端/跨语言一致性测试的脚本：

```json
{
  "name": "update_available",
  "commands": {
    "check_update": {"lines": [{"isSuccess": true, "errorMsg": "", "data": {"has_update": true}}]},
    "download_update": {"lines": ["…进度行…", {"isSuccess": true, "errorMsg": "", "data": null}], "line_delay_ms": 10}
  }
}
```

- `-check` 逐行校验：信封字段、载荷类型/必填/未知字段、`download_update` 的 `data:null` 位置、最后一行带载荷必须是失败、`exit_code` 必须为 0。
- `-check` 还要求 **9 条命令都至少有一个场景**；新增命令忘记补样例同样会失败。
- 新增/修改命令输出时**必须**同步更新场景；场景失败说明 schema 或样例有一方漂移。
- `line_delay_ms` 供假客户端模拟流式输出（见下节）。

### 本地回放（假客户端）

`tools/fake-aly-client` 按场景回放 stdout，供各语言 SDK 在无网络、无真实更新器时测试：

```bash
cd tools/fake-aly-client
go build -o fake-aly-client.exe .

# SDK 测试把更新器路径指向它，并设置 ALY_FAKE_SCENARIO
# 路径要用绝对路径：SDK 会把子进程工作目录设为 UpdateFolder
ALY_FAKE_SCENARIO="<repo>/spec/fixtures/scenarios/update_available.json" ./fake-aly-client.exe check_update
```

- 命令之后的参数（`--must-close-process-name`、`--close-timeout` 等）会被忽略。
- 只回放 stdout 与退出码：不会关闭宿主、不替换文件、不重启主程序。`apply_update` 的副作用需要测试方自行模拟/断言。
- 场景本身先经 `go run . -check`（`tools/alyprotocol`）校验，假客户端不再重复校验。

现有协议级用例（同一套 fixtures）：

- **C#**：`AlySdkConformance.exe <fake-aly-client.exe> spec/fixtures/scenarios`，覆盖 check/download/部分失败/停流/apply/自更新/回滚/缺失更新器。
- **TypeScript**：`cd client/aly-client-sdk/AlyClient.TypeScriptSDK && npm test`，自动构建假客户端并回放同一批场景。
- **Go**：`cd client/aly-client-sdk/AlyClient.GoSDK && go test ./...`，测试内构建假客户端。
- **Python**：`cd client/aly-client-sdk/AlyClient.PythonSDK && python -m unittest discover -s tests -v`。
- **Java**：`cd client/aly-client-sdk/AlyClient.JavaSDK && bash scripts/build.sh out`，再运行
  `java -cp out io.github.lishuangquan1987.alyclient.tests.TestMain <fake> spec/fixtures/scenarios`。
- **Rust**：`cd client/aly-client-sdk/AlyClient.RustSDK && cargo test`，测试内构建假客户端。

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

1. 读 `aly-protocol.schema.json`，在 `tools/alyprotocol` 里为该语言加渲染器并纳入 `-check`；
   不要手抄模型（C# 与 TypeScript 都是这样生成的）。
2. 读 `host-contract.md`，实现进程调用、超时、状态机、确认事件与错误分类。
3. 参考现有实现：`AlyClient.CSharpSDK`（事件驱动、net40）、`AlyClient.TypeScriptSDK`
   （Promise/回调、Node ≥18）、`AlyClient.GoSDK`（context/回调、Go ≥1.21）、
   `AlyClient.PythonSDK`（threading/回调、Python ≥3.9）、`AlyClient.JavaSDK`
   （threading/回调、Java 8+）与 `AlyClient.RustSDK`（trait 对象回调、Rust stable）；
   不要发明新的语义。
4. 部署更新器与 `client.json` 的方式由各生态自己决定（NuGet targets / npm 本地依赖 /
   Go embed 等），参考 C# 的 `AlyClient.CSharpSDK.targets` 与 TS SDK 的 README。

## 常见错误

- 只改 schema 不改 `dto.go`：`-check` 报“schema property is not emitted by Go struct”。
- 只改 `dto.go` 不改 schema：`-check` 报“Go field is missing from schema”。
- 手改 `Models/Generated/*.g.cs`：下次 `-write` 会被覆盖，`-check` 会失败。
- 手改 TypeScript 的 `src/models.ts`、Go 的 `models_gen.go`、Python 的
  `alyclient/models_gen.py`、Java 的 `.../models/*.java` 或 Rust 的
  `src/models_gen.rs`：同样会被覆盖，`-check` 会失败。
- 把严格 schema 当成 SDK 的解析规则：宿主必须容忍未知字段。