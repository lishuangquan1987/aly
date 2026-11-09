# aly-client-ui

`aly-client` 的图形前端集合，与更新器分离维护：

| 平台 | 目录 | 产物 | 工具链 | 说明 |
|------|------|------|--------|------|
| Windows XP+ | `aly-client-winui/` | `aly-client-ui.exe` | Go 1.10.8 + GOARCH=386 + walk | 屏幕右下角 topmost，XP 兼容；见 `aly-client-winui/BUILD.md` |
| Linux amd64/arm64 | `aly-client-linuxui/` | `aly-client-ui` | Go 1.27.3 + MyGo 0.8.3 原生 UI | 依赖 GTK3；见 `aly-client-linuxui/README.md` |

## 部署布局（两平台一致）

```text
<主程序目录>/                       # 随版本包分发，自更新的比对源
  aly-client.exe / aly-client
  aly-client-ui.exe / aly-client-ui
<UpdateFolder>/                     # 实际运行
  aly-client.exe / aly-client
  aly-client-ui.exe / aly-client-ui
  client.json
  version.json
```

宿主只需要启动 `<UpdateFolder>/aly-client-ui(.exe) -mode auto -lang zh-CN`；
UI 启动时先同步更新器（`aly-client self_update`），再按需替换 UI 自身。

## 构建

- Windows XP：见 `aly-client-winui/BUILD.md`（Go 1.10.8/386）。
- Linux：`./build-linux.sh`（或 `build-linux.ps1`）构建两架构的
  `aly-client` + `aly-client-ui`，输出到 `dist/linux-<arch>/`；Linux 侧用
  `./deploy-client-linux.sh <UpdateFolder> <MainFolder> [arch]` 部署两件套。
  Linux UI 需要 GTK3 运行库，不需要 WebKitGTK/Bun。

## 共同契约

- CLI、标准 JSON 输出、`client.json`/`version.json`、journal
  （`aly-client-ui.selfupdate.json`）与 `-finish-self-update` 语义在两平台一致。
- Windows UI 用 `aly-client.exe`；Linux UI 用 `aly-client`（无扩展名）。
- Linux UI 的构建/部署见子目录 README；Windows XP 的 CI/PE 校验见
  `.github/workflows/release.yml`。