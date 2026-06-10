using CommunityToolkit.Mvvm.ComponentModel;
using Newtonsoft.Json;

namespace ZapPublish.Models.Local;

public partial class ProjectConfig : ObservableObject
{
    [ObservableProperty]
    [property: JsonProperty("ServerUrl")]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    [property: JsonProperty("ProjectName")]
    private string _projectName = string.Empty;

    [ObservableProperty]
    [property: JsonProperty("ProjectPath")]
    private string _projectPath = string.Empty;
}