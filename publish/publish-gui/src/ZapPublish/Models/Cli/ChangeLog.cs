using Newtonsoft.Json;

namespace ZapPublish.Models.Cli;

public class ChangeLog
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    [JsonProperty("time")]
    public string Time { get; set; } = string.Empty;
}