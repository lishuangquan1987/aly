namespace ZapPublish.Models.Local;

/// <summary>
/// .updator/shared.json 的配置文件结构（当前仅读取 server_url 和 project_name，其余字段为完整性保留）
/// </summary>
public class SharedConfig
{
    public string server_url { get; set; } = string.Empty;
    public string project_name { get; set; } = string.Empty;
    public List<string> ignore_folders { get; set; } = new();
    public List<string> ignore_files { get; set; } = new();
}
