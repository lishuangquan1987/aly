using System;
using System.Globalization;
using System.IO;
using System.Text.Json;
using Irihi.Lingua;

namespace AlyPublish.Localization;

/// <summary>
/// JSON-backed Lingua manager. All XAML strings use {Translate ...} and update
/// automatically when <see cref="Toggle"/> or <see cref="Initialize"/> changes
/// the culture.
/// </summary>
[LinguaManager("./Resources/Strings.json")]
public partial class LanguageManager
{
    private const string SettingsDirectoryName = "AlyPublish";
    private const string SettingsFileName = "settings.json";

    /// <summary>Set from Program.Main (--lang) before Avalonia starts.</summary>
    public static string? RequestedLanguage { get; set; }

    public static event EventHandler? LanguageChanged;

    public static bool IsChinese => Instance.CurrentCulture.Name.StartsWith("zh", StringComparison.OrdinalIgnoreCase);

    public static string LanguageLabel
        => IsChinese ? T(Instance.Common_Language_English) : T(Instance.Common_Language_Chinese);

    /// <summary>Current translated value of a generated observable string.</summary>
    public static string T(IObservable<string?> value)
        => value is LinguaObservableString observable ? observable.CurrentValue ?? string.Empty : string.Empty;

    /// <summary>Formats a localized template with the current culture.</summary>
    public static string F(IObservable<string?> value, params object?[] args)
        => F(T(value), args);

    /// <summary>Formats an already-resolved localized template.</summary>
    public static string F(string format, params object?[] args)
        => string.Format(CultureInfo.CurrentCulture, format, args);

    /// <summary>
    /// 上传进度行的状态文案（审查 publish 轻微⑤）。
    ///
    /// 旧实现在 DTO（Models/Cli/UploadProgress.cs）里硬编码中文，Lingua 双语
    /// 基础设施已存在却没用上——英文界面会混进中文。文案属展示逻辑，统一收在这里。
    /// 注意：StatusText 是计算属性，切换语言不会自动刷新已有进度行；
    /// 进度行本身在每次发布时重建，可接受（需要实时切换时应改为 INotifyPropertyChanged）。
    /// </summary>
    public static string UploadStatusText(string status, string error)
    {
        switch (status)
        {
            case "START":
                return T(Instance.ProjectTab_Upload_Starting);
            case "DONE":
                return T(Instance.ProjectTab_Upload_Done);
            case "FAIL":
                return string.IsNullOrWhiteSpace(error)
                    ? T(Instance.ProjectTab_Upload_Failed)
                    : F(T(Instance.ProjectTab_Upload_FailedWithReason), error);
            default:
                return status;
        }
    }

    public static void Initialize()
    {
        var culture = Resolve(RequestedLanguage) ?? LoadSavedCulture() ?? DetectCulture();
        Instance.UpdateCulture(culture);
    }

    public static void Toggle()
    {
        var next = IsChinese ? CultureInfo.InvariantCulture : new CultureInfo("zh-Hans");
        Instance.UpdateCulture(next);
        SaveLanguage(next);
        LanguageChanged?.Invoke(null, EventArgs.Empty);
    }

    private static CultureInfo? Resolve(string? language)
    {
        if (string.IsNullOrWhiteSpace(language))
        {
            return null;
        }

        return language.Trim().StartsWith("zh", StringComparison.OrdinalIgnoreCase)
            ? new CultureInfo("zh-Hans")
            : CultureInfo.InvariantCulture;
    }

    private static CultureInfo DetectCulture()
        => CultureInfo.CurrentUICulture.Name.StartsWith("zh", StringComparison.OrdinalIgnoreCase)
            ? new CultureInfo("zh-Hans")
            : CultureInfo.InvariantCulture;

    private static CultureInfo? LoadSavedCulture()
    {
        try
        {
            var path = SettingsPath();
            if (!File.Exists(path))
            {
                return null;
            }

            using var document = JsonDocument.Parse(File.ReadAllText(path));
            if (document.RootElement.TryGetProperty("language", out var value))
            {
                return Resolve(value.GetString());
            }
        }
        catch (Exception)
        {
            // Settings are best-effort; never block startup.
        }

        return null;
    }

    private static void SaveLanguage(CultureInfo culture)
    {
        try
        {
            var path = SettingsPath();
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(
                path,
                JsonSerializer.Serialize(new { language = culture.Name.Length == 0 ? "en-US" : culture.Name }));
        }
        catch (Exception)
        {
            // Persisting the UI language must never break a publish operation.
        }
    }

    private static string SettingsPath()
        => Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            SettingsDirectoryName,
            SettingsFileName);
}