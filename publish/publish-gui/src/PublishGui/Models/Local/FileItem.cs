using CommunityToolkit.Mvvm.ComponentModel;
using PublishGui.Constants;

namespace PublishGui.Models.Local;

/// <summary>
/// 文件列表项（用于 UI 绑定）
/// </summary>
public partial class FileItem : ObservableObject
{
    /// <summary>相对路径</summary>
    [ObservableProperty]
    private string _relativePath = string.Empty;

    /// <summary>状态（new/modified/deleted/unchanged）</summary>
    [ObservableProperty]
    private string _status = string.Empty;

    /// <summary>本地文件大小（字节）</summary>
    [ObservableProperty]
    private long _localSize;

    /// <summary>服务端文件大小（字节）</summary>
    [ObservableProperty]
    private long _serverSize;

    /// <summary>是否选中</summary>
    [ObservableProperty]
    private bool _isSelected = true;

    /// <summary>状态的中文显示文本</summary>
    public string StatusDisplay => FileStatus.GetDisplayText(Status);
}
