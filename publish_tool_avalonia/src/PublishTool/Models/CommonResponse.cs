using Newtonsoft.Json;

namespace PublishTool.Models;

public class CommonResponse
{
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    [JsonProperty("errorMsg")]
    public string? ErrorMsg { get; set; }

    [JsonProperty("data")]
    public object? Data { get; set; }
}

public class CommonResponse<T> where T : class
{
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    [JsonProperty("errorMsg")]
    public string? ErrorMsg { get; set; }

    [JsonProperty("data")]
    public T? Data { get; set; }
}
