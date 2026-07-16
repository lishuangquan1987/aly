using Newtonsoft.Json;
using AlyPublish.Models.Local;
using Serilog;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;

namespace AlyPublish.Services;

public class ConfigService
{
    private static readonly string ConfigDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "AlyPublish");

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

    // ── UpdateFolder ─────────────────────────────────────

    /// <summary>
    /// 在 projectPath 的父目录下创建 UpdateFolder，复制 aly-client.exe 并写入 client.json。
    /// </summary>
    public void CreateUpdateFolder(string projectPath, string mainExePath, string closeProcessName)
    {
        if (string.IsNullOrWhiteSpace(projectPath))
        {
            Log.Warning("CreateUpdateFolder: projectPath 为空");
            return;
        }

        var parentDir = Path.GetDirectoryName(Path.GetFullPath(projectPath));
        if (parentDir == null)
        {
            Log.Warning("CreateUpdateFolder: 无法获取 projectPath 的父目录: {Path}", projectPath);
            return;
        }

        var updateFolder = Path.Combine(parentDir, "UpdateFolder");
        Directory.CreateDirectory(updateFolder);
        Log.Information("创建 UpdateFolder: {Path}", updateFolder);

        // 复制 aly-client.exe
        var clientExeName = OperatingSystem.IsWindows() ? "aly-client.exe" : "aly-client";
        var clientExeSource = FindAlyClientExe(clientExeName);
        if (clientExeSource != null)
        {
            var destPath = Path.Combine(updateFolder, clientExeName);
            try
            {
                File.Copy(clientExeSource, destPath, overwrite: true);
                Log.Information("复制 aly-client.exe: {Source} -> {Dest}", clientExeSource, destPath);
            }
            catch (Exception ex)
            {
                Log.Error(ex, "复制 aly-client.exe 失败: {Source} -> {Dest}", clientExeSource, destPath);
            }
        }
        else
        {
            Log.Warning("未找到 aly-client.exe，跳过复制。用户可手动放置到 {Path}", updateFolder);
        }

        // 写入 client.json
        var relativePath = GetRelativePath(updateFolder, Path.GetFullPath(mainExePath));
        Log.Information("计算 main_exe_relative_path: UpdateFolder={Folder}, MainExe={Exe}, Relative={Rel}",
            updateFolder, mainExePath, relativePath);

        var config = new UpdateClientConfig
        {
            MainExeRelativePath = relativePath,
            MustCloseProcessName = new List<string> { closeProcessName }
        };

        var clientJsonPath = Path.Combine(updateFolder, "client.json");
        try
        {
            var json = JsonConvert.SerializeObject(config, Formatting.Indented);
            File.WriteAllText(clientJsonPath, json);
            Log.Information("写入 client.json: {Path}", clientJsonPath);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "写入 client.json 失败: {Path}", clientJsonPath);
        }
    }

    /// <summary>查找 aly-client.exe 源文件</summary>
    private static string? FindAlyClientExe(string exeName)
    {
        var exeDir = AppDomain.CurrentDomain.BaseDirectory;
        Log.Debug("查找 aly-client.exe: BaseDirectory={Dir}", exeDir);

        // 1. 同目录
        var same = Path.Combine(exeDir, exeName);
        if (File.Exists(same))
        {
            Log.Debug("找到 aly-client.exe (同目录): {Path}", same);
            return same;
        }

        // 2. 开发路径：从 bin/Debug/net8.0/ 往上 7 层到项目根，再进入 client/aly-client/
        var dev = Path.GetFullPath(Path.Combine(exeDir, "..", "..", "..", "..", "..", "..", "..",
            "client", "aly-client", exeName));
        if (File.Exists(dev))
        {
            Log.Debug("找到 aly-client.exe (开发路径): {Path}", dev);
            return dev;
        }

        Log.Warning("未找到 aly-client.exe");
        return null;
    }

    /// <summary>计算从 fromDir 到 targetPath 的相对路径</summary>
    private static string GetRelativePath(string fromDir, string targetPath)
    {
        // 确保目录以分隔符结尾
        if (!fromDir.EndsWith(Path.DirectorySeparatorChar.ToString()))
            fromDir += Path.DirectorySeparatorChar;

        var fromUri = new Uri(fromDir);
        var toUri = new Uri(targetPath);
        var relativeUri = fromUri.MakeRelativeUri(toUri);
        var relativePath = Uri.UnescapeDataString(relativeUri.ToString())
            .Replace('/', Path.DirectorySeparatorChar);
        return relativePath;
    }
}
