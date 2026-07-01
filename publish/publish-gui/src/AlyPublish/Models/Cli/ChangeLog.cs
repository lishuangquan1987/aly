using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

public class ChangeLog
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("project_id")]
    public int ProjectId { get; set; }

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    [JsonProperty("time")]
    public string Time { get; set; } = string.Empty;
}