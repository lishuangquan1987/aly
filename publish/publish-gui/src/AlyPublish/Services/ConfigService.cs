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
            WriteAllTextAtomic(ConfigFile, json);
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
    /// 在 projectPath（主程序目录，内含 .updator/）的**同级**创建 UpdateFolder，
    /// 复制 aly-client.exe / aly-client-ui.exe 并写入 client.json。
    ///
    /// 目录布局必须与 aly-client 的口径一致（README / DEVELOMENT_PLAN §3.0）：
    /// <c>PackageFolder/{ApplicationFolder, UpdateFolder}</c> —— 更新器目录是主程序目录的
    /// **兄弟目录**（"..\UpdateFolder\"），不是主程序目录的子目录。放错层级有两个硬后果：
    ///   ① aly-client 由 client.json 的 main_exe_relative_path 推导 MainFolder，
    ///      UpdateFolder 在主程序目录内会把整个主程序目录当更新目标；
    ///   ② apply / rollback 要给主程序目录改名，更新器自身（与 .updator/）会被一起搬走。
    /// </summary>
    public void CreateUpdateFolder(string projectPath, string mainExePath, string closeProcessName)
    {
        if (string.IsNullOrWhiteSpace(projectPath))
        {
            Log.Warning("CreateUpdateFolder: projectPath 为空");
            return;
        }

        // 归一化后再取上级目录：Path.GetDirectoryName 对**带结尾分隔符**的路径会回退成
        // "目录自身"。浏览对话框返回的路径正是带结尾 '\' 的形状（实测
        // "…\net8.0-windows\"），历史 bug：UpdateFolder 因此被创建在主程序目录内部，
        // 而不是它的兄弟目录。
        var projectDir = NormalizeDirectoryPath(projectPath);
        var parentDir = Path.GetDirectoryName(projectDir);
        if (parentDir == null)
        {
            Log.Warning("CreateUpdateFolder: 无法获取主程序目录的上级目录: {Path}", projectPath);
            return;
        }

        var updateFolder = Path.Combine(parentDir, "UpdateFolder");
        Directory.CreateDirectory(updateFolder);
        Log.Information("创建 UpdateFolder: {Path}（主程序目录={Dir}）", updateFolder, projectDir);

        // 更新器 + 图形前端：两者都必须与 client.json 同在 UpdateFolder
        CopyClientExecutable(updateFolder, projectDir, "aly-client");
        CopyClientExecutable(updateFolder, projectDir, "aly-client-ui");

        // 写入 client.json
        var mainExeFullPath = Path.GetFullPath(mainExePath);
        var relativePath = GetRelativePath(updateFolder, mainExeFullPath);
        // 关进程名缺省时从主程序名推导：更新前必须把主程序关掉，否则 apply 无法替换主目录
        var processName = string.IsNullOrWhiteSpace(closeProcessName)
            ? Path.GetFileNameWithoutExtension(mainExeFullPath)
            : closeProcessName;
        Log.Information("计算 main_exe_relative_path: UpdateFolder={Folder}, MainExe={Exe}, Relative={Rel}",
            updateFolder, mainExeFullPath, relativePath);

        var config = new UpdateClientConfig
        {
            MainExeRelativePath = relativePath,
            MustCloseProcessName = new List<string> { processName }
        };

        var clientJsonPath = Path.Combine(updateFolder, "client.json");
        try
        {
            var json = JsonConvert.SerializeObject(config, Formatting.Indented);
            WriteAllTextAtomic(clientJsonPath, json);
            Log.Information("写入 client.json: {Path}", clientJsonPath);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "写入 client.json 失败: {Path}", clientJsonPath);
        }
    }

    /// <summary>
    /// 原子写文件：先写 .tmp 再原子替换目标，崩溃/断电不会留下半截 JSON。
    /// 旧实现 File.WriteAllText 直接截断目标，写入中途进程退出会损坏配置。
    ///
    /// **目标不存在时必须改名（File.Move），不能 File.Replace**：System.IO 的 ReplaceFile
    /// 要求目标文件已存在，否则抛 FileNotFoundException（实测："写入 client.json 失败"
    /// + UpdateFolder 里只剩 client.json.tmp）。这正是"初始化后 client.json 被命名为
    /// client.json.tmp"的根因——不是文件名拼错，而是首次写必然失败、.tmp 成了唯一残留。
    /// </summary>
    private static void WriteAllTextAtomic(string path, string content)
    {
        var tmpPath = path + ".tmp";
        File.WriteAllText(tmpPath, content);

        if (!File.Exists(path))
        {
            // 首次写：目标尚不存在，ReplaceFile 必失败 → 直接改名
            File.Move(tmpPath, path);
            return;
        }

        try
        {
            File.Replace(tmpPath, path, null);
        }
        catch (PlatformNotSupportedException)
        {
            // 非 Windows 平台（netstandard2.0/Mono 等）不支持原子替换：删除目标 + 改名
            // 回退（与 SDK 自更新 UpdateSelf 同口径）
            File.Delete(path);
            File.Move(tmpPath, path);
        }
        catch (Exception)
        {
            // 替换失败（目标被别的进程占用等）：不留 .tmp 残迹，失败原因交给调用方上报
            try { File.Delete(tmpPath); } catch { }
            throw;
        }
    }

    /// <summary>归一化路径：绝对化 + 去掉结尾分隔符（上级目录推导与相对路径基准都要求无尾分隔符）</summary>
    private static string NormalizeDirectoryPath(string path)
        => Path.GetFullPath(path).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);

    /// <summary>复制更新器可执行文件到 UpdateFolder（源找不到时留痕，不中断 client.json 的写入）</summary>
    private static void CopyClientExecutable(string updateFolder, string projectDir, string baseName)
    {
        var exeName = OperatingSystem.IsWindows() ? baseName + ".exe" : baseName;
        var source = FindClientExecutable(projectDir, exeName);
        if (source == null)
        {
            Log.Warning("未找到 {Exe}，跳过复制。用户可手动放置到 {Path}", exeName, updateFolder);
            return;
        }

        var destPath = Path.Combine(updateFolder, exeName);
        try
        {
            File.Copy(source, destPath, overwrite: true);
            Log.Information("复制 {Exe}: {Source} -> {Dest}", exeName, source, destPath);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "复制 {Exe} 失败: {Source} -> {Dest}", exeName, source, destPath);
        }
    }

    /// <summary>
    /// 查找更新器可执行文件，按可靠性依次回退：
    ///   ① GUI 同目录（GUI 与更新器一起分发，工具自带的副本）；
    ///   ② 主程序目录（发布包携带的副本，与主程序同版本分发）；
    ///   ③ 仓库开发路径 client/aly-client/（开发态）。
    /// 都找不到时返回 null：调用方只留痕，不清空 UpdateFolder 里已有的副本。
    /// </summary>
    private static string? FindClientExecutable(string projectDir, string exeName)
    {
        var exeDir = AppDomain.CurrentDomain.BaseDirectory;
        Log.Debug("查找 {Exe}: BaseDirectory={Dir}", exeName, exeDir);

        // 1. GUI 同目录（GUI 与更新器一起分发时）
        var same = Path.Combine(exeDir, exeName);
        if (File.Exists(same))
        {
            Log.Debug("找到 {Exe} (GUI 同目录): {Path}", exeName, same);
            return same;
        }

        // 2. 主程序目录：发布包携带的副本（README：随发布包下发，作为自更新的比对源）
        if (!string.IsNullOrWhiteSpace(projectDir))
        {
            var shipped = Path.Combine(projectDir, exeName);
            if (File.Exists(shipped))
            {
                Log.Debug("找到 {Exe} (主程序目录): {Path}", exeName, shipped);
                return shipped;
            }
        }

        // 3. 开发路径：从 bin/Debug/net8.0/ 往上 7 层到仓库根，再进入 client/aly-client/
        var dev = Path.GetFullPath(Path.Combine(exeDir, "..", "..", "..", "..", "..", "..", "..",
            "client", "aly-client", exeName));
        if (File.Exists(dev))
        {
            Log.Debug("找到 {Exe} (开发路径): {Path}", exeName, dev);
            return dev;
        }

        Log.Warning("未找到 {Exe}（UpdateFolder 内已有副本将保留）", exeName);
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
