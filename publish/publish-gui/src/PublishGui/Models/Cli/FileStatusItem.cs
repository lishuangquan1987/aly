using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

/// <summary>
/// 文件状态项（来自 publish-cli status 命令）
/// </summary>
public class FileStatusItem
{
    /// <summary>相对路径</summary>
    [JsonProperty("relativePath")]
    public string RelativePath { get; set; } = string.Empty;

    /// <summary>状态（new/modified/deleted/unchanged）</summary>
    [JsonProperty("status")]
    public string Status { get; set; } = string.Empty;

    /// <summary>本地 MD5</summary>
    [JsonProperty("localMd5")]
    public string LocalMd5 { get; set; } = string.Empty;

    /// <summary>本地文件大小（字节）</summary>
    [JsonProperty("localSize")]
    public long LocalSize { get; set; }

    /// <summary>服务端 MD5</summary>
    [JsonProperty("serverMd5")]
    public string ServerMd5 { get; set; } = string.Empty;

    /// <summary>服务端文件大小（字节）</summary>
    [JsonProperty("serverSize")]
    public long ServerSize { get; set; }
}
