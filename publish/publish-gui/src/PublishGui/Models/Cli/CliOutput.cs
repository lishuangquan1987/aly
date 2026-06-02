using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

/// <summary>
/// CLI 命令输出的通用包装类
/// </summary>
/// <typeparam name="T">数据类型</typeparam>
public class CliOutput<T>
{
    /// <summary>是否成功</summary>
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    /// <summary>错误信息</summary>
    [JsonProperty("errorMsg")]
    public string ErrorMsg { get; set; } = string.Empty;

    /// <summary>响应数据</summary>
    [JsonProperty("data")]
    public T? Data { get; set; }
}
