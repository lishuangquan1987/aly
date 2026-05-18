using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.Models.Local;

public partial class ProjectConfig : ObservableObject
{
    [ObservableProperty]
    private int _serverId;

    [ObservableProperty]
    private string _name = string.Empty;

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private string? _exePath;

    [ObservableProperty]
    private string _localPath = string.Empty;

    [ObservableProperty]
    private int _sortOrder;

    [ObservableProperty]
    private List<string> _ignoreFolders = new();

    [ObservableProperty]
    private List<string> _ignoreFiles = new();
}
