using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.Models.Local;

public enum UploadStatus
{
    Pending,
    Uploading,
    Done,
    Failed
}

public partial class UploadFileItem : ObservableObject
{
    [ObservableProperty]
    private string _fileName = string.Empty;

    [ObservableProperty]
    private string _localPath = string.Empty;

    [ObservableProperty]
    private string _relativePath = string.Empty;

    [ObservableProperty]
    private DateTime _lastModified = DateTime.MinValue;

    [ObservableProperty]
    private UploadStatus _status = UploadStatus.Pending;

    [ObservableProperty]
    private bool _isChecked;

    [ObservableProperty]
    private int _uploadProgress;
}
