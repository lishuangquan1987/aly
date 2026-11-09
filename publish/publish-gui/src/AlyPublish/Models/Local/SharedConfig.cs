using Newtonsoft.Json;

namespace AlyPublish.Models.Local;

/// <summary>The shape of <c>.updator/shared.json</c>, shared by the CLI and the updater.</summary>
public sealed class SharedConfig
{
    private string _serverUrl = string.Empty;
    private string _projectName = string.Empty;
    private List<string> _ignoreFolders = [];
    private List<string> _ignoreFiles = [];
    private List<string> _unCopyFolders = [];
    private List<string> _unCopyFiles = [];

    [JsonProperty("server_url")]
    public string ServerUrl
    {
        get => _serverUrl;
        set => _serverUrl = value ?? string.Empty;
    }

    [JsonProperty("project_name")]
    public string ProjectName
    {
        get => _projectName;
        set => _projectName = value ?? string.Empty;
    }

    [JsonProperty("ignore_folders")]
    public List<string> IgnoreFolders
    {
        get => _ignoreFolders;
        set => _ignoreFolders = value ?? [];
    }

    [JsonProperty("ignore_files")]
    public List<string> IgnoreFiles
    {
        get => _ignoreFiles;
        set => _ignoreFiles = value ?? [];
    }

    [JsonProperty("un_copy_folders")]
    public List<string> UnCopyFolders
    {
        get => _unCopyFolders;
        set => _unCopyFolders = value ?? [];
    }

    [JsonProperty("un_copy_files")]
    public List<string> UnCopyFiles
    {
        get => _unCopyFiles;
        set => _unCopyFiles = value ?? [];
    }
}
