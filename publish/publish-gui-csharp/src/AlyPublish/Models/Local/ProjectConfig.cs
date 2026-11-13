using CommunityToolkit.Mvvm.ComponentModel;
using Newtonsoft.Json;

namespace AlyPublish.Models.Local;

/// <summary>
/// A GUI bookmark: display name and local path only. The real project settings live in
/// <c>.updator/shared.json</c> and are read through the CLI.
/// </summary>
public sealed partial class ProjectConfig : ObservableObject
{
    [ObservableProperty]
    [property: JsonProperty("DisplayName")]
    private string _displayName = string.Empty;

    [ObservableProperty]
    [property: JsonProperty("ProjectPath")]
    private string _projectPath = string.Empty;

    /// <summary>Executable launched by the "启动程序" button; relative to <see cref="ProjectPath"/> when possible.</summary>
    [ObservableProperty]
    [property: JsonProperty("MainExePath")]
    private string _mainExePath = string.Empty;
}
