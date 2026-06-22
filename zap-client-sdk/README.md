# zap-client-sdk — 更新客户端 SDK

供 `zap-client`（可执行程序）引用的共享库，包含配置管理、HTTP API 客户端、文件工具、进程管理等模块。

## 模块

| 目录 | 说明 |
|------|------|
| `config/` | `client.json` / `shared.json` / `version.json` 加载与状态管理 |
| `client/` | HTTP API 客户端（调用服务端接口） |
| `model/` | JSON DTO 数据结构 |
| `util/` | 文件操作（MD5/SHA256/拷贝/遍历）、进程管理 |
| `test/` | Mock 服务端（用于本地测试） |

## 依赖

| 用途 | 库 | 说明 |
|------|-----|------|
| HTTP 请求 | 标准库 `net/http` | Go 1.10 内置 |
| JSON 序列化 | 标准库 `encoding/json` | Go 1.10 内置 |
| SHA-256 | 标准库 `crypto/sha256` | Go 1.10 内置 |
| MD5 | 标准库 `crypto/md5` | Go 1.10 内置 |
| 进程管理 | `golang.org/x/sys/windows` | Windows API |

## 兼容性

- Go 1.10（兼容 Windows XP，不使用 Go Modules）
- GOARCH=386
- 100% 标准库 + golang.org/x/sys/windows
