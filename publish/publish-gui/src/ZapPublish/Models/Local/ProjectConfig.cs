using CommunityToolkit.Mvvm.ComponentModel;
using Newtonsoft.Json;

namespace ZapPublish.Models.Local;

/// <summary>
/// GUI 本地项目配置 —— 类比 SourceTree 的书签。
/// 只存储显示名称和项目路径，server_url / project_name 等真实配置从项目的 .updator/ 目录读取。
/// </summary>
public partial class ProjectConfig : ObservableObject
{
    /// <summary>本地显示名称，仅供 GUI 展示，用户可重命名</summary>
    [ObservableProperty]
    [property: JsonProperty("DisplayName")]
    private string _displayName = string.Empty;

    /// <summary>项目本地路径（构建产物目录，内含 .updator/）</summary>
    [ObservableProperty]
    [property: JsonProperty("ProjectPath")]
    private string _projectPath = string.Empty;
}