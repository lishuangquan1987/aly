using Newtonsoft.Json;

namespace AlyPublish.Models.Local;

/// <summary>
/// .updator/shared.json 的配置文件结构
/// </summary>
public class SharedConfig
{
    [JsonProperty("server_url")]
    public string ServerUrl { get; set; } = string.Empty;

    [JsonProperty("project_name")]
    public string ProjectName { get; set; } = string.Empty;

    [JsonProperty("ignore_folders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignore_files")]
    public List<string> IgnoreFiles { get; set; } = new();
}
