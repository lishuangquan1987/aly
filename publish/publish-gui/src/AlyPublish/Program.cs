using System;
using AlyPublish.Localization;
using Avalonia;

namespace AlyPublish;

internal static class Program
{
    [STAThread]
    public static void Main(string[] args)
    {
        var languageIndex = Array.IndexOf(args, "--lang");
        if (languageIndex >= 0 && languageIndex + 1 < args.Length)
        {
            LanguageManager.RequestedLanguage = args[languageIndex + 1];
        }

        LanguageManager.Initialize();

        BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);
    }

    public static AppBuilder BuildAvaloniaApp()
        => AppBuilder.Configure<App>()
            .UsePlatformDetect()
            .WithInterFont()
            .LogToTrace();
}