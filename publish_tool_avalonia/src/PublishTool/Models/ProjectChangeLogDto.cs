using System.Collections.Generic;
using Newtonsoft.Json;

namespace PublishTool.Models;

public class ProjectChangeLogDto
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    [JsonProperty("time")]
    public string? Time { get; set; }

    [JsonProperty("created_at")]
    public string? CreatedAt { get; set; }

    [JsonProperty("is_deleted")]
    public bool IsDeleted { get; set; }
}
