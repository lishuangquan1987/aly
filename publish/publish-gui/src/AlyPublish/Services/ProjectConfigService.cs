using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Local;
using Newtonsoft.Json;
using Serilog;

namespace AlyPublish.Services;

/// <summary>Reads and writes a project's <c>.updator/shared.json</c> through the CLI.</summary>
public sealed class ProjectConfigService
{
    private const string UpdatorDirectoryName = ".updator";
    private const string SharedFileName = "shared.json";
    private const string ServerUrlKey = "server.url";
    private const string ProjectNameKey = "project.name";
    private const string IgnoreFoldersKey = "ignore.folders";
    private const string IgnoreFilesKey = "ignore.files";
    private const string UnCopyFoldersKey = "un_copy.folders";
    private const string UnCopyFilesKey = "un_copy.files";

    private readonly CliService _cli;

    public ProjectConfigService(CliService cli)
    {
        _cli = cli;
    }

    public static SharedConfig Load(string projectPath)
    {
        var sharedPath = GetSharedConfigPath(projectPath);
        if (!File.Exists(sharedPath))
        {
            throw new ProjectConfigException(
                LanguageManager.F(
                    LanguageManager.Instance.Message_ProjectConfig_SharedMissing,
                    Path.Combine(UpdatorDirectoryName, SharedFileName)));
        }

        try
        {
            var json = File.ReadAllText(sharedPath);
            return JsonConvert.DeserializeObject<SharedConfig>(json)
                ?? throw new ProjectConfigException(
                    LanguageManager.F(LanguageManager.Instance.Message_ProjectConfig_SharedEmpty, SharedFileName));
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or JsonException)
        {
            throw new ProjectConfigException(
                LanguageManager.F(
                    LanguageManager.Instance.Message_ProjectConfig_ReadSharedFailed,
                    SharedFileName,
                    ex.Message),
                ex);
        }
    }

    /// <summary>
    /// Persists all edited values. The current file is re-read first so an old dialog snapshot can
    /// never overwrite changes made elsewhere; only the real differences are sent to the CLI.
    /// </summary>
    /// <returns><see langword="true"/> when at least one value was written to the project configuration.</returns>
    public async Task<bool> SaveAsync(
        string projectPath,
        ProjectConfigDraft draft,
        CancellationToken cancellationToken = default)
    {
        var current = Load(projectPath);
        var changed = false;

        if (!string.Equals(current.ServerUrl, draft.ServerUrl, StringComparison.Ordinal))
        {
            await SetValueAsync(projectPath, ServerUrlKey, draft.ServerUrl, cancellationToken);
            changed = true;
        }

        changed |= await ApplyListAsync(projectPath, IgnoreFoldersKey, current.IgnoreFolders, draft.IgnoreFolders, cancellationToken);
        changed |= await ApplyListAsync(projectPath, IgnoreFilesKey, current.IgnoreFiles, draft.IgnoreFiles, cancellationToken);
        changed |= await ApplyListAsync(projectPath, UnCopyFoldersKey, current.UnCopyFolders, draft.UnCopyFolders, cancellationToken);
        changed |= await ApplyListAsync(projectPath, UnCopyFilesKey, current.UnCopyFiles, draft.UnCopyFiles, cancellationToken);

        if (!string.Equals(current.ProjectName, draft.ProjectName, StringComparison.Ordinal))
        {
            await SetValueAsync(projectPath, ProjectNameKey, draft.ProjectName, cancellationToken);
            changed = true;
        }

        return changed;
    }

    /// <summary>
    /// Pushes the local ignore rules to the server project, keeping its title and force-update flag.
    /// Returns a warning for soft failures; hard CLI failures still throw.
    /// </summary>
    public async Task<string?> SyncToServerAsync(string projectPath, CancellationToken cancellationToken = default)
    {
        var shared = Load(projectPath);
        if (string.IsNullOrWhiteSpace(shared.ServerUrl) || string.IsNullOrWhiteSpace(shared.ProjectName))
        {
            return LanguageManager.T(LanguageManager.Instance.Message_ProjectConfig_NoServer);
        }

        try
        {
            var projects = await _cli.GetServerProjectsAsync(shared.ServerUrl, cancellationToken);
            var project = projects.FirstOrDefault(candidate =>
                string.Equals(candidate.Name, shared.ProjectName, StringComparison.OrdinalIgnoreCase));
            if (project is null)
            {
                return LanguageManager.F(LanguageManager.Instance.Message_ProjectConfig_ProjectMissing, shared.ProjectName);
            }

            await _cli.UpdateServerProjectAsync(
                shared.ServerUrl,
                project.Name,
                project.Title,
                project.IsForceUpdate,
                shared.IgnoreFolders,
                shared.IgnoreFiles,
                cancellationToken);
            return null;
        }
        catch (CliException ex)
        {
            Log.Warning(ex, "同步忽略规则到服务端失败: {Project}", shared.ProjectName);
            return LanguageManager.F(LanguageManager.Instance.Message_ProjectConfig_SyncFailed, ex.Message);
        }
    }

    /// <summary>Server URLs used by existing bookmarks, most useful first.</summary>
    public static IReadOnlyList<string> GetKnownServerUrls(IEnumerable<string> projectPaths)
    {
        var urls = new List<string>();
        foreach (var projectPath in projectPaths)
        {
            try
            {
                var shared = Load(projectPath);
                if (!string.IsNullOrWhiteSpace(shared.ServerUrl))
                {
                    urls.Add(shared.ServerUrl);
                }
            }
            catch (ProjectConfigException ex)
            {
                Log.Debug(ex, "跳过无法读取配置的项目: {Path}", projectPath);
            }
        }

        return urls
            .Distinct(StringComparer.OrdinalIgnoreCase)
            .Take(5)
            .ToList();
    }

    private static string GetSharedConfigPath(string projectPath)
        => Path.Combine(projectPath, UpdatorDirectoryName, SharedFileName);

    private async Task SetValueAsync(
        string projectPath,
        string key,
        string value,
        CancellationToken cancellationToken)
    {
        try
        {
            await _cli.SetConfigAsync(projectPath, key, value, cancellationToken);
        }
        catch (CliException ex)
        {
            throw new CliException(
                LanguageManager.F(LanguageManager.Instance.Message_ProjectConfig_SaveFailed, key, ex.Message),
                ex);
        }
    }

    private async Task<bool> ApplyListAsync(
        string projectPath,
        string key,
        IReadOnlyList<string> current,
        IReadOnlyList<string> desired,
        CancellationToken cancellationToken)
    {
        var diff = ListDiffHelper.Compute(current, desired);
        if (diff.Added.Count == 0 && diff.Removed.Count == 0)
        {
            return false;
        }

        foreach (var item in diff.Removed)
        {
            try
            {
                await _cli.RemoveConfigArrayItemAsync(projectPath, key, item, cancellationToken);
            }
            catch (CliException ex)
            {
                throw new CliException(
                    LanguageManager.F(LanguageManager.Instance.Message_ProjectConfig_RemoveFailed, key, item, ex.Message),
                    ex);
            }
        }

        foreach (var item in diff.Added)
        {
            try
            {
                await _cli.AddConfigArrayItemAsync(projectPath, key, item, cancellationToken);
            }
            catch (CliException ex)
            {
                throw new CliException(
                    LanguageManager.F(LanguageManager.Instance.Message_ProjectConfig_AddFailed, key, item, ex.Message),
                    ex);
            }
        }

        return true;
    }
}
