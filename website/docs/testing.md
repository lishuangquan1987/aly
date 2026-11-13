---
title: 测试体系
description: Aly 的 148 条端到端用例、单元测试分布与本地运行方式。
---

# 测试体系

Aly 用仓库内的 `aly测试用例文档.md` 管理端到端用例：**148 条**，编号连续 1..148，每条自带“前置条件 → 步骤 → 期望结果 → 失败判定”，可独立复现。最近一次全量回归结果以该文档为准，不在官网固化 PASS 数字。

## E2E 框架（test/tc）

Go 驱动的 harness，不是脚本拼凑：

- 每条用例是一个标准 `go test` 函数，共享 `tc` 包的环境编排（起服务端、构建客户端、布置目录、断言 JSON）；
- 双工具链感知：XP 轨用 Go 1.10.8 构建 fake，现代轨构建服务端；路径可用 `ALY_GO126` / `ALY_GO1108` 覆盖；
- 覆盖正常流、崩溃注入、断点续传、回滚、并发锁、自更新统一路径、发布 CLI/GUI 全命令。

| 分组 | 覆盖内容 |
|---|---|
| 正常流程 | check → download → apply 全链路 |
| 新特性 | 停滞超时、失败文件重试、semver 比较 |
| 崩溃与恢复 | `applying` 恢复、断电注入、状态守卫 |
| 回滚与清理 | rollback 全路径、clean 快照保留 |
| 并发与锁 | 双实例并发触发更新 |
| 自更新 | 统一执行入口、UI/SDK 委派、防循环、Linux 收尾进程 |
| 发布工具 | 全命令、分片上传边界、初始化原子写、发布 GUI |

## 单元测试

| 包 | 覆盖 |
|---|---|
| `selfupdate/` | 哈希比对、替换编排、运行中 exe 改名舞步、回滚、Journal 防循环 |
| `cmd/` | 各命令 JSON 协议、锁语义、脚本执行 |
| `config/` / `util/` / `model/` | 配置加载、平台工具函数 |
| `linuxui` | 收尾进程、替换编排、参数解析 |
| `publish-gui` tests | 配置、版本、CLI 输出解析等（38 个） |

XP 轨约束下的测试同样遵守 Go 1.10.8 兼容性（不使用 `t.Cleanup` 等新 API）。

## 运行

```bash
# 全部 E2E（需要双 Go 工具链）
cd test/tc && go test ./...

# 单条用例
cd test/tc && go test -run TestCase130 ./...

# 更新器单元测试（Linux）
cd client/aly-client && go test ./cmd/ ./config/ ./util/ ./selfupdate/

# 发布 GUI
cd publish/publish-gui && dotnet test tests/AlyPublish.Tests/AlyPublish.Tests.csproj
```

## CI

`release.yml` 在两条轨上并行：

- Windows runner：XP 轨构建（386）+ PE/manifest 校验 + 服务端/发布工具矩阵构建；
- Ubuntu runner：Linux 客户端构建（amd64/arm64）+ Linux 单元测试 + 发布 GUI 测试。