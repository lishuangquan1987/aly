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

    [JsonProperty("force_update")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignore_folders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignore_files")]
    public List<string> IgnoreFiles { get; set; } = new();

    [JsonProperty("created_at")]
    public string? CreatedAt { get; set; }

    [JsonProperty("is_deleted")]
    public bool IsDeleted { get; set; }
}
