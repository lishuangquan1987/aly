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
}