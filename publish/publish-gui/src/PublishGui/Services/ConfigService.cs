using Newtonsoft.Json;
using PublishGui.Models.Local;
using Serilog;
using System;
using System.IO;

namespace PublishGui.Services;

public class ConfigService
{
    private static readonly string ConfigDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "PublishGui");

    private static readonly string ConfigFile = Path.Combine(ConfigDir, "config.json");

    public List<ProjectConfig> LoadProjects()
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

    public void SaveProjects(List<ProjectConfig> projects)
    {
        try
        {
            Directory.CreateDirectory(ConfigDir);
            var json = JsonConvert.SerializeObject(projects, Formatting.Indented);
            File.WriteAllText(ConfigFile, json);
            Log.Information("保存配置成功: {Count} 个项目 -> {Path}", projects.Count, ConfigFile);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "保存配置失败: {Path}", ConfigFile);
        }
    }

    public void AddProject(ProjectConfig project)
    {
        Log.Information("添加项目到配置: Name={Name}", project.ProjectName);
        var projects = LoadProjects();
        projects.Add(project);
        SaveProjects(projects);
    }

    public void RemoveProject(string projectName)
    {
        Log.Information("从配置移除项目: Name={Name}", projectName);
        var projects = LoadProjects();
        projects.RemoveAll(p => p.ProjectName == projectName);
        SaveProjects(projects);
    }

    public void UpdateProject(ProjectConfig project)
    {
        Log.Information("更新项目配置: Name={Name}", project.ProjectName);
        var projects = LoadProjects();
        var index = projects.FindIndex(p => p.ProjectName == project.ProjectName);
        if (index >= 0)
        {
            projects[index] = project;
            SaveProjects(projects);
        }
        else
        {
            Log.Warning("更新项目配置失败: 未找到 Name={Name}", project.ProjectName);
        }
    }
}
