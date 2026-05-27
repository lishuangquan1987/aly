using Newtonsoft.Json;

namespace PublishTool.Models;

public class FileInfoDto
{
    [JsonProperty("fileAbsolutePath")]
    public string FileAbsolutePath { get; set; } = string.Empty;

    [JsonProperty("fileRelativePath")]
    public string FileRelativePath { get; set; } = string.Empty;

    [JsonProperty("lastUpdateTime")]
    public string? LastUpdateTime { get; set; }

    [JsonProperty("fileSize")]
    public long FileSize { get; set; }

    [JsonProperty("md5")]
    public string Md5 { get; set; } = string.Empty;

    [JsonProperty("sha256")]
    public string SHA256 { get; set; } = string.Empty;
}
