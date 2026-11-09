namespace AlyPublish.Constants;

public static class FileStatus
{
    public const string New = "new";
    public const string Modified = "modified";
    public const string Deleted = "deleted";
    public const string Unchanged = "unchanged";

    public static string GetDisplayText(string status) => status switch
    {
        New => "新增",
        Modified => "修改",
        Deleted => "删除",
        Unchanged => "无变化",
        _ => status
    };
}

public static class AppConstants
{
    public const string AppName = "AlyPublish";
    public const string AppTitle = "AlyPublish - 版本发布工具";
    public const string ConfigDirectoryName = "AlyPublish";
    public const string ConfigFileName = "config.json";
    public const string LogDirectoryName = "logs";
    public const string LogFileName = "log.log";
    public const string CliExecutableName = "aly-publish.exe";
    public const string JsonOutputArgument = "--json";

    public static IReadOnlyList<string> IgnoreFolderSuggestions { get; } =
        ["node_modules", ".git", ".vs", "bin", "obj", "dist", "logs"];

    public static IReadOnlyList<string> IgnoreFileSuggestions { get; } =
        ["*.log", "*.tmp", "*.pdb", "Thumbs.db", ".DS_Store"];
}
