using AlyPublish.Constants;
using AlyPublish.Helpers;
using AlyPublish.Localization;
using CommunityToolkit.Mvvm.ComponentModel;

namespace AlyPublish.Models.Local;

public sealed partial class FileItem : ObservableObject
{
    [ObservableProperty]
    private string _relativePath = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(StatusDisplay))]
    [NotifyPropertyChangedFor(nameof(IsNew))]
    [NotifyPropertyChangedFor(nameof(IsModified))]
    [NotifyPropertyChangedFor(nameof(IsDeleted))]
    private string _status = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(LocalSizeText))]
    private long _localSize;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(ServerSizeText))]
    private long _serverSize;

    [ObservableProperty]
    private string _localMd5 = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(ServerSizeText))]
    private string _serverMd5 = string.Empty;

    [ObservableProperty]
    private bool _isSelected;

    public string StatusDisplay => FileStatus.GetDisplayText(Status);

    public bool IsNew => Status == FileStatus.New;

    public bool IsModified => Status == FileStatus.Modified;

    public bool IsDeleted => Status == FileStatus.Deleted;

    public string LocalSizeText => FileSizeHelper.Format(LocalSize);

    public string ServerSizeText => string.IsNullOrEmpty(ServerMd5) ? "—" : FileSizeHelper.Format(ServerSize);

    public void RefreshLocalization() => OnPropertyChanged(nameof(StatusDisplay));

    public static FileItem FromCliItem(Models.Cli.FileStatusItem item) => new()
    {
        RelativePath = item.RelativePath,
        Status = item.Status,
        LocalSize = item.LocalSize,
        ServerSize = item.ServerSize,
        LocalMd5 = item.LocalMd5,
        ServerMd5 = item.ServerMd5
    };
}
