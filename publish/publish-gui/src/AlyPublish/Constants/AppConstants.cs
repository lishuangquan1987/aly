using AlyPublish.Localization;

namespace AlyPublish.Constants;

public static class FileStatus
{
    public const string New = "new";
    public const string Modified = "modified";
    public const string Deleted = "deleted";
    public const string Unchanged = "unchanged";

    public static string GetDisplayText(string status) => status switch
    {
        New => LanguageManager.T(LanguageManager.Instance.Status_New),
        Modified => LanguageManager.T(LanguageManager.Instance.Status_Modified),
        Deleted => LanguageManager.T(LanguageManager.Instance.Status_Deleted),
        Unchanged => LanguageManager.T(LanguageManager.Instance.Status_Unchanged),
        _ => status
    };
}

public static class AppConstants
{
    public const string AppName = "AlyPublish";
    public const string ConfigDirectoryName = "AlyPublish";
    public const string ConfigFileName = "config.json";
    public const string LogDirectoryName = "logs";
    public const string LogFileName = "log.log";
    public const string JsonOutputArgument = "--json";

    public static IReadOnlyList<string> IgnoreFolderSuggestions { get; } =
        ["node_modules", ".git", ".vs", "bin", "obj", "dist", "logs"];

    public static IReadOnlyList<string> IgnoreFileSuggestions { get; } =
        ["*.log", "*.tmp", "*.pdb", "Thumbs.db", ".DS_Store"];
}
