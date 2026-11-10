using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Local;
using Newtonsoft.Json;
using Serilog;

namespace AlyPublish.Services;

/// <summary>
/// Creates the updater package that sits next to the application folder:
/// <c>PackageFolder/{ApplicationFolder, UpdateFolder}</c>.
/// </summary>
public static class UpdateFolderService
{
    private static readonly string[] ClientExecutableNames = ["aly-client", "aly-client-ui"];

    /// <returns>A warning for missing client executables, or <see langword="null"/> when complete.</returns>
    public static string? Create(string projectPath, string mainExePath, string closeProcessName)
    {
        var projectDirectory = Path.TrimEndingDirectorySeparator(Path.GetFullPath(projectPath));
        var parentDirectory = Path.GetDirectoryName(projectDirectory)
            ?? throw new InvalidOperationException(
                LanguageManager.T(LanguageManager.Instance.Message_UpdateFolder_ParentMissing));
        var updateFolder = Path.Combine(parentDirectory, "UpdateFolder");
        Directory.CreateDirectory(updateFolder);

        var warnings = new List<string>();
        foreach (var baseName in ClientExecutableNames)
        {
            var executableName = OperatingSystem.IsWindows() ? baseName + ".exe" : baseName;
            var source = FindClientExecutable(projectDirectory, executableName);
            if (source is null)
            {
                warnings.Add(LanguageManager.F(LanguageManager.Instance.Message_UpdateFolder_FileMissing, executableName));
                continue;
            }

            try
            {
                File.Copy(source, Path.Combine(updateFolder, executableName), overwrite: true);
                Log.Information("复制 {Executable}: {Source} -> {Destination}", executableName, source, updateFolder);
            }
            catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
            {
                warnings.Add(LanguageManager.F(
                    LanguageManager.Instance.Message_UpdateFolder_CopyFailed,
                    executableName,
                    ex.Message));
            }
        }

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
        return warnings.Count == 0
            ? null
            : LanguageManager.F(
                LanguageManager.Instance.Message_UpdateFolder_Incomplete,
                string.Join("; ", warnings));
    }

    /// <summary>
    /// Looks for the updater next to the GUI, in the application folder, then in the repository
    /// development tree. A missing updater is a warning, not a failure: the user can drop it in.
    /// </summary>
    private static string? FindClientExecutable(string projectDirectory, string executableName)
    {
        var baseDirectory = AppDomain.CurrentDomain.BaseDirectory;
        var candidates = new[]
        {
            Path.Combine(baseDirectory, executableName),
            Path.Combine(projectDirectory, executableName),
            Path.GetFullPath(Path.Combine(
                baseDirectory,
                "..",
                "..",
                "..",
                "..",
                "..",
                "..",
                "..",
                "client",
                "aly-client",
                executableName))
        };

        foreach (var candidate in candidates)
        {
            if (File.Exists(candidate))
            {
                return candidate;
            }
        }

        return null;
    }
}
