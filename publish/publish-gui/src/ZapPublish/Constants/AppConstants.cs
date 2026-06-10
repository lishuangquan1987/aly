namespace ZapPublish.Constants;

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
    public const string AppName = "Publish GUI";
    public const string AppTitle = "Publish GUI - 版本发布工具";
    public const string ConfigDirectoryName = "ZapPublish";
    public const string ConfigFileName = "config.json";
    public const string LogDirectoryName = "logs";
    public const string LogFileName = "log.log";
    public const string CliExecutableName = "zap-publish.exe";
    public const string DefaultServerUrl = "http://localhost:2000";
    public const string JsonOutputArg = "--json";
    public const string PathArgPrefix = "--path";
    public const string ServerArgPrefix = "--server";
    public const string ProjectArgPrefix = "--project";
    public const string IdArgPrefix = "--id";
    public const string VersionArgPrefix = "--version";
    public const string MessageArgPrefix = "--message";
}

/// <summary>
/// CLI 命令常量
/// </summary>
public static class CliCommands
{
    public const string Status = "status";
    public const string Add = "add";
    public const string AddAll = "add --all";
    public const string Reset = "reset";
    public const string ResetAll = "reset --all";
    public const string Staged = "staged";
    public const string Publish = "publish";
    public const string Push = "push";
    public const string Log = "log";
    public const string ConfigInit = "config init";
    public const string ConfigSet = "config set";
    public const string ConfigList = "config list";
    public const string ProjectList = "project list";
    public const string ProjectCreate = "project create";
    public const string ProjectDelete = "project delete";
    public const string ServerInfo = "server info";
}

/// <summary>
/// UI 文本常量
/// </summary>
public static class UiTexts
{
    // 状态消息
    public const string Ready = "就绪";
    public const string CliNotFound = "未找到 publish-cli，请在项目设置中配置路径";
    public const string CliFound = "已找到 publish-cli";
    public const string RefreshingStatus = "正在刷新状态...";
    public const string AddingAllChanges = "正在添加所有变更...";
    public const string ClearingStaging = "正在清空暂存区...";
    public const string Publishing = "正在发布...";
    public const string CreatingProject = "正在创建项目...";
    
    // 成功消息
    public const string AddedAllSuccess = "已添加所有变更到暂存区";
    public const string StagingCleared = "暂存区已清空";
    public const string PublishSuccess = "发布成功";
    public const string ProjectAdded = "已添加项目";
    public const string ProjectRemoved = "已移除项目";
    public const string ProjectCreated = "项目创建成功";
    public const string SettingsUpdated = "已更新项目设置";
    
    // 错误消息前缀
    public const string RefreshFailed = "刷新失败";
    public const string AddFailed = "添加失败";
    public const string ClearFailed = "清空失败";
    public const string PublishFailed = "发布失败";
    public const string CreateProjectFailed = "创建项目失败";
    public const string InitConfigFailed = "初始化本地配置失败";
    public const string GetServerInfoFailed = "获取服务器信息失败";
    
    // 提示文本
    public const string EnterVersion = "请输入版本号";
    public const string EnterCommitMessage = "请输入变更说明";
    public const string VersionPlaceholder = "例如: V1.0.1";
    public const string CommitMessagePlaceholder = "输入变更说明...";
    
    // 按钮文本
    public const string RefreshStatus = "刷新状态";
    public const string AddAllChanges = "添加所有变更";
    public const string ClearStaging = "清空暂存区";
    public const string PublishNewVersion = "发布新版本";
    public const string AddExistingProject = "添加已有项目";
    public const string CreateNewProject = "新建项目";
    public const string RemoveProject = "移除项目";
    public const string ServerInfoButton = "服务器信息";
    public const string ProjectSettings = "项目设置";
    public const string Cancel = "取消";
    public const string Confirm = "确认";
    public const string Save = "保存";
    public const string Browse = "浏览...";
    public const string Create = "创建项目";
    
    // 面板标题
    public const string ProjectList = "项目列表";
    public const string ServerInfoPanel = "服务器信息";
    public const string UnstagedFiles = "未暂存文件";
    public const string StagedFiles = "暂存文件";
    public const string PublishPanel = "发布";
    public const string CurrentVersion = "当前版本";
    public const string NewVersion = "新版本号";
    public const string ChangeDescription = "变更说明";
    public const string VersionHistory = "版本历史";
    
    // 对话框标题
    public const string AddProjectDialogTitle = "添加已有项目";
    public const string CreateProjectDialogTitle = "新建项目";
    public const string ProjectSettingsDialogTitle = "项目设置";
}
