using AlyPublish.Helpers;
using AlyPublish.Localization;
using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

/// <summary>Per-file upload progress emitted by <c>aly-publish push --json</c>.</summary>
public sealed class UploadProgress
{
    private string _file = string.Empty;
    private string _status = string.Empty;
    private string _error = string.Empty;

    [JsonProperty("index")]
    public int Index { get; set; }

    [JsonProperty("total")]
    public int Total { get; set; }

    [JsonProperty("file")]
    public string File
    {
        get => _file;
        set => _file = value ?? string.Empty;
    }

    /// <summary>START / DONE / FAIL</summary>
    [JsonProperty("status")]
    public string Status
    {
        get => _status;
        set => _status = value ?? string.Empty;
    }

    [JsonProperty("file_size")]
    public long FileSize { get; set; }

    [JsonProperty("error")]
    public string Error
    {
        get => _error;
        set => _error = value ?? string.Empty;
    }

    [JsonIgnore]
    public bool IsRunning => Status == "START";

    [JsonIgnore]
    public bool IsDone => Status == "DONE";

    [JsonIgnore]
    public bool IsFailed => Status == "FAIL";

    [JsonIgnore]
    public string FileSizeText => FileSizeHelper.Format(FileSize);

    [JsonIgnore]
    // 审查 publish 轻微⑤：文案不再在这里硬编码中文，统一走 Lingua（中英双语）。
    public string StatusText => LanguageManager.UploadStatusText(Status, Error);
}
