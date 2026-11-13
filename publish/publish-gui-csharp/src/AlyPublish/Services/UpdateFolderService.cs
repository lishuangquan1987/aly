using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Local;
using Newtonsoft.Json;
using Serilog;

namespace AlyPublish.Services;

/// <summary>
/// Creates the updater package that sits next to the application folder:
/// <c>PackageFolder/{ApplicationFolder, UpdateFolder}</c>. The updater executables are taken from the
/// packaged payload (<c>client/{rid}/</c>) that the AlyPublish GUI was installed with, so the platform is
/// decided by the package instead of by the caller.
/// </summary>
public static class UpdateFolderService
{
    /// <returns>A warning for missing updater executables, or <see langword="null"/> when complete.</returns>
    public static string? Create(string projectPath, string mainExePath, string closeProcessName)
        => Create(projectPath, mainExePath, closeProcessName, AppDomain.CurrentDomain.BaseDirectory);

    /// <summary>Same as <see cref="Create(string, string, string)"/> with an explicit package root.</summary>
    public static string? Create(
        string projectPath,
        string mainExePath,
        string closeProcessName,
        string packageDirectory)
    {
        var projectDirectory = Path.TrimEndingDirectorySeparator(Path.GetFullPath(projectPath));
        var parentDirectory = Path.GetDirectoryName(projectDirectory)
            ?? throw new InvalidOperationException(
                LanguageManager.T(LanguageManager.Instance.Message_UpdateFolder_ParentMissing));
        var updateFolder = Path.Combine(parentDirectory, "UpdateFolder");
        Directory.CreateDirectory(updateFolder);

        var warning = CopyUpdaters(updateFolder, projectDirectory, packageDirectory);

        var mainExeFullPath = Path.GetFullPath(mainExePath);
        var relativePath = Path.GetRelativePath(updateFolder, mainExeFullPath);
        if (Path.IsPathRooted(relativePath))
        {
            throw new InvalidOperationException(
                LanguageManager.T(LanguageManager.Instance.Message_UpdateFolder_SameDisk));
        }

        var processName = string.IsNullOrWhiteSpace(closeProcessName)
            ? Path.GetFileNameWithoutExtension(mainExeFullPath)
            : closeProcessName.Trim();

        var config = new UpdateClientConfig
        {
            MainExeRelativePath = relativePath,
            MustCloseProcessName = [processName]
        };
        var clientJsonPath = Path.Combine(updateFolder, "client.json");
        FileWriteHelper.WriteAllTextAtomic(
            clientJsonPath,
            JsonConvert.SerializeObject(config, Formatting.Indented));

        Log.Information("创建 UpdateFolder: {Path} (主程序相对路径={RelativePath})", updateFolder, relativePath);
        return warning;
    }

    /// <summary>
    /// Refreshes the updater executables of an <b>already existing</b> <c>UpdateFolder</c> without touching
    /// <c>client.json</c>. Used when a project is added or edited and no main executable is known yet: the
    /// updater files still get the packaged payload copies, but the user's configuration is left alone.
    /// </summary>
    /// <returns>A warning for missing updater executables, or <see langword="null"/> when complete.</returns>
    public static string? RefreshUpdaters(string projectPath)
        => RefreshUpdaters(projectPath, AppDomain.CurrentDomain.BaseDirectory);

    /// <summary>Same as <see cref="RefreshUpdaters(string)"/> with an explicit package root.</summary>
    public static string? RefreshUpdaters(string projectPath, string packageDirectory)
    {
        if (string.IsNullOrWhiteSpace(projectPath))
        {
            return null;
        }

        var projectDirectory = Path.TrimEndingDirectorySeparator(Path.GetFullPath(projectPath));
        var parentDirectory = Path.GetDirectoryName(projectDirectory);
        if (string.IsNullOrWhiteSpace(parentDirectory))
        {
            return null;
        }

        var updateFolder = Path.Combine(parentDirectory, "UpdateFolder");
        if (!Directory.Exists(updateFolder))
        {
            Log.Information("UpdateFolder 不存在，跳过更新器刷新: {Path}", updateFolder);
            return null;
        }

        return CopyUpdaters(updateFolder, projectDirectory, packageDirectory);
    }

    /// <summary>
    /// Copies the updater executables of this platform from the packaged payload into <c>UpdateFolder</c>.
    /// A missing payload is a warning, not a failure: the updater stays a runtime component the user can
    /// still drop in by hand.
    /// </summary>
    private static string? CopyUpdaters(
        string updateFolder,
        string projectDirectory,
        string packageDirectory)
    {
        var warnings = new List<string>();
        foreach (var baseName in HostHelper.UpBaseNames)
        {
            var fileName = HostHelper.UpFileName(baseName);
            var location = PkHelper.FindUp(
                packageDirectory,
                projectDirectory,
                PkHelper.UpRids(packageDirectory),
                fileName);

            if (!location.IsFound)
            {
                warnings.Add(LanguageManager.F(
                    LanguageManager.Instance.Message_UpdateFolder_FileMissing,
                    string.Join(';', location.ProbedPaths)));
                continue;
            }

            if (location.IsFallbackRid)
            {
                Log.Warning(
                    "使用非本机首选平台的更新器 {Name}: {Rid}（首选 {Expected}）",
                    fileName,
                    location.Rid,
                    location.ExpectedRid);
            }

            try
            {
                File.Copy(location.FullName, Path.Combine(updateFolder, fileName), overwrite: true);
                Log.Information(
                    "复制 {Name}: {Source} -> {Destination}",
                    fileName,
                    location.FullName,
                    updateFolder);
            }
            catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
            {
                warnings.Add(LanguageManager.F(
                    LanguageManager.Instance.Message_UpdateFolder_CopyFailed,
                    fileName,
                    ex.Message));
            }
        }

        return warnings.Count == 0
            ? null
            : LanguageManager.F(
                LanguageManager.Instance.Message_UpdateFolder_Incomplete,
                string.Join("; ", warnings));
    }
}
