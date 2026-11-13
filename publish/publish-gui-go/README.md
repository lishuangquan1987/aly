# publish-gui-go（实验性）

C#/Avalonia 版 `publish-gui-csharp` 的 MyGo native UI 复刻实验。**不作为发布版本，也不进入
release 资产或公开文档**；稳定后再同步文档。

## 范围

Phase 1 已覆盖项目书签、添加本地/服务端项目、CLI config init、UpdateFolder/client.json、
未暂存/已暂存文件、暂存/取消暂存、发布（流式进度/取消/force update）、编辑项目（服务器地址/
项目名/四组规则）、服务端 ignore 同步、每个项目独立表单状态、主题与主题色。打包与公开文档
同步仍留到稳定后。

- 纯 Go、无 cgo、native UI（`ui` 包），不使用 WebView/HTML/JS。
- Linux 运行时只依赖 GTK3；Windows 用 D3D11（WARP 回退）；macOS 未纳入本阶段。
- 配置/日志路径与 C# 版一致，Phase 1 可用 `ALYPUBLISH_CONFIG_DIR` 覆盖，避免污染真实书签。

## 构建

需要 Go 1.27.3（MyGo 0.8.3 要求），并确保 `GOROOT` 指向该工具链。Windows 上如果全局
`GOROOT` 指向 Go 1.10（XP client 的开发环境），先覆盖：

```powershell
$env:GOROOT = 'D:\Install\go1.27.3'   # 本机路径，按实际安装调整
$env:GO111MODULE = 'on'
$env:GOARCH = 'amd64'                  # 全局 GOARCH 可能是 386（XP client），必须覆盖
$env:GOOS = 'windows'
go test ./...
# -H windowsgui 很重要：否则双击/启动时会带一个黑色控制台窗口
go build -trimpath -ldflags "-H windowsgui" -o build\windows-amd64\AlyPublish.exe .
```

或直接运行 `build.ps1`（会显式设置 GOOS/GOARCH 并构建三平台）。

Linux 交叉编译：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/AlyPublish .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o build/linux-arm64/AlyPublish .
```

`build.ps1` / `build.sh` 封装了上述命令（不修改全局环境）。运行时需要 `aly-publish`
（同目录、`publish-cli/<rid>/`、仓库开发路径或 `ALY_PUBLISH` 环境变量）。

## 功能与限制（Phase 1）

- 支持“添加本地项目”和“添加服务端项目”：获取/创建服务端项目、选择尚未初始化的本地目录、
  执行 CLI `config init`、生成同级 `UpdateFolder/client.json` 并复制更新器。
- 编辑项目可修改显示名、主程序、`server.url`、`project.name` 和四组忽略规则；本地写入成功后
  会把 ignore folders/files 同步回服务端项目（失败只提示，不回滚本地）。
- 每个项目在切换时保留版本号、发布说明、文件选择和发布表单状态，不再因切项目丢失。
- 主题与主题色独立存 `appearance.json`；`settings.json` 只放 `language`，避免 C# 版切换语言时
  覆盖 Go 的主题设置。
- Windows 构建必须带 `-ldflags "-H windowsgui"`，否则会带一个黑色控制台窗口；`build.ps1` 已内置。
- 视觉不追求 Semi/Ursa 像素级一致；服务端同步和 UpdateFolder 的失败以 toast/状态栏提示。
- `go test ./...` 包含 `ui.NewTester` 无窗口视图测试（浅色/深色、最小窗口、编辑框 chips、
  添加服务端项目弹窗、主题色）以及 `UpdateFolder`、设置隔离测试。