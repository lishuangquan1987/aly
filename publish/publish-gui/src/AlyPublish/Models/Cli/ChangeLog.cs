using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

public sealed class ChangeLog
{
    private string _version = string.Empty;
    private List<string> _logs = [];
    private string _time = string.Empty;

    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("project_id")]
    public int ProjectId { get; set; }

    [JsonProperty("version")]
    public string Version
    {
        get => _version;
        set => _version = value ?? string.Empty;
    }

    [JsonProperty("logs")]
    public List<string> Logs
    {
        get => _logs;
        set => _logs = value ?? [];
    }

    [JsonProperty("time")]
    public string Time
    {
        get => _time;
        set => _time = value ?? string.Empty;
    }
}
