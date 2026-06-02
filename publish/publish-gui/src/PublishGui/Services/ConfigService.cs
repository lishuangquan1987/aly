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
            if (!File.Exists(ConfigFile))
            {
                return new List<ProjectConfig>();
            }

            var json = File.ReadAllText(ConfigFile);
            return JsonConvert.DeserializeObject<List<ProjectConfig>>(json) ?? new List<ProjectConfig>();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "Failed to load config");
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
        }
        catch (Exception ex)
        {
            Log.Error(ex, "Failed to save config");
        }
    }

    public void AddProject(ProjectConfig project)
    {
        var projects = LoadProjects();
        projects.Add(project);
        SaveProjects(projects);
    }

    public void RemoveProject(string projectName)
    {
        var projects = LoadProjects();
        projects.RemoveAll(p => p.ProjectName == projectName);
        SaveProjects(projects);
    }

    public void UpdateProject(ProjectConfig project)
    {
        var projects = LoadProjects();
        var index = projects.FindIndex(p => p.ProjectName == project.ProjectName);
        if (index >= 0)
        {
            projects[index] = project;
            SaveProjects(projects);
        }
    }
}
