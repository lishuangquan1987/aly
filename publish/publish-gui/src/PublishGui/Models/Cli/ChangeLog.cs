using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

/// <summary>
/// 版本变更日志
/// </summary>
public class ChangeLog
{
    /// <summary>日志 ID</summary>
    [JsonProperty("id")]
    public int Id { get; set; }

    /// <summary>版本号</summary>
    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    /// <summary>变更说明列表</summary>
    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    /// <summary>发布时间</summary>
    [JsonProperty("time")]
    public string Time { get; set; } = string.Empty;

    /// <summary>创建时间</summary>
    [JsonProperty("created_at")]
    public string CreatedAt { get; set; } = string.Empty;
}
