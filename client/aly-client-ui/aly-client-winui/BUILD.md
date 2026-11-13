# aly-client-winui 编译说明

Windows 图形前端（walk，右下角 topmost，**兼容 Windows XP**）。产物
`aly-client-ui.exe`，必须与 `aly-client.exe` 同目录部署（UpdateFolder）。

- 源码：`client/aly-client-ui/aly-client-winui/`
- GOPATH：`%GOPATH%/src/aly/client/aly-client-ui/aly-client-winui/`
- 依赖：Go 1.10.8、`GOARCH=386`、仓库内 vendor、`rsrc.syso`（ComCtl32 v6 manifest）

## 构建

```batch
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client-ui/aly-client-winui
```

- `-H=windowsgui` 不能省，否则会附带黑色控制台窗口。
- **不要删除 `rsrc.syso`**：`go build` 会自动链接同包目录下的 `*.syso`；缺失时
  UI 启动失败（`create window failed: TTM_ADDTOOL failed`，exit 1）。
- 构建前需把 `client/aly-client` 也复制到 GOPATH（winui 依赖
  `aly/client/aly-client/selfupdate` 等包）。

## 回拷

```batch
copy /y aly-client-ui.exe client\aly-client-ui\aly-client-winui\aly-client-ui.exe
del aly-client-ui.exe
```

`deploy-client.go` 会自动完成构建/回拷/部署（用 Docker 镜像 `golang:1.10.8-stretch`
在 WSL 内编译并校验 PE 头）；手工构建时才需要执行。产物路径已加入 `.gitignore`。

## 一键脚本

`build-ui.bat` 内置 Go 1.10.x 断言；用法见脚本头部注释（先 cd 到包目录）。

## 常见问题

### Q: 启动后立刻退出，退出码 1？

说明创建窗口失败。最常见原因是 **manifest 没被链接**：确认
`client/aly-client-ui/aly-client-winui/rsrc.syso` 存在，且从包目录构建
（`go build aly/client/aly-client-ui/aly-client-winui`）。用控制台方式构建
（去掉 `-H=windowsgui`）可直接看到错误：

```batch
go build -ldflags="-s -w" -o ui-console.exe aly/client/aly-client-ui/aly-client-winui
ui-console.exe -mode check
:: 正常应能看到 create window failed: TTM_ADDTOOL failed 之类的具体原因
```

### Q: 怎么验证产物真的能跑在 XP 上？

两个硬指标（CI 的 “Verify XP compatibility” 步骤会检查）：

1. PE 头 `Machine=0x014C`（32 位）且 `MajorSubsystemVersion <= 5`；
2. `aly-client-ui.exe` 内含 `Microsoft.Windows.Common-Controls` 字符串。

### Q: 体积多大？

约 4.6 MB（含 walk / lxn/win 及其依赖，已 strip）。