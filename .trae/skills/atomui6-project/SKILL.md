---
name: "atomui6-project"
description: "AtomUI 6.0 + Avalonia 12 项目配置完整指南。包含 NuGet 包结构、命名空间、App.axaml.cs 初始化、控件映射、主题令牌、构建配置。当项目使用 AtomUI 6.0 预览版时使用此技能。"
---

# AtomUI 6.0 + Avalonia 12 项目配置指南

> 基于 AtomUI 6.0.0-build.2 + Avalonia 12.0.3 实战踩坑总结。

---

## 一、NuGet 包依赖

AtomUI 6.0 拆分为 4 个独立包，必须全部安装：

```xml
<PackageReference Include="AtomUI.Core" Version="6.0.0-build.2" />
<PackageReference Include="AtomUI.Controls.Shared" Version="6.0.0-build.2" />
<PackageReference Include="AtomUI.Desktop.Controls" Version="6.0.0-build.2" />
<PackageReference Include="AtomUI.Fonts.AlibabaSans" Version="6.0.0-build.2" />
```

### 版本兼容性

| 依赖 | 要求 |
|------|------|
| Avalonia.Desktop | `>= 12.0.3` |
| ReactiveUI.Avalonia | `>= 12.0.1`（对应 ReactiveUI `>= 23.2.1`） |

---

## 二、XAML 命名空间

所有使用 AtomUI 控件的 XAML 文件**必须**添加专用命名空间：

```xml
<atom:Window xmlns:atom="https://atomui.net"
             xmlns="https://github.com/avaloniaui"
             xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
```

- ✅ 使用 `https://atomui.net`
- ❌ 不要使用 `xmlns:atom="using:AtomUI.Controls"`（不兼容）

控件通过 `atom:` 前缀引用，如 `atom:Button`、`atom:TextBox`。

---

## 三、AtomUI 控件映射

| 标准 Avalonia 控件 | AtomUI 6.0 等价控件 | 说明 |
|-------------------|---------------------|------|
| `Window` | `atom:Window` | Ant Design 风格窗口 |
| `Button` | `atom:Button` | 使用 `ButtonType`（Primary/Default/Text），不支持 `Size`/`Status` |
| `TextBox` | `atom:TextBox` | Ant Design 风格输入框 |
| `TabControl` | `atom:TabControl` | 标签页控件 |
| `ProgressBar` | `atom:ProgressBar` | 进度条 |
| `CheckBox` | `atom:CheckBox` | 复选框 |
| `ListBox` | `atom:ListBox` | 列表控件 |
| `ScrollViewer` | `atom:ScrollViewer` | 滚动容器 |
| `RadioButton` | `atom:RadioButton` | 单选按钮 |
| `ToggleSwitch` | `atom:ToggleSwitch` | 开关 |
| `ToolTip` | `atom:ToolTip` | 工具提示 |
| `Menu` | `atom:Menu` | 菜单 |

### Button 属性说明

```xml
<!-- ✅ 主要按钮（蓝色背景，白色文字） -->
<atom:Button Content="提交" ButtonType="Primary" />

<!-- ✅ 默认按钮 -->
<atom:Button Content="取消" ButtonType="Default" />

<!-- ✅ 文本按钮（无背景，仅文字） -->
<atom:Button Content="链接" ButtonType="Text" />

<!-- ❌ 不支持 Size 和 Status 属性 -->
<!-- <atom:Button Size="Small" Status="Danger" />  -->
```

---

## 四、App.axaml — 必须移除 FluentTheme

AtomUI 6.0 自带完整的 Ant Design 主题系统，**必须移除** `<FluentTheme />`：

```xml
<!-- App.axaml -->
<Application xmlns="https://github.com/avaloniaui"
             xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
             x:Class="MyApp.App">
  <!-- ❌ 删除 Application.Styles 中的 FluentTheme -->
  <!-- <Application.Styles>
    <FluentTheme />
  </Application.Styles> -->
</Application>
```

---

## 五、App.axaml.cs 初始化（Avalonia 12 关键坑）

**关键坑**：Avalonia 12 移除了 `Application.OnInitialized()`，`UseAtomUI()` 必须放在 `Initialize()` 中调用。

```csharp
using AtomUI;
using AtomUI.Desktop.Controls;
using AtomUI.Theme;
using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;

public partial class App : Application
{
    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
        this.UseAtomUI(builder =>
        {
            builder.WithDefaultTheme(IThemeManager.DEFAULT_THEME_ID);
            builder.UseAlibabaSansFont();
            builder.UseDesktopControls();
            // builder.UseDesktopDataGrid();      // 可选：启用 DataGrid
            // builder.UseDesktopColorPicker();   // 可选：启用 ColorPicker
        });
    }

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            var services = new ServiceCollection();
            ConfigureServices(services);
            var sp = services.BuildServiceProvider();

            desktop.MainWindow = new MainWindow
            {
                DataContext = sp.GetRequiredService<MainWindowViewModel>()
            };
        }
        base.OnFrameworkInitializationCompleted();
    }
}
```

### 命名空间对应关系

| 命名空间 | NuGet 包 | 用途 |
|---------|----------|------|
| `AtomUI` | AtomUI.Desktop.Controls | `UseAtomUI()` 扩展方法 |
| `AtomUI.Theme` | AtomUI.Core | `IThemeManager` 接口 |
| `AtomUI.Desktop.Controls` | AtomUI.Desktop.Controls | 桌面控件 |

**常见错误**：
- ❌ `using AtomUI.Core;` — 该命名空间在 6.0 中不存在，`IThemeManager` 在 `AtomUI.Theme` 中
- ❌ `using AtomUI.Theme;` 编译失败 — 缺少对 `AtomUI.Core` NuGet 包的引用（类在 `AtomUI.Core` 包里，但命名空间是 `AtomUI.Theme`）

---

## 六、Program.cs ReactiveUI 配置

Avalonia 12 中 `UseReactiveUI()` 需要传入 lambda 参数：

```csharp
using Avalonia;
using ReactiveUI.Avalonia;

public static class Program
{
    public static void Main(string[] args)
        => BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);

    public static AppBuilder BuildAvaloniaApp()
        => AppBuilder.Configure<App>()
            .UseReactiveUI(_ => { })  // Avalonia 12 + ReactiveUI 23.x 需要 lambda
            .UsePlatformDetect()
            .WithInterFont()
            .LogToTrace();
}
```

---

## 七、主题令牌（Design Tokens）

所有颜色必须通过 `{DynamicResource}` 引用 AtomUI 设计令牌，**禁止**使用硬编码颜色值。

### 常用令牌一览

| 令牌 | 用途 | 典型旧值 |
|------|------|---------|
| `BgLayout` | 布局背景（顶栏/底栏/侧栏） | `#0d0d0d` |
| `BgContainer` | 容器背景（内容区） | `#1a1a1a`、`#1e1e1e`、`#252525` |
| `TextBase` | 主要文字 | `White`、`#FFFFFF` |
| `TextSecondary` | 次要文字 | `#87ceeb` |
| `TextTertiary` | 辅助文字（提示、时间戳） | `#888888`、`#666666` |
| `PrimaryColor` | 主题色 | `#00a8ff` |
| `SuccessColor` | 成功/完成色 | `#00ff88` |
| `WarningColor` | 警告色 | `#faad14` |
| `ErrorColor` | 错误色 | `#ff4d4f` |
| `InfoColor` | 信息色 | `#1890ff` |
| `BorderColor` | 边框色 | `#2d2d2d`、`#3d3d3d` |
| `ControlStrokeColorSecondary` | 控件次要描边 | — |

### 使用示例

```xml
<Border Background="{DynamicResource BgContainer}"
        BorderBrush="{DynamicResource BorderColor}"
        BorderThickness="1"
        CornerRadius="4">
  <StackPanel Spacing="8" Margin="12">
    <TextBlock Text="标题" Foreground="{DynamicResource TextBase}"
               FontSize="14" FontWeight="SemiBold" />
    <TextBlock Text="辅助说明文字" Foreground="{DynamicResource TextTertiary}"
               FontSize="11" />
    <atom:Button Content="提交" ButtonType="Primary" />
    <atom:Button Content="取消" ButtonType="Default" />
  </StackPanel>
</Border>
```

---

## 八、构建配置

### 8.1 SkiaSharp PDB 文件缺失

AtomUI 6.0 依赖 SkiaSharp，可能遇到因缺少 `.pdb` 文件导致的 MSB3030 错误：

```
error MSB3030: 无法复制文件 ...\skiasharp.nativeassets.win32\...\libSkiaSharp.pdb
```

**解决方案**：在 `.csproj` 的 `<PropertyGroup>` 中添加：

```xml
<PropertyGroup>
  <AllowedReferenceRelatedFileExtensions>.dll</AllowedReferenceRelatedFileExtensions>
</PropertyGroup>
```

### 8.2 Avalonia 12 包变更

| 旧包 | 新包 |
|------|------|
| `Avalonia.Diagnostics` | `AvaloniaUI.DiagnosticsSupport`（Community 维护） |

`Avalonia.Diagnostics` 在 Avalonia 12 中已被移除。

---

## 九、综合示例

一个完整的 AtomUI 6.0 页面布局示例：

```xml
<atom:Window xmlns="https://github.com/avaloniaui"
             xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
             xmlns:atom="https://atomui.net"
             x:Class="MyApp.MainWindow"
             Title="示例窗口" Width="800" Height="600"
             WindowStartupLocation="CenterScreen">

  <Grid RowDefinitions="40,*,30">
    <!-- 顶栏 -->
    <Border Grid.Row="0" Background="{DynamicResource BgLayout}"
            BorderThickness="0,0,0,1"
            BorderBrush="{DynamicResource BorderColor}">
      <StackPanel Orientation="Horizontal" Spacing="8" Margin="12,0">
        <TextBlock Text="标题" Foreground="{DynamicResource TextBase}" />
        <atom:TextBox PlaceholderText="搜索..." Width="200" />
        <atom:Button Content="操作" ButtonType="Primary" />
      </StackPanel>
    </Border>

    <!-- 内容区 -->
    <Border Grid.Row="1" Background="{DynamicResource BgContainer}">
      <!-- 内容 -->
    </Border>

    <!-- 底栏 -->
    <Border Grid.Row="2" Background="{DynamicResource BgLayout}"
            BorderThickness="0,1,0,0"
            BorderBrush="{DynamicResource BorderColor}">
      <TextBlock Text="状态信息" Foreground="{DynamicResource TextTertiary}"
                 Margin="12,4" />
    </Border>
  </Grid>
</atom:Window>
```

---

## 十、快速参考速查表

### ❌ 禁止事项
1. ❌ `FluentTheme` — 由 `UseAtomUI()` 替代
2. ❌ `OnInitialized()` — Avalonia 12 已移除，使用 `Initialize()`
3. ❌ 硬编码颜色值（`#1a1a1a`、`#00a8ff` 等）— 使用 `{DynamicResource}`
4. ❌ `Button.Size` / `Button.Status` — 不支持，使用 `ButtonType`
5. ❌ `xmlns:atom="using:AtomUI.Controls"` — 使用 `https://atomui.net`
6. ❌ `using AtomUI.Core;` — 命名空间不存在

### ✅ 推荐实践
1. ✅ XAML 命名空间：`xmlns:atom="https://atomui.net"`
2. ✅ 主题初始化：`this.UseAtomUI(...)` 在 `Initialize()` 中
3. ✅ 颜色引用：`{DynamicResource PrimaryColor}` 等设计令牌
4. ✅ 按钮类型：`ButtonType="Primary"`、`ButtonType="Default"`、`ButtonType="Text"`
5. ✅ 构建配置：`<AllowedReferenceRelatedFileExtensions>.dll</...>`