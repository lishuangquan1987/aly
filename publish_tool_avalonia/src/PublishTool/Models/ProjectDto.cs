using Newtonsoft.Json;

namespace PublishTool.Models;

public class ProjectDto
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("forceUpdate")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignoreFolders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignoreFiles")]
    public List<string> IgnoreFiles { get; set; } = new();

    [JsonProperty("createdAt")]
    public string? CreatedAt { get; set; }

    [JsonProperty("isDeleted")]
    public bool IsDeleted { get; set; }
}
