using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishGui.Models.Local;

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
    private string _localMd5 = string.Empty;

    [ObservableProperty]
    private string _serverMd5 = string.Empty;

    [ObservableProperty]
    private bool _isSelected = true;

    public string StatusDisplay => Status switch
    {
        "new" => "新增",
        "modified" => "修改",
        "deleted" => "删除",
        _ => Status
    };

    public static FileItem FromCliItem(Models.Cli.FileStatusItem item) => new()
    {
        RelativePath = item.RelativePath,
        Status = item.Status,
        LocalSize = item.LocalSize,
        ServerSize = item.ServerSize,
        LocalMd5 = item.LocalMd5,
        ServerMd5 = item.ServerMd5,
        IsSelected = item.Status is "new" or "modified"
    };
}