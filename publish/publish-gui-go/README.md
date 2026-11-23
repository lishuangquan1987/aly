# publish-gui-go

`AlyPublish` 发布 GUI 的 Go / MyGo native UI 实现，用于替换 C# / Avalonia 版
`publish-gui-csharp`。两者共用同一套发布包布局（`payload.json`、Inno Setup 安装器、
Linux `install.sh`）与同一组 release 资产名，配置格式也完全一致，因此切换实现
对使用者透明。

## 发布与替换

Release Build 工作流提供 `gui` 输入，最终生效值由顶层 `env.PUBLISH_GUI` 统一决定：

| 取值 | 行为 |
|---|---|
| `auto`（默认） | 跟随仓库变量 `PUBLISH_GUI`；未设置时为 `csharp` |
| `csharp` | 强制发布 Avalonia 版，现有流程不变 |
| `go` | 强制用本目录构建 GUI 本体，产出**同名**的 4 个资产 |

优先级：手动显式选 `go` / `csharp` > 仓库变量 `PUBLISH_GUI` > `csharp`。
`release` 事件没有 inputs，会落到仓库变量。

手动触发：仓库 Actions → Release Build → Run workflow，`gui` 选 `go`；正式发版时
同时填 `version=vX.Y.Z` 并勾选 `upload_to_release`。「一键全部构建」也有同名
`gui` 输入，`auto` 默认跟随仓库变量。产出
`AlyPublish-Setup.exe`、`AlyPublish-win-x64.zip`、`AlyPublish-linux-x64.zip`、
`AlyPublish-linux-arm64.zip` —— 与 C# 版同名，网站下载链接无需改动。两个打包 job
的 payload 铺装、安装器、zip 逻辑完全复用，只是 GUI 二进制的产出步骤二选一。

要把 Go 版设为**默认**实现（含 `release` 事件自动发布），在仓库
Settings → Secrets and variables → Actions → Variables 新增仓库变量
`PUBLISH_GUI=go` 即可，无需改代码；临时验证用手动触发选 `gui=go`。
注意 `workflow_dispatch` 的 `default` 只影响手动触发，且只有 `auto` 会读取仓库变量
—— 把 `default` 改成 `go` 并不会让 `release` 事件生效。

替换前提（均已落实）：

- **配置兼容**：`config.json`、`settings.json`、`.updator/shared.json` 的路径与字段
  与 C# 版一致，可无缝接管已有项目书签；主题/主题色单独存 `appearance.json`，
  不会被 C# 版切换语言时覆盖。
- **payload 驱动**：读包内 `payload.json` 的 `cli_rid` / `client_rid` /
  `payload_rids` 定位发布 CLI 与更新器，与 C# `PkHelper` 行为一致；开发树没有
  manifest 时回退到本机 rid 候选。
- **包布局**：GUI 本体、安装器、启动器同名同位置，`install.sh` 补齐可执行位。
- **功能对等**：项目书签、添加本地/服务端项目、CLI `config init`、
  `UpdateFolder/client.json`、暂存/取消暂存、发布（流式进度/取消/强制更新）、
  编辑项目与服务端 ignore 同步、版本历史，以及 F5 / Ctrl+Enter / Ctrl+E 快捷键。
- **额外能力**：深浅色主题与主题色、每个项目独立的表单/选择状态、添加服务端项目
  时的高级忽略规则。

## 运行依赖

- **Linux**：需要 GTK3（`libgtk-3.so.0`；MyGo 用 purego dlopen 加载）。Go 包带
  `aly-go-gui.marker` 标记（不以点开头，确保打 zip 时不会被排除），`install.sh`
  在缺库时给出安装提示，只提示不阻断；C# 包不带该标记，不会误报。
- **Windows**：D3D11（WARP 回退）；构建带 `-ldflags "-H windowsgui"` 去掉黑色控制台。
- **macOS**：与 C# 版一致 —— 两者目前都只有 Windows / Linux 发布包。

纯 Go、无 cgo、native UI（`ui` 包），不使用 WebView/HTML/JS。配置/日志路径与 C# 版
一致；测试可用 `ALYPUBLISH_CONFIG_DIR` 覆盖，避免污染真实书签。

## 构建

需要 Go 1.27.3（MyGo 0.8.3 要求），并确保 `GOROOT` 指向该工具链。Windows 上如果全局
`GOROOT` 指向 Go 1.10（XP client 的开发环境），先覆盖：

```powershell
$env:GOROOT = 'D:\Install\go1.27.3'   # 本机路径，按实际安装调整
$env:GO111MODULE = 'on'
$env:GOARCH = 'amd64'                  # 全局 GOARCH 可能是 386（XP client），必须覆盖
$env:GOOS = 'windows'
go test ./...
go build -trimpath -ldflags "-H windowsgui" -o build\windows-amd64\AlyPublish.exe .
```

或直接运行 `build.ps1` / `build.sh`（显式设置 GOOS/GOARCH，构建 windows-amd64、
linux-amd64、linux-arm64 三个目标，并执行 fmt/vet/test）。

运行时需要 `aly-publish`：同目录、`publish-cli/<rid>/`、仓库开发路径或
`ALY_PUBLISH` 环境变量；发布包内由 `payload.json` 指定具体 rid。

## 测试

`go test ./...` 包含 `ui.NewTester` 无窗口视图测试（浅色/深色、最小窗口、编辑框
chips、添加服务端项目弹窗、主题色）、`payload.json` 驱动与 manifest 字段一致性测试、
快捷键、`UpdateFolder`、设置隔离与配置解析测试。
