using System;
using System.IO;
using AlyPublish.Constants;
using AlyPublish.Services;
using AlyPublish.ViewModels;
using AlyPublish.Views;
using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Microsoft.Extensions.DependencyInjection;
using Serilog;

namespace AlyPublish;

public partial class App : Application
{
    public IServiceProvider Services { get; private set; } = null!;

    public override void Initialize() => AvaloniaXamlLoader.Load(this);

    public override void OnFrameworkInitializationCompleted()
    {
        var logDirectory = ResolveLogDirectory();
        Log.Logger = new LoggerConfiguration()
#if DEBUG
            .MinimumLevel.Debug()
#else
            .MinimumLevel.Information()
#endif
            .WriteTo.File(
                Path.Combine(logDirectory, AppConstants.LogFileName),
                rollingInterval: RollingInterval.Day,
                outputTemplate: "{Timestamp:yyyy-MM-dd HH:mm:ss.fff} [{Level:u3}] {Message:lj}{NewLine}{Exception}",
                formatProvider: System.Globalization.CultureInfo.InvariantCulture)
            .CreateLogger();

        Log.Information("=== AlyPublish 启动 ===");
        Log.Information("日志目录: {Directory}", logDirectory);

        AppDomain.CurrentDomain.UnhandledException += (_, args) =>
            Log.Fatal(args.ExceptionObject as Exception, "未处理的异常");
        TaskScheduler.UnobservedTaskException += (_, args) =>
        {
            Log.Error(args.Exception, "未观察的任务异常");
            args.SetObserved();
        };

        var services = new ServiceCollection();
        services.AddSingleton<CliService>();
        services.AddSingleton<ConfigService>();
        services.AddSingleton<ProjectConfigService>();
        services.AddSingleton<ProjectSetupService>();
        services.AddSingleton<INotificationService, NotificationService>();
        services.AddSingleton<IDialogService, DialogService>();

        services.AddTransient<MainWindowViewModel>();
        services.AddTransient<AddProjectDialogViewModel>();
        services.AddTransient<AddLocalProjectDialogViewModel>();
        services.AddTransient<CreateProjectDialogViewModel>();
        services.AddTransient<EditProjectDialogViewModel>();

        Services = services.BuildServiceProvider();

        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            desktop.MainWindow = new MainWindow
            {
                DataContext = Services.GetRequiredService<MainWindowViewModel>()
            };
            desktop.ShutdownRequested += (_, _) =>
            {
                Log.Information("=== AlyPublish 关闭 ===");
                Log.CloseAndFlush();
            };
        }

        base.OnFrameworkInitializationCompleted();
    }

    /// <summary>Prefers the application folder, then LocalApplicationData, then the temp folder.</summary>
    private static string ResolveLogDirectory()
    {
        var candidates = new[]
        {
            Path.Combine(AppDomain.CurrentDomain.BaseDirectory, AppConstants.LogDirectoryName),
            Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                AppConstants.AppName,
                AppConstants.LogDirectoryName),
            Path.Combine(Path.GetTempPath(), AppConstants.AppName, AppConstants.LogDirectoryName)
        };

        foreach (var candidate in candidates)
        {
            try
            {
                Directory.CreateDirectory(candidate);
                return candidate;
            }
            catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
            {
                // Try the next candidate; logging must never prevent startup.
            }
        }

        return AppDomain.CurrentDomain.BaseDirectory;
    }
}
