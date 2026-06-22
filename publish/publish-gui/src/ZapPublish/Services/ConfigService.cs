using Newtonsoft.Json;
using ZapPublish.Models.Local;
using Serilog;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;

namespace ZapPublish.Services;

public class ConfigService
{
    private static readonly string ConfigDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "ZapPublish");

    private static readonly string ConfigFile = Path.Combine(ConfigDir, "config.json");

    private readonly object _lock = new();

    public IReadOnlyList<ProjectConfig> LoadProjects()
    {
        lock (_lock)
        {
            return LoadProjectsUnlocked().ToList();
        }
    }

    public bool SaveProjects(List<ProjectConfig> projects)
    {
        lock (_lock)
        {
            return SaveProjectsUnlocked(projects);
        }
    }

    public bool AddProject(ProjectConfig project)
    {
        Log.Information("添加项目到配置: DisplayName={Name}, Path={Path}", project.DisplayName, project.ProjectPath);
        lock (_lock)
        {
            var projects = LoadProjectsUnlocked();
            projects.Add(project);
            return SaveProjectsUnlocked(projects);
        }
    }

    public bool RemoveProject(string projectPath)
    {
        Log.Information("从配置移除项目: Path={Path}", projectPath);
        lock (_lock)
        {
            var projects = LoadProjectsUnlocked();
            projects.RemoveAll(p => p.ProjectPath == projectPath);
            return SaveProjectsUnlocked(projects);
        }
    }

    public bool UpdateProject(ProjectConfig project)
    {
        Log.Information("更新项目配置: DisplayName={Name}, Path={Path}", project.DisplayName, project.ProjectPath);
        lock (_lock)
        {
            var projects = LoadProjectsUnlocked();
            // 按 ProjectPath 匹配（唯一标识），而非 DisplayName（可重名/可重命名）
            var index = projects.FindIndex(p => p.ProjectPath == project.ProjectPath);
            if (index >= 0)
            {
                projects[index] = project;
                return SaveProjectsUnlocked(projects);
            }
            else
            {
                Log.Warning("更新项目配置失败: 未找到 Path={Path}", project.ProjectPath);
                return false;
            }
        }
    }

    private List<ProjectConfig> LoadProjectsUnlocked()
    {
        try
        {
            Log.Debug("加载配置文件: {Path}", ConfigFile);
            if (!File.Exists(ConfigFile))
            {
                Log.Information("配置文件不存在，返回空列表: {Path}", ConfigFile);
                return new List<ProjectConfig>();
            }

            var json = File.ReadAllText(ConfigFile);
            var projects = JsonConvert.DeserializeObject<List<ProjectConfig>>(json) ?? new List<ProjectConfig>();
            Log.Information("加载配置成功: {Count} 个项目", projects.Count);
            return projects;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "加载配置失败: {Path}", ConfigFile);
            return new List<ProjectConfig>();
        }
    }

    private bool SaveProjectsUnlocked(List<ProjectConfig> projects)
    {
        try
        {
            Directory.CreateDirectory(ConfigDir);
            var json = JsonConvert.SerializeObject(projects, Formatting.Indented);
            File.WriteAllText(ConfigFile, json);
            Log.Information("保存配置成功: {Count} 个项目 -> {Path}", projects.Count, ConfigFile);
            return true;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "保存配置失败: {Path}", ConfigFile);
            return false;
        }
    }
}
