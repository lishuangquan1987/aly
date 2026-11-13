using Newtonsoft.Json;

namespace AlyPublish.Models.Local;

/// <summary>The contents of <c>UpdateFolder/client.json</c>, read by aly-client.</summary>
public sealed class UpdateClientConfig
{
    [JsonProperty("main_exe_relative_path")]
    public string MainExeRelativePath { get; set; } = string.Empty;

    [JsonProperty("must_close_process_name")]
    public List<string> MustCloseProcessName { get; set; } = [];
}
