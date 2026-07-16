using System.Collections.Generic;
using Newtonsoft.Json;

namespace AlyPublish.Models.Local;

/// <summary>
/// UpdateFolder/client.json 模型 — aly-client.exe 读取此文件获取更新目标信息。
/// </summary>
public class UpdateClientConfig
{
    /// <summary>主程序相对于 UpdateFolder 的相对路径</summary>
    [JsonProperty("main_exe_relative_path")]
    public string MainExeRelativePath { get; set; } = string.Empty;

    /// <summary>应用更新前必须关闭的进程名称列表</summary>
    [JsonProperty("must_close_process_name")]
    public List<string> MustCloseProcessName { get; set; } = new();
}
