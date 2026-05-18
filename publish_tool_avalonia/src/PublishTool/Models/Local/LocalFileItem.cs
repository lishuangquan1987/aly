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
    private DateTime _lastModified;

    [ObservableProperty]
    private bool _isChecked;

    [ObservableProperty]
    private bool _isModified;
}
