# Screenshot capture / 截图脚本

`capture_screenshots.py` 用真实程序与演示数据生成文档站点需要的界面截图，不参与 VitePress 构建。

## 前置条件 / Prerequisites

- Windows 交互式桌面（脚本会移动窗口并截屏）。
- Python 3.9+ 与 Pillow。
- 仓库内已有构建产物：
  - `dist-test/server.exe`、`dist-test/aly-publish.exe`、`dist-test/aly-client.exe`、`dist-test/fake_main.exe`
  - `publish/publish-gui-go/build/windows-amd64/AlyPublish.exe`（Go 主实现；脚本通过 `ALY_PUBLISH` 指向 `dist-test/aly-publish.exe`）
  - `client/aly-client-ui/aly-client-winui/aly-client-ui.exe`

## 用法 / Usage

```powershell
python website/scripts/capture_screenshots.py
python website/scripts/capture_screenshots.py --keep
```

默认在 `C:\AlyDemo` 创建临时演示包，结束后删除；`--keep` 保留用于排查。可用环境变量
`ALY_SCREENSHOT_TMP` 指定其他临时目录。

输出到 `website/public/images/`：

- `publish-gui-overview-zh.webp` / `publish-gui-overview.webp`（中/英发布 GUI）
- `client-ui-update-zh.webp` / `client-ui-update-en.webp`（中/英更新窗口）

## 行为 / Behaviour

- 启动真实的 `aly-server`（端口 29100），创建中英两个演示项目并推送版本；不修改仓库内数据目录。
- 发布 GUI 启动前会**临时替换** `%LOCALAPPDATA%\AlyPublish\config.json`，结束时恢复原文件；如果
  `AlyPublish.exe` 已在运行会直接报错退出，避免覆盖用户配置。
- 英文截图通过 `AlyPublish.exe --lang en-US` 启动；中文截图通过 `--lang zh-CN`。
- 英文演示项目的 V1.0.0 变更日志会在临时 SQLite 中改写为 `Initial version`，仅为截图演示数据。
- 截图会裁切窗口、缩放并保存为 WebP；请人工检查是否包含隐私路径或提示框。

## 新增语言后 / After adding a language

发布 GUI 的文案内建在 `publish/publish-gui-go` 的 `i18n.go`、`i18n_addproject.go`、
`i18n_detail.go`、`i18n_edit.go` 键表里（zh/en 两套，键集一致性由测试保证）。新增语言时
在 `main.go` 的 `--lang` 解析与 `newApp` 的语言判定处加入该语言，并补齐键表；随后用
`--lang` 重新截图。

## 文档防漂移 / Docs drift check

```powershell
python website/scripts/check_docs.py
```

比对两个二进制的自述清单与网站文档：`aly-publish docs`（全部命令与参数）与
`server -print-routes`（全部路由）。缺一条即退出码 1；确属不写入文档的项加到同目录
`docs-whitelist.txt`。网站 CI 在部署前自动执行本检查（需要 Go 工具链）。

## 文档结构检查 / Docs structure check

```powershell
python website/scripts/check_docs_parity.py
```

两件事，任一失败即退出码 1：

- **中英页面对齐**：`website/docs/**/*.md` 与 `website/en/docs/**/*.md` 的相对路径集合必须完全一致，
  每个页面必须有 frontmatter `title`；
- **起步页零原理**：`docs/start/*.md` 与 `en/docs/start/*.md` 不得出现原理类词汇
  （状态机 / 原子替换 / 架构、state machine / atomic / architecture），且必须带 `✅` 完成态与至少一个代码块。

CI 在 `check_docs.py` 之前执行本检查（纯 Python，无额外依赖）。
