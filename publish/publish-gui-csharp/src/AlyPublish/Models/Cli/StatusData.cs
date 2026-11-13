using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

public sealed class StatusData
{
    private List<FileStatusItem> _staged = [];
    private List<FileStatusItem> _unstaged = [];
    private List<FileStatusItem> _unchanged = [];

    [JsonProperty("staged")]
    public List<FileStatusItem> Staged
    {
        get => _staged;
        set => _staged = value ?? [];
    }

    [JsonProperty("unstaged")]
    public List<FileStatusItem> Unstaged
    {
        get => _unstaged;
        set => _unstaged = value ?? [];
    }

    [JsonProperty("unchanged")]
    public List<FileStatusItem> Unchanged
    {
        get => _unchanged;
        set => _unchanged = value ?? [];
    }
}
