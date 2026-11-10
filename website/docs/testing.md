# 测试体系

Aly 用**仓库内唯一的权威测试文档**（`aly测试用例文档.md`，当前 v11）管理全部端到端用例：135 条，编号连续 1..N，每条自带"前置条件 → 步骤 → 期望结果 → 失败判定"，单条即可独立复现。当前口径：**131 PASS / 0 FAIL / 1 SKIP / 3 未执行**。

## E2E 测试框架（test/tc）

Go 驱动的测试 harness，不是 PowerShell 脚本拼凑：

- 每条用例是一个标准 `go test` 函数，共享 `tc` 包提供的环境编排（起服务端、构建客户端、布置目录、断言 JSON 输出）；
- 双工具链感知：harness 会调用 Go 1.10.8 构建 XP 口径的客户端 fake、用现代 Go 构建服务端（工具链路径可用 `ALY_GO126` / `ALY_GO1108` 环境变量覆盖）；
- 用例组覆盖：正常流程（检查/下载/应用）、第四批新特性、崩溃恢复、断点续传、回滚、并发锁、自更新统一路径、publish-cli/gui 全命令。

## 用例分布（节选）

| 分组 | 编号 | 覆盖内容 |
|---|---|---|
| 正常流程 | 1–8 | check → download → apply 全链路 |
| 第四批新特性 | 9–17 | 停滞超时、失败文件重试、semver 比较 |
| 崩溃与恢复 | — | `applying` 状态恢复、断电注入、状态守卫 |
| 回滚与清理 | — | rollback 全路径、clean 快照保留策略 |
| 并发与锁 | — | 双实例并发触发更新的互斥 |
| 自更新 | 130–135 | `self_update` 统一执行、UI/SDK 委派与兜底、防循环、收尾进程 |
| publish-cli / gui | — | 全命令、分片上传边界、初始化原子写 |

## 单元测试

除 E2E 外，核心包有独立单元测试（Linux 与 Windows 均可运行）：

| 包 | 数量 | 覆盖 |
|---|---|---|
| `selfupdate/` | 14 | 哈希比对、替换编排、运行中 exe 改名舞步、失败回滚、Journal 防循环 |
| `cmd/` | 40+ | 各命令 JSON 协议、锁语义、脚本执行 |
| `config/` / `util/` / `model/` | 60+ | 配置加载、平台工具函数 |
| linuxui | 20 | 收尾进程、替换编排、参数解析 |

XP 轨约束下的测试同样遵守 Go 1.10.8 兼容性（UI 测试不使用 `t.Cleanup` 等新 API），保证测试与产物同口径。

## CI 中的测试

`release.yml` 在两条轨上并行执行：

- **Windows runner**：XP 轨构建（386）+ PE 头/manifest 校验 + 服务端/发布工具矩阵构建；
- **Ubuntu runner**：Linux 客户端构建（amd64/arm64）+ `mygo vet` + Linux 全包单元测试。

## 如何运行

```bash
# 全部 E2E（需要 Go 1.10.8 与现代 Go，Linux 默认取 PATH 与 ~/sdk/go1.10.8）
cd test/tc && go test ./...

# 单条用例（示例：自更新统一执行路径）
cd test/tc && go test -run TestCase130 ./...

# 客户端单元测试（Linux）
cd client/aly-client && go test ./cmd/ ./config/ ./util/ ./selfupdate/
```
