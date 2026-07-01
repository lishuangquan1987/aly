namespace AlyPublish.Constants;

/// <summary>
/// 文件状态常量
/// </summary>
public static class FileStatus
{
    public const string New = "new";
    public const string Modified = "modified";
    public const string Deleted = "deleted";
    public const string Unchanged = "unchanged";

    /// <summary>
    /// 获取状态的中文显示文本
    /// </summary>
    public static string GetDisplayText(string status) => status switch
    {
        New => "新增",
        Modified => "修改",
        Deleted => "删除",
        Unchanged => "无变化",
        _ => status
    };
}

/// <summary>
/// 应用程序常量
/// </summary>
public static class AppConstants
{
    public const string AppName = "AlyPublish";
    public const string AppTitle = "AlyPublish - 版本发布工具";
    public const string ConfigDirectoryName = "AlyPublish";
    public const string ConfigFileName = "config.json";
    public const string LogDirectoryName = "logs";
    public const string LogFileName = "log.log";
    public const string CliExecutableName = "aly-publish.exe";
    public const string DefaultServerUrl = "http://localhost:2000";
    public const string JsonOutputArg = "--json";
    public const string PathArgPrefix = "--path";
    public const string ServerArgPrefix = "--server";
    public const string ProjectArgPrefix = "--project";
    public const string IdArgPrefix = "--id";
    public const string VersionArgPrefix = "--version";
    public const string MessageArgPrefix = "--message";
}

