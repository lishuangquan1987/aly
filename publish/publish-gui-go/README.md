# publish-gui-go（实验性）

C#/Avalonia 版 `publish-gui-csharp` 的 MyGo native UI 复刻实验。**不作为发布版本，也不进入
release 资产或公开文档**；稳定后再同步文档。

## 范围

Phase 0B 只验证纵向切片：项目书签、未暂存/已暂存文件、暂存/取消暂存、发布（流式进度/取消）、
编辑项目（忽略/不复制规则 chips、主程序选择、CLI 配置保存）。完整功能对等、打包、文档同步
留到后续阶段。

- 纯 Go、无 cgo、native UI（`ui` 包），不使用 WebView/HTML/JS。
- Linux 运行时只依赖 GTK3；Windows 用 D3D11（WARP 回退）；macOS 未纳入本阶段。
- 配置/日志路径与 C# 版一致，Phase 0B 可用 `ALYPUBLISH_CONFIG_DIR` 覆盖，避免污染真实书签。

## 构建

需要 Go 1.27.3（MyGo 0.8.3 要求），并确保 `GOROOT` 指向该工具链。Windows 上如果全局
`GOROOT` 指向 Go 1.10（XP client 的开发环境），先覆盖：

```powershell
$env:GOROOT = 'D:\Install\go1.27.3'   # 本机路径，按实际安装调整
$env:GO111MODULE = 'on'
$env:GOARCH = 'amd64'                  # 全局 GOARCH 可能是 386（XP client），必须覆盖
$env:GOOS = 'windows'
go test ./...
go build -o build\windows-amd64\AlyPublish.exe .
```

或直接运行 `build.ps1`（会显式设置 GOOS/GOARCH 并构建三平台）。

Linux 交叉编译：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/AlyPublish .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o build/linux-arm64/AlyPublish .
```

`build.ps1` / `build.sh` 封装了上述命令（不修改全局环境）。运行时需要 `aly-publish`
（同目录、`publish-cli/<rid>/`、仓库开发路径或 `ALY_PUBLISH` 环境变量）。

## 已知限制（Phase 0B）

- 只实现 add-local 项目；服务端项目创建/选择向导、UpdateFolder 生成、服务端 ignore 同步留到
  Phase 1。
- 编辑项目保存本地书签与 `.updator/shared.json`（通过 CLI `config set*`），不主动改服务端项目信息。
- 支持跟随系统/浅色/深色三态主题，选择写入 `settings.json`；视觉不追求 Semi/Ursa 像素级一致。
- `go test ./...` 包含 `ui.NewTester` 无窗口视图测试（浅色/深色、最小窗口、编辑框 chips），
  不依赖真实窗口或 CLI。