using Newtonsoft.Json;

namespace PublishTool.Models;

public class UpdateProjectDto
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("forceUpdate")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignoreFolders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignoreFiles")]
    public List<string> IgnoreFiles { get; set; } = new();
}
