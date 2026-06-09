using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

public class ProjectInfo
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("force_update")]
    public bool IsForceUpdate { get; set; }
}