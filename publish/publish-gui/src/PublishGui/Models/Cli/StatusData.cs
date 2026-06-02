using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

/// <summary>
/// 状态数据（来自 publish-cli status 命令）
/// </summary>
public class StatusData
{
    /// <summary>已暂存文件列表</summary>
    [JsonProperty("staged")]
    public List<FileStatusItem> Staged { get; set; } = new();

    /// <summary>未暂存文件列表</summary>
    [JsonProperty("unstaged")]
    public List<FileStatusItem> Unstaged { get; set; } = new();

    /// <summary>未变更文件列表</summary>
    [JsonProperty("unchanged")]
    public List<FileStatusItem> Unchanged { get; set; } = new();
}
