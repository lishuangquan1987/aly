using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

/// <summary>
/// 项目信息（来自服务端）
/// </summary>
public class ProjectInfo
{
    /// <summary>项目 ID</summary>
    [JsonProperty("id")]
    public int Id { get; set; }

    /// <summary>项目名称（英文标识）</summary>
    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    /// <summary>项目标题（中文显示）</summary>
    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    /// <summary>当前版本号</summary>
    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    /// <summary>是否强制更新</summary>
    [JsonProperty("force_update")]
    public bool ForceUpdate { get; set; }

    /// <summary>忽略的文件夹列表</summary>
    [JsonProperty("ignore_folders")]
    public List<string> IgnoreFolders { get; set; } = new();

    /// <summary>忽略的文件列表</summary>
    [JsonProperty("ignore_files")]
    public List<string> IgnoreFiles { get; set; } = new();

    /// <summary>创建时间</summary>
    [JsonProperty("created_at")]
    public string CreatedAt { get; set; } = string.Empty;

    /// <summary>是否已删除</summary>
    [JsonProperty("is_deleted")]
    public bool IsDeleted { get; set; }
}
