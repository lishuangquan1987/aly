# aly-client 命令速查

更新器全部命令的统一输出格式：

```json
{ "isSuccess": true, "errorMsg": "", "data": { ... } }
```

配置加载优先级：`client.json` → `.updator/shared.json` → CLI 参数覆盖。
完整参数说明见仓库 [COMMANDS.md](https://github.com/lishuangquan1987/aly/blob/master/COMMANDS.md)。

## 命令总览

| 命令 | 说明 | 典型调用方 |
|---|---|---|
| `check_update` | 检查是否有新版本 | 宿主应用 / GUI / SDK |
| `check_diff` | 比对本地与服务端文件差异（MD5+SHA256） | 调试 / 高级集成 |
| `download_update` | 下载变更文件（字节级进度 + 物化未变化文件） | 宿主应用 / SDK |
| `apply_update` | 应用已下载的更新（原子替换 + 重启主程序） | 宿主应用 / SDK |
| `list_rollback_versions` | 列出可回滚的历史版本快照 | GUI / SDK |
| `rollback --version X` | 回滚到指定版本快照 | GUI / SDK / 运维 |
| `clean [--keep N] [--all]` | 清理历史快照与临时文件 | 定期任务 |
| `check_self_update` | 检查更新器自身是否需要更新 | SDK / GUI |
| `self_update` | 用主程序目录的新版更新器替换自身 | SDK / GUI 委派 |

## 核心命令示例

### check_update

```bash
aly-client.exe check_update [--url <服务器地址>] [--project-name <项目名>]
```

```json
{ "isSuccess": true, "data": { "has_update": true, "need_download_update": true,
  "new_version": "1.0.1", "force_update": false } }
```

### download_update

```bash
aly-client.exe download_update
```

- 只下载与本地不同的文件（服务端清单哈希比对）
- 逐文件 MD5 + SHA256 双校验，断点续传（`.part` 临时文件）
- 单文件 3 次重试后不中断整体，失败清单进 `failed_files`
- 下载完成后自动物化未变化文件到新版本目录

### apply_update

```bash
aly-client.exe apply_update
```

流程：校验已下载包完整性 → 关闭 `must_close_process_name` 中的进程 → 三次目录改名（备份 → 激活）→ 重启主程序。任意时刻崩溃，下次启动自动恢复，详见[核心概念](/guide/concepts#崩溃恢复)。

### rollback

```bash
aly-client.exe list_rollback_versions
aly-client.exe rollback --version V1.0.0
```

回滚使用本地版本快照（`ApplicationFolder_V1.0.0/`），不需要网络。应用未完成时拒绝执行；失败时恢复执行前状态。

### self_update

```bash
aly-client.exe self_update
```

用主程序目录（`ApplicationFolder`）中随版本分发的新版更新器，替换 `UpdateFolder` 中正在运行的自身副本：SHA256 比对决定是否需要更新、`.new` 中转 + 校验 + 原子落位、运行中 exe 采用改名舞步（`.old` 备份 + 失败回滚）。与 `check_self_update` 的检测输出配合，构成自更新闭环——详见[自更新设计](/advanced/self-update)。

## aly-client-ui 的启动模式

```bash
# 常规启动（配合 aly-client 完成更新，右下角置顶窗口）
aly-client-ui.exe

# 检查模式（不弹窗，只输出检查结果 JSON）
aly-client-ui.exe -mode check

# Linux 自更新收尾进程（UI 自身替换的守护模式，由启动器调用）
aly-client-ui -finish-self-update --target <exe路径> --pid <旧PID> --created <starttime> --new <.new文件> -- <重启命令...>
```

收尾进程会校验目标进程身份（`/proc/<pid>/stat` 的 starttime 防 PID 复用）、等待其退出（最长 10 秒后 SIGKILL）、原子落位 `.new` 文件并重启 UI——不依赖 GTK，可在无图形环境运行。
