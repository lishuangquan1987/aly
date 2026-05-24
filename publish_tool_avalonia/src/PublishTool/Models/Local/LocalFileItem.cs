using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.Models.Local;

public partial class LocalFileItem : ObservableObject
{
    [ObservableProperty]
    private string _fileName = string.Empty;

    [ObservableProperty]
    private string _absolutePath = string.Empty;

    [ObservableProperty]
    private string _relativePath = string.Empty;

    [ObservableProperty]
    private DateTime _lastModified = DateTime.MinValue;

    [ObservableProperty]
    private bool _isChecked;

    [ObservableProperty]
    private bool _isModified;

    [ObservableProperty]
    private long _fileSize;

    [ObservableProperty]
    private FileCompareStatus _compareStatus = FileCompareStatus.Unchanged;
}

public enum FileCompareStatus
{
    Unchanged,
    Modified,
    New,
    Deleted
}
