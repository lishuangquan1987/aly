using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

public class StatusData
{
    [JsonProperty("staged")]
    public List<FileStatusItem> Staged { get; set; } = new();

    [JsonProperty("unstaged")]
    public List<FileStatusItem> Unstaged { get; set; } = new();

    [JsonProperty("unchanged")]
    public List<FileStatusItem> Unchanged { get; set; } = new();
}