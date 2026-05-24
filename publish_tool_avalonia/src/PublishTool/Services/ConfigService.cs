using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using PublishTool.Models.Local;
using Serilog;

namespace PublishTool.Services;

public class ConfigService
{
    private readonly string _configPath;
    private List<ProjectConfig> _projects = new();

    public ConfigService()
    {
        var appData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        var dir = Path.Combine(appData, "PublishTool");
        Directory.CreateDirectory(dir);
        _configPath = Path.Combine(dir, "publish_tool_config.json");
        Log.Information("配置文件路径: {Path}", _configPath);
        Load();
    }

    public IReadOnlyList<ProjectConfig> Projects => _projects.AsReadOnly();

    public event Action? ProjectsChanged;

    public void Load()
    {
        if (!File.Exists(_configPath))
        {
            Log.Information("配置文件不存在，将创建新文件");
            return;
        }

        try
        {
            var json = File.ReadAllText(_configPath);
            if (string.IsNullOrWhiteSpace(json))
            {
                Log.Warning("配置文件为空");
                _projects = new List<ProjectConfig>();
                return;
            }

            var token = JToken.Parse(json);
            if (token is JArray arr)
            {
                _projects = arr.ToObject<List<ProjectConfig>>() ?? new List<ProjectConfig>();
                Log.Information("加载了 {Count} 个项目配置", _projects.Count);
            }
            else
            {
                Log.Warning("配置文件格式错误，期望 JSON 数组");
                _projects = new List<ProjectConfig>();
            }
        }
        catch (JsonException ex)
        {
            Log.Error(ex, "配置文件解析失败，备份旧文件并创建新配置");
            BackupAndReset();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "加载配置文件时发生未知错误");
            BackupAndReset();
        }
    }

    private void BackupAndReset()
    {
        try
        {
            if (File.Exists(_configPath))
            {
                var backupPath = _configPath + ".backup." + DateTime.Now.ToString("yyyyMMddHHmmss");
                File.Copy(_configPath, backupPath);
                Log.Information("已备份旧配置文件到: {Path}", backupPath);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "备份配置文件失败");
        }
        _projects = new List<ProjectConfig>();
    }

    public void Save()
    {
        try
        {
            var json = JsonConvert.SerializeObject(_projects, Formatting.Indented);
            File.WriteAllText(_configPath, json);
            Log.Debug("已保存 {Count} 个项目配置", _projects.Count);
            ProjectsChanged?.Invoke();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "保存配置文件失败");
        }
    }

    public async Task SaveAsync()
    {
        try
        {
            var json = JsonConvert.SerializeObject(_projects, Formatting.Indented);
            await File.WriteAllTextAsync(_configPath, json);
            Log.Debug("已异步保存 {Count} 个项目配置", _projects.Count);
            ProjectsChanged?.Invoke();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "异步保存配置文件失败");
        }
    }

    public void AddProject(ProjectConfig config)
    {
        config.SortOrder = _projects.Count;
        _projects.Add(config);
        Save();
        Log.Information("添加项目: {Title} ({Name})", config.Title, config.Name);
    }

    public void RemoveProject(ProjectConfig config)
    {
        _projects.Remove(config);
        ReorderProjects();
        Save();
        Log.Information("删除项目: {Title} ({Name})", config.Title, config.Name);
    }

    public void UpdateProject(ProjectConfig config)
    {
        var index = _projects.FindIndex(p => p.ServerId == config.ServerId
            && p.ServerUrl == config.ServerUrl);
        if (index >= 0)
        {
            _projects[index] = config;
            Save();
            Log.Information("更新项目: {Title}", config.Title);
        }
        else
        {
            Log.Warning("未找到要更新的项目: {ServerId} @ {Url}", config.ServerId, config.ServerUrl);
        }
    }

    public void MoveUp(int index)
    {
        if (index > 0)
        {
            (_projects[index], _projects[index - 1]) = (_projects[index - 1], _projects[index]);
            ReorderProjects();
            Save();
        }
    }

    public void MoveDown(int index)
    {
        if (index < _projects.Count - 1)
        {
            (_projects[index], _projects[index + 1]) = (_projects[index + 1], _projects[index]);
            ReorderProjects();
            Save();
        }
    }

    private void ReorderProjects()
    {
        for (var i = 0; i < _projects.Count; i++)
        {
            _projects[i].SortOrder = i;
        }
    }
}
