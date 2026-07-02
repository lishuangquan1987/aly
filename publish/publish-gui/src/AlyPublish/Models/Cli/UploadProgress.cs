using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

/// <summary>
/// CLI push 命令的过程进度输出（每行一个 JSON）。
/// 对应 publish-cli 的 models.UploadProgress 结构。
/// </summary>
public class UploadProgress
{
    [JsonProperty("index")]
    public int Index { get; set; }

    [JsonProperty("total")]
    public int Total { get; set; }

    [JsonProperty("file")]
    public string File { get; set; } = string.Empty;

    /// <summary>START / DONE / FAIL</summary>
    [JsonProperty("status")]
    public string Status { get; set; } = string.Empty;

    [JsonProperty("file_size")]
    public long FileSize { get; set; }

    [JsonProperty("error")]
    public string Error { get; set; } = string.Empty;
}
