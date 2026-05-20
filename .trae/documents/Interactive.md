# Interactive.md

> 在写代码之前，如果有不懂不清楚的地方，将问题写在这里，用户会在该文件中作答。

---

## 问题 1：Atom UI 主题包

计划中我写了 `AtomUI` NuGet 包作为 Avalonia 主题，但我搜索了 Avalonia 生态，没有找到名为 "Atom UI" 的成熟主题库。Avalonia 主流主题有：

| 主题             | 包名                         | 特点                    |
| -------------- | -------------------------- | --------------------- |
| Fluent（官方）     | `Avalonia.Themes.Fluent`   | 官方默认，类似 Win10/11 风格   |
| Material       | `Avalonia.Themes.Material` | Material Design 风格    |
| FluentAvalonia | `FluentAvalonia`           | Win11 Fluent 增强，带更多控件 |

**请确认：您想要的 Atom UI 具体是指哪个 NuGet 包？还是指 FluentAvalonia？**

回答：AtomUI地址：[https://github.com/AtomUI/AtomUI](https://github.com/AtomUI/AtomUI/tree/release/6.0/controlgallery)

[使用方法：https://github.com/AtomUI/AtomUI/tree/release/6.0/controlgallery](https://github.com/AtomUI/AtomUI/tree/release/6.0/controlgallery)

---

## 问题 2：目标平台

Avalonia 支持 Windows、macOS、Linux。但不同平台有差异：

- `Process.Start` 在 Linux/macOS 上需要特殊处理（不能用 `explorer.exe`）
- 文件选择器在不同平台行为不同

**请确认：目标平台是仅 Windows，还是 Windows + macOS + Linux？**

回答：支持windows+macos+linux

---

## 问题 3：文件选择器

Avalonia 内置 `OpenFileDialog` / `FolderPicker`，但 Win11 风格不够现代。可选方案：

| 方案             | 说明                            |
| -------------- | ----------------------------- |
| Avalonia 内置    | 跨平台，Win11 风格不够现代              |
| Avalonia.Win32 | 仅 Windows 使用原生 Win32 API，体验最好 |

**请确认：文件选择器使用 Avalonia 内置即可，还是需要额外的第三方库？**

回答：Avalonia 内置

---

## 问题 4：对话框弹出方式

计划中对话框使用 `ShowDialog()` 模态弹出。

**请确认：对话框（添加项目、设置等）使用** **`ShowDialog()`** **模态弹窗是否可接受？**

回答：接受

---

## 问题 5：Flurl.Http vs Refit

我选择了 `Flurl.Http` 作为 HTTP 客户端（比 Refit 更灵活，适合文件上传/下载）。

**请确认：使用 Flurl.Http 是否可接受？**\
回答：接受

---

## 问题 6：Microsoft.Extensions.DependencyInjection

我选择了 `Microsoft.Extensions.DependencyInjection` 而非 CommunityToolkit 的 DI，以获得更完整的 DI 容器功能。

**请确认：使用 Microsoft.Extensions.DependencyInjection 是否可接受？**

回答：接受

---

## 问题 7：AtomUI 6.0 版本更新

**用户要求：AtomUI 使用预览版 6.0.0 的最新版本，然后检查使用 AtomUI 的最新 API**

### AtomUI 6.0 关键变化

1. **NuGet 包拆分**：从单一包 `AtomUI` 拆分为多个独立包：
   - `AtomUI.Core` — 核心基础设施（主题系统、Token 系统、动画）
   - `AtomUI.Controls.Shared` — 共享接口与枚举
   - `AtomUI.Desktop.Controls` — 桌面控件库（主要安装包）
   - `AtomUI.Fonts.AlibabaSans` — 阿里巴巴普惠体字体包

2. **启用方式变更**：从 XAML 中 `<atom:AtomTheme />` 改为代码中 `UseAtomUI()` 扩展方法

3. **Avalonia 版本升级**：AtomUI 6.0 基于 Avalonia 12.x

4. **依赖 ReactiveUI**：AtomUI 6.0 需要 ReactiveUI 作为底层依赖

### 已更新的代码

**PublishTool.csproj** — 更新 NuGet 包：
```xml
<PackageReference Include="Avalonia" Version="12.0.0-preview1" />
<PackageReference Include="AtomUI.Core" Version="6.0.0-build.2" />
<PackageReference Include="AtomUI.Controls.Shared" Version="6.0.0-build.2" />
<PackageReference Include="AtomUI.Desktop.Controls" Version="6.0.0-build.2" />
<PackageReference Include="AtomUI.Fonts.AlibabaSans" Version="6.0.0-build.2" />
<PackageReference Include="ReactiveUI" Version="21.0.1" />
```

**App.axaml** — 移除 `<atom:AtomTheme />`，只保留 `<FluentTheme />`

**App.axaml.cs** — 添加 `OnInitialized()` 方法启用 AtomUI：
```csharp
public override void OnInitialized()
{
    base.OnInitialized();

    this.UseAtomUI(builder =>
    {
        builder.WithDefaultTheme(AtomUI.Core.IThemeManager.DEFAULT_THEME_ID);
        builder.UseAlibabaSansFont();
        builder.UseDesktopControls();
    });
}
```

---

## 问题 8：UI 完善方向确认

我已经完整读懂了项目。当前项目基于 **Avalonia 12 + CommunityToolkit.Mvvm + AtomUI 6.0**，是一个客户端版本发布工具，功能包括项目管理、文件比对、版本上传、增量拉取等。

### 当前 UI 已完成的优化
根据 publish_tool_plan.md，已完成：
- 文件列表：文件大小、差异状态、全选
- 上传队列：状态图标、进度统计、重试按钮
- 服务器信息栏：磁盘格式优化
- 工具栏：分组显示
- 项目设置：服务器地址显示、忽略配置入口
- 底部栏：停止按钮

### 我发现的仍有优化空间的点

| # | 问题 | 说明 |
|---|------|------|
| 1 | **文件差异状态未显示** | `LocalFileItem.CompareStatus` 和 `StatusToColorConverter` 已存在，但 `ProjectPage.axaml` 文件列表中没有显示差异标签（新增/修改） |
| 2 | **「添加到上传队列」按钮缺失** | `AddToUploadQueue` 和 `RemoveFromQueue` 命令在 ViewModel 中，但 `ProjectPage.axaml` 中没有对应的触发按钮 |
| 3 | **服务端信息未展示** | `ServerOsInfo`（CPU、磁盘、OS）已获取但从未在 UI 中展示 |
| 4 | **硬编码颜色** | `StatusToColorConverter`, `BoolToColorConverter`, `UploadStatusToColorConverter` 使用硬编码颜色而非 `DynamicResource` 主题令牌，影响深色主题兼容性 |
| 5 | **未使用的 UserControl** | `LocalFilesPanel.axaml` 和 `UploadQueuePanel.axaml` 似乎是旧版产物，已被 ProjectPage 的内联内容替代 |
| 6 | **上传队列缺少单文件进度** | 每个上传项没有独立的进度条显示 |
| 7 | **搜索框未绑定过滤逻辑** | `LocalFilesPanel.axaml` 中的过滤 TextBox 未绑定任何命令 |
| 8 | **输入验证缺失** | 对话框中缺少输入校验提示（如空值、格式错误） |

### 请确认优先处理方向

请回复以下编号，告诉我你希望我优先处理哪些问题（可多选）：

**A. 展示层增强** — 解决 1、2、3（差异状态、添加队列按钮、服务器信息展示）
**B. 视觉打磨** — 解决 4、6、7、8（主题色、进度条、验证、未使用控件清理）
**C. 全面优化** — 一次解决所有 8 个问题
**D. 其他想法** — 你有自己的 UI 改进思路

回答：B
