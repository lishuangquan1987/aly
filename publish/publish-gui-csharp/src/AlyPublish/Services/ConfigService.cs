using AlyPublish.Constants;
using AlyPublish.Helpers;
using AlyPublish.Models.Local;
using Newtonsoft.Json;
using Serilog;

namespace AlyPublish.Services;

/// <summary>Stores the GUI's project bookmarks (display name and local path only).</summary>
public sealed class ConfigService
{
    private readonly string _configDirectory;
    private readonly string _configFile;

    public ConfigService()
        : this(Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            AppConstants.ConfigDirectoryName))
    {
    }

    internal ConfigService(string configDirectory)
    {
        _configDirectory = configDirectory;
        _configFile = Path.Combine(configDirectory, AppConstants.ConfigFileName);
    }

    public IReadOnlyList<ProjectConfig> LoadProjects()
    {
        try
        {
            if (!File.Exists(_configFile))
            {
                return [];
            }

            var json = File.ReadAllText(_configFile);
            var projects = JsonConvert.DeserializeObject<List<ProjectConfig>>(json);
            foreach (var project in projects ?? [])
            {
                project.DisplayName ??= string.Empty;
                project.ProjectPath ??= string.Empty;
                project.MainExePath ??= string.Empty;
            }

            Log.Information("加载项目配置: {Count} 个项目", projects?.Count ?? 0);
            return projects ?? [];
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or JsonException)
        {
            Log.Error(ex, "加载项目配置失败: {Path}", _configFile);
            return [];
        }
    }

    public void SaveProjects(IEnumerable<ProjectConfig> projects)
    {
        var projectList = projects.ToList();
        Directory.CreateDirectory(_configDirectory);
        var json = JsonConvert.SerializeObject(projectList, Formatting.Indented);
        FileWriteHelper.WriteAllTextAtomic(_configFile, json);
        Log.Information("保存项目配置: {Count} 个项目", projectList.Count);
    }
}
