using System.Collections.Generic;
using Newtonsoft.Json;

namespace PublishTool.Models;

public class CreateProjectDto
{
    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("isForceUpdate")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignoreFolders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignoreFiles")]
    public List<string> IgnoreFiles { get; set; } = new();
}
