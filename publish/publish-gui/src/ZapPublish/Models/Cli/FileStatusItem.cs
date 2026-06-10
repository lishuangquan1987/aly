using Newtonsoft.Json;

namespace ZapPublish.Models.Cli;

public class FileStatusItem
{
    [JsonProperty("relativePath")]
    public string RelativePath { get; set; } = string.Empty;

    [JsonProperty("status")]
    public string Status { get; set; } = string.Empty;

    [JsonProperty("localMd5")]
    public string LocalMd5 { get; set; } = string.Empty;

    [JsonProperty("localSize")]
    public long LocalSize { get; set; }

    [JsonProperty("serverMd5")]
    public string ServerMd5 { get; set; } = string.Empty;

    [JsonProperty("serverSize")]
    public long ServerSize { get; set; }
}