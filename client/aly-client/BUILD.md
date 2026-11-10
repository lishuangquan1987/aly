# aly-client 编译说明

本目录只产出 **`aly-client.exe`**（更新器本体，控制台程序）。

图形前端已迁出到 `client/aly-client-ui/`：

| 平台 | 源码 | 产物 | 文档 |
|------|------|------|------|
| Windows XP+ | `client/aly-client-ui/aly-client-winui/` | `aly-client-ui.exe` | `aly-client-winui/BUILD.md` |
| Linux | `client/aly-client-ui/aly-client-linuxui/` | `aly-client-ui` | `aly-client-linuxui/README.md` |

## 环境要求

- **Go 1.10.8**（Windows XP 硬性要求；Go 1.11+ 产出的二进制 XP 无法加载，
  因为 PE `MajorSubsystemVersion` 会变成 5.1 以上。`build.bat` 有版本断言）。
- 项目位于 GOPATH 下：
  - `client/aly-client/`     → `%GOPATH%/src/aly/client/aly-client/`
  - `client/aly-client-ui/`  → `%GOPATH%/src/aly/client/aly-client-ui/`（构建 winui 时需要）
  - `client/aly-client-sdk/` → `%GOPATH%/src/aly/client/aly-client-sdk/`

## 完整编译流程

### 1. 复制源码到 GOPATH

```batch
xcopy /e /i /y client\aly-client %GOPATH%\src\aly\client\aly-client
xcopy /e /i /y client\aly-client-ui\aly-client-winui %GOPATH%\src\aly\client\aly-client-ui\aly-client-winui
xcopy /e /i /y client\aly-client-sdk %GOPATH%\src\aly\client\aly-client-sdk
```

### 2. 编译更新器（32 位，兼容 Windows XP）

```batch
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client
```

> 必须使用 import path `aly/client/aly-client`，不能使用绝对路径；否则 Go 1.10
> 在 GOPATH 模式下会找不到包。

### 3. 将 exe 复制回工作目录

```batch
copy /y aly-client.exe client\aly-client\aly-client.exe
del aly-client.exe
```

> `deploy/deploy-client.go` 会自动完成更新器与 winui 的构建/回拷；手工构建时才需要执行。

### 4. Linux 更新器（无 `.exe`）

Linux 更新器与 Windows 共用同一份源码，但平台文件名/进程能力不同
（`config.UpdaterFileName()` → `aly-client`；`util/process_unix.go` 用 `/proc`
实现查找/终止/等待）：

```sh
mkdir -p /tmp/aly-gopath/src/aly/client
cp -a client/aly-client /tmp/aly-gopath/src/aly/client/aly-client
cd /tmp/aly-gopath/src/aly/client/aly-client
GOPATH=/tmp/aly-gopath GO111MODULE=off GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -o aly-client aly/client/aly-client
```

Go 1.27.3 交叉编译；产物无扩展名。完整两件套（更新器 + MyGo UI）用
`client/aly-client-ui/build-linux.sh`（Windows 用 `build-linux.ps1`）。

> Windows XP 仍必须用 Go 1.10.8/386 构建；不要用 Go 1.27 产物替代。

## 一键编译脚本

`build.bat` 只构建更新器（内置 Go 1.10.x 版本断言）。完整部署用仓库根的
`deploy/deploy-client.go`，它会同时构建 `aly-client.exe` 与 `aly-client-ui.exe`。

## 常见问题

### Q: 为什么不能用 `go build` 在项目根目录直接编译？

源码中的 import 路径是 `aly/client/aly-client/...`，需要在 GOPATH 模式下通过
`$GOPATH/src/aly/` 解析。直接在项目根目录用相对路径编译会找不到内部包。

### Q: 为什么 exe 时间戳没有更新？

检查是否用了绝对路径作为 package 参数。Go 1.10 GOPATH 模式下，绝对路径会被
拼接到 `$GOROOT/src/` 和 `$GOPATH/src/` 后面，导致找不到包或编译旧文件。
应始终使用 import path `aly/client/aly-client`。

### Q: 怎么验证产物真的能跑在 XP 上？

`aly-client.exe` 的硬指标：PE 头 `Machine=0x014C`（32 位）且
`MajorSubsystemVersion <= 5`。Go 1.11+ 构建出来是 5.1/6.1，XP 加载器直接拒绝。
CI 的 “Verify XP compatibility” 步骤会检查。

### Q: 编译出的 exe 体积多大？

- `aly-client.exe`：约 3.8 MB（32 位，已 strip 调试符号）。
- winui 的 `aly-client-ui.exe`：约 4.6 MB（含 walk / lxn/win，已 strip），见
  `aly-client-winui/BUILD.md`。