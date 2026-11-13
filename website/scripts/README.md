# Screenshot capture / 截图脚本

`capture_screenshots.py` 用真实程序与演示数据生成文档站点需要的界面截图，不参与 VitePress 构建。

## 前置条件 / Prerequisites

- Windows 交互式桌面（脚本会移动窗口并截屏）。
- .NET 8 运行时（启动 `AlyPublish.exe`）。
- Python 3.9+ 与 Pillow。
- 仓库内已有构建产物：
  - `dist-test/server.exe`、`dist-test/aly-publish.exe`、`dist-test/aly-client.exe`、`dist-test/fake_main.exe`
  - `publish/publish-gui-csharp/src/AlyPublish/bin/Debug/net8.0/AlyPublish.exe`（及同目录 `aly-publish.exe`）
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

发布 GUI 的文案在 `publish/publish-gui-csharp/src/AlyPublish/Resources/Strings.json` 与
`Strings.zh-Hans.json`。新增语言时复制一份 `Strings.<culture>.json`，保持键集合完全一致，
并在 `LanguageManager` 的语言解析处加入该 culture；随后用 `--lang` 重新截图。