using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishGui.Models.Local;

public partial class ProjectConfig : ObservableObject
{
    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private string _projectName = string.Empty;

    [ObservableProperty]
    private string _projectPath = string.Empty;

    [ObservableProperty]
    private int _projectId;

    [ObservableProperty]
    private string _publishCliPath = string.Empty;
}

public partial class FileItem : ObservableObject
{
    [ObservableProperty]
    private string _relativePath = string.Empty;

    [ObservableProperty]
    private string _status = string.Empty;

    [ObservableProperty]
    private long _localSize;

    [ObservableProperty]
    private long _serverSize;

    [ObservableProperty]
    private bool _isSelected = true;

    public string StatusDisplay => Status switch
    {
        "new" => "新增",
        "modified" => "修改",
        "deleted" => "删除",
        "unchanged" => "无变化",
        _ => Status
    };
}
