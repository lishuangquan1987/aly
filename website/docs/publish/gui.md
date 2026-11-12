---
title: 发布 GUI（AlyPublish）
description: AlyPublish.exe 的运行条件、项目书签、初始化与本地项目、变更审阅、发布与版本历史、快捷键、语言切换与日志。
---

# 发布 GUI（AlyPublish）

`AlyPublish.exe` 是 `aly-publish` 的 Avalonia 桌面前端（.NET 8），面向不习惯命令行的发布人员。它不重新实现发布逻辑：所有操作都通过同目录的 `aly-publish.exe --json` 完成，因此 `.updator/` 配置、暂存区和版本规则与 CLI 完全一致。

<ScreenshotFrame src="/images/publish-gui-overview-zh.webp" alt="AlyPublish 中文主界面：左侧项目列表，中部未暂存与已暂存文件，右侧发布面板与版本历史" caption="AlyPublish 主界面（中文）" wide />

界面语言切换到 EN 后的截图见 [publish-gui-overview.webp](/images/publish-gui-overview.webp)。

## 运行条件

| 条件 | 说明 |
|---|---|
| 可执行文件 | `AlyPublish(.exe)` |
| 必需依赖 | 安装包内的 `publish-cli/<cli_rid>/aly-publish(.exe)`；`cli_rid` 由包内 `payload.json` 给出 |
| 运行时 | Release 安装包/压缩包为自包含 .NET 8；源码 `dotnet build` 为框架依赖 |
| 最小窗口 | 1080 × 640，默认 1320 × 840 |

获取安装包（点击文件名下载）：

<ReleaseDownloads group="package" compact :show-latest-hint="false" :show-version="false" />

包内 payload 与平台选择见[下载与安装](/docs/download)。CLI 的查找顺序：

1. `payload.json` 指定的 `publish-cli/<cli_rid>/aly-publish(.exe)`；
2. 包里其它平台目录（按 `payload_rids` 顺序兜底，命中非本包平台时会告警）；
3. `AlyPublish` 同目录的 `aly-publish(.exe)`（旧的手工放法仍然可用）；
4. 项目目录、以及开发树的 `publish/publish-cli/`（仅本地开发用）。

`payload.json` 缺失或损坏时回退平台探测（Windows 取 `windows-amd64`，Linux 取 `linux-amd64` / `linux-arm64`），日志里会记一行原因。

找不到时，窗口顶部出现“未找到 aly-publish”横幅，提供“重新检测”和“打开程序目录”两个按钮；找到后右上角显示绿色的“CLI 就绪”标记。所有依赖 CLI 的操作（刷新、暂存、发布、项目设置）在缺失时都会失败，项目初始化也无法完成。

GUI 调用 CLI 时的工作目录是**项目目录**，参数固定包含 `--json`。各操作的超时：状态刷新 120 秒，普通操作 60 秒，发布 300 秒。

## 项目书签

左侧列表是**书签**，只保存显示名、项目路径和主程序路径，不保存服务端地址与忽略规则——后者在项目目录的 `.updator/shared.json` 里。

Windows 上的书签文件：

```text
%LOCALAPPDATA%\AlyPublish\config.json
```

内容示例：

```json
[
  {
    "DisplayName": "演示应用",
    "ProjectPath": "C:\\Aly\\Demo\\demo-endlist",
    "MainExePath": "DemoApp.exe"
  }
]
```

- `DisplayName` 仅用于左侧列表；搜索框同时匹配显示名与路径。
- `MainExePath` 可以是相对项目目录的路径；位于项目目录外时只能存绝对路径。
- “移除”只删除书签并需要确认，不会删除项目文件或 `.updator/`。
- 选中项目后，右侧会读取 `status` 与最近 20 条版本日志；首次加载显示“正在读取项目状态…”。

## 初始化项目

“初始化项目”是一个两步向导，用于把**服务端已有项目**与一个尚未初始化的本地目录绑定。

### 第一步：选择服务端项目

- 服务端地址可从历史书签中出现过的地址里选择，也可以手填；
- 点击“获取项目”拉取列表，可按项目名或标题过滤；
- 选中后可进入下一步，显示该项目的当前版本（未发布时显示“未发布”）；
- 服务端还没有项目时，可直接“新建服务端项目”：填写标题与名称（名称默认跟随标题，手动改过之后不再同步）、强制更新、忽略规则。创建成功后自动回到列表并选中。

### 第二步：本地目录与规则

| 字段 | 说明 |
|---|---|
| 本地项目目录 | 构建产物所在目录；可浏览或直接把文件夹拖进窗口 |
| 显示名称 | 左侧列表中的别名，默认取服务端项目名 |
| 主程序 | 可选；用于“启动”按钮和 `UpdateFolder` |
| 更新时关闭的进程 | 默认取主程序文件名（不含扩展名） |
| 忽略文件夹/文件 | 发布扫描时跳过；可输入后回车，或从项目目录选择 |
| 不复制文件夹/文件 | 应用更新时跳过复制，保留目标机原文件 |

目录校验：

- 必须存在；
- 不能已经包含 `.updator/shared.json`（这种情况请用“添加本地项目”）；
- 不能已经存在于左侧书签中；
- 显示名称不能为空。

确认后，GUI 调用：

```bash
aly-publish config init --server <地址> --project <名称> \
  --ignore-folders <...> --ignore-files <...> \
  --un-copy-folders <...> --un-copy-files <...>
```

若填写了主程序，还会在项目目录的**同级**创建 `UpdateFolder/`：

- 更新器取自**安装包 payload** 的 `client/<client_rid>/`（平台由包内 `payload.json` 决定），无条件覆盖写入；
- 写入 `client.json`，包含主程序相对路径与需要关闭的进程名；
- 找不到更新器时只给警告，不阻断初始化；跨盘会导致主程序相对路径无法计算而失败；
- 要在别的平台交付更新器时，直接从包内其它 `client/<rid>/` 取（见[下载与安装](/docs/download)）。

初始化完成后，项目会加入左侧书签并被选中。它**不会**上传任何文件，也不会自动创建服务端项目。

::: warning 首次发布版本号
服务端创建项目时已占用 V1.0.0。初始化完成后第一次发布必须填 V1.0.1 或更高，否则会被防倒退校验拒绝。
:::

## 添加本地项目

“添加本地项目”用于接管一个**已经初始化**的目录：

- 目录中必须已有 `.updator/shared.json`；
- 读取后显示服务端地址、项目名、忽略规则条数；
- 显示名称默认取服务端项目名，否则取文件夹名；
- 只保存书签，不复制文件、不修改 `.updator/`，也不调用 CLI；
- 若项目同级**已有** `UpdateFolder/`，会顺手把里面两个更新器刷新成安装包 payload 的同名文件（不动 `client.json`）；没有 `UpdateFolder/` 时什么都不做。

同一个目录不能重复添加；已在书签中的目录会提示“该项目已在左侧列表中”。

## 项目设置

工具栏的“项目设置”或 Ctrl+E 打开设置对话框，同时编辑两类配置：

| 类别 | 字段 | 写入位置 |
|---|---|---|
| 本机书签 | 显示名称、主程序 | `%LOCALAPPDATA%\AlyPublish\config.json` |
| 项目配置 | 服务端地址、项目名、忽略/不复制规则 | `.updator/shared.json`（通过 CLI） |

行为：

- 保存前重新读取 `shared.json`，只把真正变化的项发给 CLI（`config set` / `config set-array`），避免用旧快照覆盖别处的修改。
- 只要 `shared.json` 有至少一项变化，就会把本地忽略规则同步到服务端项目，同时保留服务端的标题与强制更新标记；同步失败只警告，不阻断本地保存。
- 修改项目名不会重命名服务端项目，只会让客户端按新名称连接；必须确保服务端已有同名项目。
- 服务端地址或项目名为空时无法保存。
- 保存时若填了主程序、且项目同级还没有 `UpdateFolder/`，会顺手建出它（与初始化同一条代码路径：从安装包 payload 复制更新器 + 写 `client.json`）；已有 `UpdateFolder/` 则不动，避免覆盖用户配置。

## 审阅变更

右侧上半部分是变更区：

- “未暂存”表格列出 `status` 返回的新增与修改；**删除项不会显示**，GUI 不能手动暂存删除。删除由服务端在发布后固化的清单驱动客户端执行。
- “已暂存”表格列出暂存区条目；暂存条目的“服务端”列为空（`—`），因为加入暂存区时只记录了本地快照。
- 列：选择、文件、状态、本地、服务端。状态徽标区分新增 / 修改。
- 每行可“打开所在位置”；顶部按钮支持刷新、设置、打开项目目录、启动主程序。
- 未暂存区提供“全选/取消全选”“暂存所选”“全部暂存”；已暂存区提供“全选/取消全选”“取消暂存”“清空”（清空需确认）。
- 首次选中项目和手动刷新都会重新读取 `status` 与日志；底部状态栏显示“未暂存 N，已暂存 M”、项目路径和日志入口。

## 发布与版本历史

右侧发布面板要求三个条件同时满足：

1. 暂存区非空；
2. 版本号非空；
3. 变更说明非空。

| 控件 | 说明 |
|---|---|
| 版本号 | 例如 `V1.0.2`；不填时按钮禁用并提示“请填写版本号” |
| 下一版本 | 基于当前版本号把最后一段数字 +1（`V1.0.2` → `V1.0.3`）；当前版本缺失或格式无法识别时禁用 |
| 变更说明 | 必填；GUI 把它作为**一条**日志提交，多条日志请用 CLI 的重复 `--message` |
| 高级选项 | 更新后执行的脚本（相对于应用目录）、强制更新复选框 |
| 发布按钮 | 文案随暂存数量变化（“发布 3 个文件”）；未暂存时提示先暂存 |

发布过程：

- GUI 以 `--json` 调用 `push`，逐行解析 START / DONE / FAIL，展示文件名、大小、状态与总进度；
- 可取消；取消会终止 CLI 进程。已上传的文件不会回滚，暂存区也不会清空；
- 成功后清空版本号、变更说明、脚本和强制更新勾选，刷新状态与版本历史，并提示“发布成功：V1.0.2”；
- 失败时弹出错误信息，暂存区保留，可直接重试；若文件已上传但版本登记失败，需要用更高的版本号重试。

版本历史显示最近 20 条，服务端按最新在前返回：

- 版本历史中最新一条以高亮徽标显示；当前版本同时出现在“变更”标题与发布面板的版本徽标中；
- 服务端创建项目时写入的 V1.0.0“第一次创建”也会出现在历史里；
- 没有记录时显示“暂无发布记录”。

## 快捷键

| 快捷键 | 动作 |
|---|---|
| F5 | 刷新当前项目（等价于“刷新”按钮） |
| Ctrl+Enter | 发布当前项目（仅当发布按钮可用时触发） |
| Ctrl+E | 打开当前项目的设置 |

## 界面语言

右上角的语言按钮显示**目标语言**：中文界面显示 `EN`，英文界面显示 `中文`。

- 点击后立即切换，并把选择写入 `%LOCALAPPDATA%\AlyPublish\settings.json`：

  ```json
  {"language":"zh-Hans"}
  ```

- 启动时语言优先级：命令行 `--lang` > 已保存的 `settings.json` > 操作系统 UI 语言。
- `--lang` 的取值只要以 `zh` 开头就是中文，其余一律英文：

  ```bash
  AlyPublish.exe --lang zh
  AlyPublish.exe --lang en
  ```

- `--lang` 只影响本次启动，不会写入设置文件。

翻译资源使用 Irihi.Lingua 的 JSON 方案：

| 文件 | 作用 |
|---|---|
| `Resources/Strings.json` | 默认（英文）资源 |
| `Resources/Strings.zh-Hans.json` | 简体中文资源 |

`LanguageManager` 通过 `[LinguaManager("./Resources/Strings.json")]` 在编译期生成键类与可观察字符串；XAML 使用：

```xml
Text="{Translate {x:Static loc:LanguageManager+Keys.S009}}"
```

新增文案时，需要同时往两个 JSON 加同一个键，然后重新编译以生成 `Keys` 成员；运行时切换语言由 `UpdateCulture` 驱动，无需重启。

## 日志与排障

GUI 用 Serilog 写文件日志。文件位于 `logs/` 目录，按天滚动，文件名形如 `log20261110.log`（同一天多个进程时可能带 `_001` 之类的后缀）。日志目录按以下顺序选择，取第一个可写的：

1. `AlyPublish.exe` 所在目录的 `logs/`；
2. `%LOCALAPPDATA%\AlyPublish\logs\`；
3. 临时目录下的 `AlyPublish\logs\`。

状态栏的“日志”按钮直接打开该目录。

| 现象 | 可能原因与处理 |
|---|---|
| 顶部提示未找到 `aly-publish` | 安装包的 `publish-cli/<rid>/` 缺失或 `payload.json` 不匹配：重新安装 AlyPublish，再点“重新检测”；旧布局下把 CLI 放到 GUI 同目录也能被识别 |
| 初始化后 `UpdateFolder` 里没有更新器 | 安装包缺少 `client/<rid>/`（重新安装）；若主程序与项目目录跨盘，`main_exe_relative_path` 算不出来，会直接失败 |
| Linux 下从包里取的 CLI/更新器不能执行 | zip 不保留可执行位：`chmod 0755`；用 `install.sh` 安装时它已经代劳 |
| 目录已初始化，无法继续初始化 | 改用“添加本地项目” |
| 服务端项目列表为空 | 先“新建服务端项目”，或检查服务端地址 |
| 发布被拒绝：版本不大于最新版本 | 使用更高版本号；首次发布至少 V1.0.1 |
| 发布失败且提示文件已修改 | 刷新后重新暂存；CLI 会复核 add 时的 MD5 |
| 上传失败 | 查看进度面板的 FAIL 行与日志；暂存区未清空，修复后重试 |
| 界面语言不对 | 检查 `settings.json`；用 `--lang zh` / `--lang en` 覆盖 |

## 构建与分发

Release 工作流会构建并上传 **4 个 GUI 资产**（见 `.github/workflows/release.yml`）：

- `AlyPublish-Setup.exe`：Windows x64 安装包，自包含 .NET 8，内含 `publish-cli/`、`client/`、`payload.json`；
- `AlyPublish-win-x64.zip`：Windows 便携包，解压即用；
- `AlyPublish-linux-x64.zip`、`AlyPublish-linux-arm64.zip`：Linux 压缩包（安装包），包内另有 `install.sh` / `uninstall.sh`；解压后先 `chmod +x install.sh`（zip 不保留可执行位）再 `./install.sh` 装到 `/opt/aly/AlyPublish`（`--prefix` 可改；`install.sh` 会替 payload 里的可执行文件补齐 0755）。

发布 CLI 与终端更新器**不再作为独立资产上传**：它们只随 GUI 安装包分发（用户装一个包就够）。

payload 的铺装由 release 工作流里的内联 pwsh 步骤完成（`Stage the package payload`）：按工作流下载的各平台产物铺成上面那份布局，拷入对应平台的 `payload.json`，并断言 10 项（9 个二进制 + 清单）全部到位（缺任一即构建失败）。本地开发走 `publish/deploy-publish.bat`，它用同一套文件名直接 copy 到 GUI 输出目录。

从源码构建：

```bash
# 日常调试（框架依赖，需要本机 .NET 8）
dotnet build publish/publish-gui/src/AlyPublish/AlyPublish.csproj -c Release

# 生成自包含单文件（与 CI 便携包一致）
dotnet publish publish/publish-gui/src/AlyPublish/AlyPublish.csproj \
  -c Release -r win-x64 --self-contained true \
  -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true \
  -o publish/publish-gui/artifacts/app

# Linux 包（同一个工程，换 runtime；CI 在此之上再加 install.sh 并压 zip）
dotnet publish publish/publish-gui/src/AlyPublish/AlyPublish.csproj \
  -c Release -r linux-x64 --self-contained true \
  -p:IncludeNativeLibrariesForSelfExtract=true \
  -o publish/publish-gui/artifacts/app
```

手工分发时，把 GUI 本体与 `payload.json`、`publish-cli/`、`client/` 放在同一目录即可（初始化向导就是按这个布局取更新器的）。

## 下一步

- CLI 命令细节：[配置命令](/docs/publish/commands-config)、[项目命令](/docs/publish/commands-project)、[发布流程命令](/docs/publish/commands-workflow)
- 发布工具总览：[aly-publish](/docs/publish)
- 获取 CLI 与服务端：[下载与安装](/docs/download)