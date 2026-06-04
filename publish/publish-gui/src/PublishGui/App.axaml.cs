using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Microsoft.Extensions.DependencyInjection;
using PublishGui.Services;
using PublishGui.ViewModels;
using PublishGui.Views;
using Serilog;
using System;
using System.IO;

namespace PublishGui;

public partial class App : Application
{
    public IServiceProvider Services { get; private set; } = null!;

    public override void Initialize() { AvaloniaXamlLoader.Load(this); }

    public override void OnFrameworkInitializationCompleted()
    {
        var logDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "PublishGui", "logs");
        Directory.CreateDirectory(logDir);
        Log.Logger = new LoggerConfiguration()
            .WriteTo.File(Path.Combine(logDir, "log.log"), rollingInterval: RollingInterval.Day)
            .CreateLogger();

        var svc = new ServiceCollection();
        svc.AddSingleton<ProcessService>();
        svc.AddSingleton<CliService>();
        svc.AddSingleton<ConfigService>();
        svc.AddSingleton<MainWindowViewModel>();
        Services = svc.BuildServiceProvider();

        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
            desktop.MainWindow = new MainWindow { DataContext = Services.GetRequiredService<MainWindowViewModel>() };

        base.OnFrameworkInitializationCompleted();
    }
}