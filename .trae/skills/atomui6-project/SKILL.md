---
name: "atomui6-project"
description: "AtomUI 6.0 + Avalonia 12 项目初始化和配置指南。包含 NuGet 包结构、命名空间、App.axaml.cs 初始化方式。当项目使用 AtomUI 6.0 预览版时使用此技能。"
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

## 二、App.axaml.cs 初始化

**关键坑**：Avalonia 12 移除了 `Application.OnInitialized()`，`UseAtomUI()` 必须放在 `Initialize()` 中调用。

```csharp
using AtomUI;
using AtomUI.Desktop.Controls;
using AtomUI.Theme;

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

---

## 三、命名空间对应关系

| 命名空间 | NuGet 包 | 用途 |
|---------|----------|------|
| `AtomUI` | AtomUI.Desktop.Controls | `UseAtomUI()` 扩展方法 |
| `AtomUI.Theme` | AtomUI.Core | `IThemeManager` 接口 |
| `AtomUI.Desktop.Controls` | AtomUI.Desktop.Controls | 桌面控件 |

**常见错误**：
- ❌ `using AtomUI.Core;` — 该命名空间在 6.0 中不存在，`IThemeManager` 在 `AtomUI.Theme` 中
- ❌ `using AtomUI.Theme;` 编译失败 — 缺少对 `AtomUI.Core` NuGet 包的引用（类在 `AtomUI.Core` 包里，但命名空间是 `AtomUI.Theme`）

---

## 四、Program.cs ReactiveUI 配置

Avalonia 12 中 `UseReactiveUI()` 需要传入 lambda 参数：

```csharp
using ReactiveUI.Avalonia;

public static AppBuilder BuildAvaloniaApp()
    => AppBuilder.Configure<App>()
        .UseReactiveUI(_ => { })  // 注意：ReactiveUI 23.x 需要 lambda
        .UsePlatformDetect()
        .WithInterFont()
        .LogToTrace();
```

---

## 五、Avalonia 12 包变更

| 旧包 | 新包 |
|------|------|
| `Avalonia.Diagnostics` | `AvaloniaUI.DiagnosticsSupport`（Community 维护） |

`Avalonia.Diagnostics` 在 Avalonia 12 中已被移除。