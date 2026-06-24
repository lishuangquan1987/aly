---
name: zap-test-guide
description: Zap 项目测试指南 — client/publish-cli/CSharpSDK 每个指令的测试步骤和验证点
---

# Zap 项目测试指南

本文档描述如何完整测试 Zap 项目的三个核心组件。

---

## 一、测试目录约定

每次测试在 `%TEMP%` 下创建临时目录，测试完成后删除：

```
%TEMP%/zap-test-{timestamp}/
├── server-data/        # SQLite 数据库
├── UpdateFolder/       # zap-client.exe + client.json + version.json
├── AppFolder/          # 模拟被更新的应用目录
├── Runner/             # C# 测试程序（用于 SDK 测试，独立于 AppFolder）
├── test-project/       # publish-cli 操作的项目目录
├── gopath/             # 临时 GOPATH（构建 32 位 client）
├── zap-server.exe
├── zap-publish.exe
```

---

## 二、Client 指令测试

### 前置：编译

```bash
# 用 Go 1.10 GOPATH 模式构建 32 位 client
set GOARCH=386
set GO111MODULE=off
set GOPATH=%TEMP%/zap-test/gopath
xcopy /e /i client\zap-client %GOPATH%/src/zap/client/zap-client
cd %GOPATH%/src/zap/client/zap-client
go build -ldflags="-s -w" -o %TEMP%/zap-test/UpdateFolder/zap-client.exe .
```

### 前置：配置 client.json

```json
{
  "main_exe_relative_path": "../AppFolder/dummy.exe",
  "must_close_process_name": []
}
```

位于 `UpdateFolder/client.json`。`main_exe_relative_path` 指向模拟的应用目录。

### 前置：配置 shared.json

```json
{
  "server_url": "http://localhost:2000",
  "project_name": "test-project",
  "ignore_folders": [],
  "ignore_files": []
}
```

位于 `AppFolder/.updator/shared.json`。

### 前置：启动 server

```bash
zap-server.exe -p 2000 -db %TEMP%/zap-test/server-data/zap.db
```

### 前置：创建项目 + 推送版本

```bash
zap-publish.exe project create --server http://localhost:2000 --name test-project --title Test --json
zap-publish.exe config init --server http://localhost:2000 --project test-project --json
zap-publish.exe add --all --json
zap-publish.exe push --version V0.1.0 --message "first" --json
```

### 2.1 check_update

```bash
# 需要管理员权限（32 位 Go 二进制）
# 使用 Start-Process -Verb RunAs 或将测试程序设为管理员运行
zap-client.exe check_update
```

**验证点**：
- `isSuccess: true`
- `has_update` 根据 local version.json 与 server 最新版本比对
- `need_download_update` 字段存在
- 断网时 `applied` 状态返回 `has_update: false`
- `downloaded` 状态断网返回 `has_update: true, need_download_update: false`

### 2.2 check_diff

```bash
zap-client.exe check_diff
```

**验证点**：
- 返回 server 与 local 的差异文件列表
- 每个文件包含 `path`, `local_md5`, `local_size`, `server_md5`, `server_size` 及 SHA256
- 本地不存在的文件 `local_md5` 为空字符串

### 2.3 download_update

```bash
zap-client.exe download_update
```

**验证点**：
- 每行一个 JSON 进度：`index`, `total`, `file`, `status`(START/DONE/SKIP), `file_size`
- `total` = 服务端文件总数
- 所有文件都有对应的进度行（含本地已匹配的 SKIP）
- `version.json` 更新：`version` 为新版本，`status` 为 `downloaded`

**大文件断点续传测试**：
1. 推送一个 ≥10MB 的文件
2. 下载完成后，截断目标目录中的文件（模拟中断）
3. 重新 download_update → 文件从断点续传（Range 头）
4. `.part` 文件被清理

### 2.4 apply_update

```bash
zap-client.exe apply_update
```

**验证点**：
- `must_close_process_name` 为空时不会杀进程
- `version_status` 从 `downloaded` → `applying` → `applied`
- `AppFolder` 目录被原子替换
- 失败时状态回退到 `downloaded`

### 2.5 list_rollback_versions

```bash
zap-client.exe list_rollback_versions
```

**验证点**：返回当前版本和历史版本列表

### 2.6 rollback

```bash
zap-client.exe rollback --version 0.1.0
```

**验证点**：AppFolder 恢复到指定版本的快照

### 2.7 check_self_update

```bash
zap-client.exe check_self_update
```

**验证点**：
- 比较自身 SHA256 与 `AppFolder/zap-client.exe` 的 SHA256
- 不同 → `need_update: true`
- 相同 → `need_update: false`

---

## 三、Publish-CLI 指令测试

### 3.1 config init

```bash
cd test-project
zap-publish.exe config init --server http://localhost:2000 --project test-project --json
```

**验证点**：创建 `.updator/shared.json` + `.updator/publish.json`

### 3.2 config set / get / list / path

```bash
zap-publish.exe config set server.url http://localhost:2000 --json
zap-publish.exe config set-array ignore.folders --add logs --json
zap-publish.exe config set-array ignore.folders --remove logs --json
zap-publish.exe config set-array ignore.files --add "*.tmp" --json
zap-publish.exe config get server.url
zap-publish.exe config list --json
zap-publish.exe config path
```

**验证点**：读写配置正确，数组操作生效

### 3.3 project create / list / update / delete / info

```bash
zap-publish.exe project create --server http://localhost:2000 --name test --title Test --json
zap-publish.exe project list --server http://localhost:2000 --json
zap-publish.exe project update --server http://localhost:2000 --name test --title NewTitle --json
zap-publish.exe project info --server http://localhost:2000 --name test --json
zap-publish.exe project delete --server http://localhost:2000 --name test --json
```

**验证点**：CRUD 操作正常，`force_update` 字段可读写

### 3.4 status

```bash
zap-publish.exe status --json
```

**验证点**：正确显示 new / modified / deleted / unchanged / staged 文件分类

### 3.5 add / staged / reset

```bash
zap-publish.exe add --all --json
zap-publish.exe staged --json
zap-publish.exe reset --all --json
```

**验证点**：暂存区操作正确，JSON 输出格式一致

### 3.6 push

```bash
# 普通推送
zap-publish.exe push --version V0.1.0 --message "test" --json

# 强制更新
zap-publish.exe push --version V0.2.0 --message "force" --set-force-update --json

# 带脚本
zap-publish.exe push --version V0.3.0 --message "script" --after-apply-update-script post_update.bat --json

# 推送后验证 force_update 为 false
zap-publish.exe push --version V0.4.0 --message "normal" --json
# 检查服务端 force_update 应为 false
```

**验证点**：
- `--set-force-update` → 服务端 `force_update: true`
- 不带 `--set-force-update` → 服务端 `force_update: false`
- `--after-apply-update-script` → 版本日志含 `after_apply_update_script` 字段
- `--dry-run` 不推送
- 暂存区为空时返回错误

### 3.7 log / diff / server info

```bash
zap-publish.exe log --limit 5 --json
zap-publish.exe diff --json
zap-publish.exe server info --server http://localhost:2000 --json
```

**验证点**：
- `log` 返回的 changelog 含 `project_id` 字段
- `server info` 返回系统信息（OS, CPU, 磁盘）

---

## 四、ZapClient.CSharpSDK 测试

### 前置

1. 编译 SDK：`dotnet build ZapClient.CSharpSDK/ZapClient.CSharpSDK.csproj -c Release`
2. 创建 Runner 目录，添加 C# 测试项目引用 SDK
3. Runner 目录独立于 AppFolder（避免 apply_update 替换时杀进程）

### 4.1 自我更新测试（check_self_update）

**步骤**：
1. 将 `zap-client.exe` 复制到 `AppFolder/`（模拟发布者部署了新版）
2. 将 `zap-client.exe` 追加 1 字节放到 `UpdateFolder/`（模拟旧版）
3. C# 代码调用 `ZapApi.CheckSelfUpdateAsync(exePath)`
4. 验证 `NeedUpdate = true`
5. 执行 `File.Copy(src, dest, true)` 将新版覆盖旧版

**验证点**：
- SHA256 不同时返回 `NeedUpdate = true`
- File.Copy 成功后 UpdateFolder 的 exe 文件大小与 AppFolder 一致

### 4.2 非强制更新（ForceUpdate = false）

**步骤**：
1. Push V0.1.0（不带 `--set-force-update`）
2. local version.json 设为 V0.0.0
3. 启动 `ZapUpdateClient`，订阅全部事件

**验证点**：
- `RequestDownloadUpdate` 触发（通知调用者确认下载）
- `RequestApplyUpdate` 触发（通知调用者确认应用）
- `StatusChanged` 触发链：`None → DiscoveredUpdate → DownloadingUpdate → DownloadedUpdate → ApplyUpdate`
- 下载完成后 `version.json` 更新为 `downloaded`
- `ErrorStatusChanged` 不触发
- `IsRunning` 在 apply 后变为 `False`

### 4.3 强制更新（ForceUpdate = true）

**步骤**：
1. Push V0.2.0（带 `--set-force-update`）作为最新版本
2. local version.json 设为 V0.1.0
3. 启动 `ZapUpdateClient`

**验证点**：
- `RequestDownloadUpdate` **不触发**（强制更新跳过确认）
- `RequestApplyUpdate` **不触发**
- `StatusChanged` 全程自动：`DiscoveredUpdate → DownloadingUpdate → DownloadedUpdate → ApplyUpdate`
- 全流程无需外部交互

### 4.4 带脚本更新

**步骤**：
1. Push V0.3.0（带 `--after-apply-update-script post_update.bat`）
2. local version.json 设为 V0.2.0

**验证点**：
- 下载完成后 `version.json` 中的 `after_apply_update_script` 字段正确
- `apply_update` 后脚本被执行（通过 `update.log` 验证）

### 4.5 ErrorStatusChanged 测试

**步骤**：
1. 关闭 server（`taskkill /F /IM zap-server.exe`）
2. 静置 2s 确保端口释放
3. 启动 `ZapUpdateClient`

**验证点**：
- `ErrorStatusChanged` 触发，传递错误消息（首次错误）
- 后续同错误被抑制（去重），不触发新事件
- 重启 server 后 `check_update` 成功 → `ErrorStatusChanged` 触发 `null`（错误恢复）
- `StatusChanged` 不因错误触发
- `IsError` 属性正确反映错误状态

---

## 五、完整测试脚本结构

```python
import os, subprocess, shutil, json, time

d = os.path.join(os.environ['TEMP'], 'zap-test-' + time.strftime('%H%M%S'))
# 1. 创建目录结构
# 2. 编译 server / CLI / client
# 3. 启动 server
# 4. 创建项目 + 推送测试版本
# 5. 配置 client.json / shared.json / version.json
# 6. 编译 C# 测试程序
# 7. 逐场景运行，收集输出
# 8. 断言验证
# 9. 清理

# 验证模式：解析 stdout 中的结构化标记
# STATUS:{status}:{message}
# ERROR:{message}
# DOWNLOAD_REQ:{version}
# APPLY_REQ:{version}
# STATUS_COUNT:{n}
# ERROR_COUNT:{n}
# FINAL_STATUS:{status}
# IS_ERROR:{true|false}
# DOWNLOAD_REQ_COUNT:{n}
# APPLY_REQ_COUNT:{n}
```
