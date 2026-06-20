using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Microsoft.Extensions.DependencyInjection;
using ZapPublish.Services;
using ZapPublish.ViewModels;
using ZapPublish.Views;
using Serilog;
using System;
using System.IO;

namespace ZapPublish;

public partial class App : Application
{
    public IServiceProvider Services { get; private set; } = null!;

    public override void Initialize() { AvaloniaXamlLoader.Load(this); }

    public override void OnFrameworkInitializationCompleted()
    {
        var logDir = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "logs");
        try
        {
            Directory.CreateDirectory(logDir);
        }
        catch (Exception ex)
        {
            // 日志目录创建失败时使用系统临时目录作为回退
            logDir = Path.Combine(Path.GetTempPath(), "ZapPublish", "logs");
            try { Directory.CreateDirectory(logDir); } catch { }
            System.Diagnostics.Debug.WriteLine($"创建日志目录失败，回退到 {logDir}: {ex.Message}");
        }
        Log.Logger = new LoggerConfiguration()
#if DEBUG
            .MinimumLevel.Debug()
#else
            .MinimumLevel.Information()
#endif
            .WriteTo.File(
                Path.Combine(logDir, "log.log"),
                rollingInterval: RollingInterval.Day,
                outputTemplate: "{Timestamp:yyyy-MM-dd HH:mm:ss.fff} [{Level:u3}] {Message:lj}{NewLine}{Exception}")
            .CreateLogger();

        Log.Information("=== ZapPublish 启动 ===");
        Log.Information("日志目录: {Dir}", logDir);

        var svc = new ServiceCollection();
        svc.AddSingleton<ProcessService>();
        svc.AddSingleton<CliService>();
        svc.AddSingleton<ConfigService>();
        svc.AddTransient<MainWindowViewModel>();
        svc.AddTransient<AddProjectDialogViewModel>();
        svc.AddTransient<AddLocalProjectDialogViewModel>();
        svc.AddTransient<CreateProjectDialogViewModel>();
        svc.AddTransient<EditProjectDialogViewModel>();
        Services = svc.BuildServiceProvider();

        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            desktop.MainWindow = new MainWindow { DataContext = Services.GetRequiredService<MainWindowViewModel>() };
            desktop.ShutdownRequested += (_, _) =>
            {
                Log.Information("=== ZapPublish 关闭 ===");
                Log.CloseAndFlush();
            };
        }

        base.OnFrameworkInitializationCompleted();
    }
}
