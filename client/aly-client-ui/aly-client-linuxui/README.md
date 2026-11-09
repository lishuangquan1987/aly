# aly-client-linuxui

Linux 图形前端（MyGo 0.8.3 **原生 UI**，Go 1.27.3）。与 Windows 的
`aly-client-winui` 功能对齐：`auto/check/update/rollback/silent`、下载进度/
暂停继续、错误分阶段重试、强制更新倒计时、中英 i18n、单实例转发、
UI 自身替换重启、启动时更新器自更新。

## 运行环境

- Linux amd64/arm64（本模块可交叉编译）。
- GTK3 运行库（原生 UI 只需 GTK3；不需要 WebKitGTK、Bun/Node）。
- 部署布局与 Windows 一致：`aly-client-ui` 与 `aly-client`（无扩展名）同在
  `UpdateFolder`，主程序目录有同名两件套 + `client.json`/`version.json`。

宿主启动：

```sh
<UpdateFolder>/aly-client-ui -mode auto -lang zh-CN
```

## 构建

需要 Go 1.27.3（MyGo 要求）；项目通过 `go.mod` 的 tool 固定 MyGo v0.8.3：

```sh
go tool mygo vet
go test ./...
go tool mygo build -platform linux/amd64,linux/arm64
```

产物在 `build/linux-<arch>/aly-client-ui`。完整两件套构建用
`client/aly-client-ui/build-linux.sh`（或 `.ps1`）。

Release 附件名为 `aly-client-linux-amd64` / `aly-client-ui-linux-amd64`（arm64
同理）；部署到 UpdateFolder 时改名为无后缀的 `aly-client` / `aly-client-ui`。

### 交叉编译

MyGo 原生 UI 纯 Go、无 cgo，可从 Windows/macOS 交叉编译 Linux；在
Windows 上执行 `build-linux.ps1` 即可，Linux 侧只需 GTK3 运行库。

## 自更新

- 更新器：`updateUpdater` 调 `<UpdateFolder>/aly-client self_update`，源为
  `<MainFolder>/aly-client`；命令不可用/旧版时直接复制兜底，锁占用不兜底。
- UI 自身：`updateSelfUI` 比对 `<MainFolder>/aly-client-ui` 与运行副本；
  有差异时复制 `.new`，派生 `.new` 的隐藏模式：

  ```text
  aly-client-ui.new -finish-self-update --target <self> --pid <oldpid> \
    --created <proc starttime> --new <.new> -- <原启动参数>
  ```

  收尾进程（不启动 MyGo/GTK）等待旧 PID 退出（最多 10s，超时 SIGKILL，
  再等 2s；`/proc/<pid>/stat` starttime 防 PID 复用），旧 exe 改名 `.old`、
  `.new` 落位、拉起新 UI；日志在 `<UpdateFolder>/aly-client-ui.update.log`。
- journal 与 winui 同 schema：`<UpdateFolder>/aly-client-ui.selfupdate.json`，
  30 分钟内连续 3 次未收敛跳过，新 UI same 后清零。

## 测试

- `go test ./...`：client JSON 解析、flow 决策、i18n、journal、路径/复制、
  UI 自更新派生、收尾参数/改名/进程已退出；`ui.NewTester` 视图测试。
- Linux 专属收尾/进程测试带 `linux` 构建 tag；Windows 上只跑跨平台部分。
- CI 在 Ubuntu 上跑 `go test`、`go tool mygo vet`、两架构 `mygo build`；
  本机 WSL 无 Go/GTK 开发包，GUI 启动验证放 CI 或带 GTK3 的 Linux。

## 与 aly-client-winui 的关系

两者共享 CLI、标准 JSON、journal、`-finish-self-update` 语义，不共享 Go
源码：Windows UI 保持 Go 1.10.8/386/walk；Linux UI 是独立 Go module，避免
Go 1.10 GOPATH 与 Go 1.27 module 混用。