using Newtonsoft.Json;

namespace PublishTool.Models;

public class ProjectChangeLog
{
    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    [JsonProperty("time")]
    public string? Time { get; set; }

    [JsonProperty("createdAt")]
    public string? CreatedAt { get; set; }

    [JsonProperty("isDeleted")]
    public bool IsDeleted { get; set; }
}
