using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Avalonia.Threading;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Services;
using PublishTool.ViewModels;
using PublishTool.Views;
using Serilog;

namespace PublishTool;

public partial class App : Application
{
    public static IServiceProvider Services { get; private set; } = null!;

    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
    }

    public override void OnFrameworkInitializationCompleted()
    {
        SetupExceptionHandling();

        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            desktop.ShutdownRequested += OnShutdownRequested;

            var services = new ServiceCollection();
            ConfigureServices(services);
            Services = services.BuildServiceProvider();

            desktop.MainWindow = new MainWindow
            {
                DataContext = Services.GetRequiredService<MainWindowViewModel>()
            };

            Log.Information("应用程序启动成功");
        }

        base.OnFrameworkInitializationCompleted();
    }

    private void SetupExceptionHandling()
    {
        AppDomain.CurrentDomain.UnhandledException += (sender, args) =>
        {
            var ex = args.ExceptionObject as Exception;
            Log.Fatal(ex, "未处理的 AppDomain 异常");
            if (args.IsTerminating)
            {
                Log.CloseAndFlush();
            }
        };

        TaskScheduler.UnobservedTaskException += (sender, args) =>
        {
            Log.Error(args.Exception, "未观察到的 Task 异常");
            args.SetObserved();
        };

        Dispatcher.UIThread.UnhandledException += (sender, args) =>
        {
            Log.Error(args.Exception, "UI 线程未处理的异常");
            args.Handled = true;
            ShowErrorDialog(args.Exception);
        };
    }

    private static void ShowErrorDialog(Exception ex)
    {
        try
        {
            Log.Warning("显示错误对话框: {Message}", ex.Message);
        }
        catch
        {
        }
    }

    private void OnShutdownRequested(object? sender, ShutdownRequestedEventArgs e)
    {
        Log.Information("应用程序正在关闭");
    }

    private void ConfigureServices(IServiceCollection services)
    {
        services.AddSingleton<ConfigService>();
        services.AddSingleton<LocalFileService>();
        services.AddSingleton<ProcessService>();
        services.AddSingleton<ProjectService>();
        services.AddSingleton<FileService>();

        services.AddTransient<MainWindowViewModel>();
        services.AddTransient<ProjectPageViewModel>();
    }
}
