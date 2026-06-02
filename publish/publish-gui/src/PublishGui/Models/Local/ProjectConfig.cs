using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishGui.Models.Local;

/// <summary>
/// 项目配置（本地持久化）
/// </summary>
public partial class ProjectConfig : ObservableObject
{
    /// <summary>服务器地址</summary>
    [ObservableProperty]
    private string _serverUrl = string.Empty;

    /// <summary>项目名称</summary>
    [ObservableProperty]
    private string _projectName = string.Empty;

    /// <summary>本地路径</summary>
    [ObservableProperty]
    private string _projectPath = string.Empty;

    /// <summary>项目 ID</summary>
    [ObservableProperty]
    private int _projectId;

    /// <summary>publish-cli 路径</summary>
    [ObservableProperty]
    private string _publishCliPath = string.Empty;
}
