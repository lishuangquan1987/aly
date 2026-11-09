using Newtonsoft.Json;

namespace AlyPublish.Models.Cli;

internal sealed class CliOutput<T>
{
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    [JsonProperty("errorMsg")]
    public string ErrorMsg { get; set; } = string.Empty;

    [JsonProperty("data")]
    public T? Data { get; set; }
}
