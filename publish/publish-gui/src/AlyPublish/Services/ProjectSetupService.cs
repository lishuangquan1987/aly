using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Local;

namespace AlyPublish.Services;

/// <summary>Runs the multi-step project initialization: CLI config, optional UpdateFolder, bookmark data.</summary>
public sealed class ProjectSetupService
{
    private readonly CliService _cli;

    public ProjectSetupService(CliService cli)
    {
        _cli = cli;
    }

    public async Task<ProjectSetupResult> InitializeAsync(
        ProjectInitializationRequest request,
        CancellationToken cancellationToken = default)
    {
        var mainExeFullPath = string.Empty;
        if (!string.IsNullOrWhiteSpace(request.MainExePath))
        {
            mainExeFullPath = ProjectInputHelper.ResolveMainExePath(request.ProjectPath, request.MainExePath);
            if (!File.Exists(mainExeFullPath))
            {
                throw new FileNotFoundException(
                    LanguageManager.F(LanguageManager.Instance.Message_Setup_MainMissing, mainExeFullPath));
            }

            var relative = Path.GetRelativePath(request.ProjectPath, mainExeFullPath);
            if (Path.IsPathRooted(relative))
            {
                throw new InvalidOperationException(
                    LanguageManager.T(LanguageManager.Instance.Message_Setup_MainSameDisk));
            }
        }

        await _cli.InitializeProjectAsync(
            request.ProjectPath,
            request.ServerUrl,
            request.ProjectName,
            request.IgnoreFolders,
            request.IgnoreFiles,
            request.UnCopyFolders,
            request.UnCopyFiles,
            cancellationToken);

        var warnings = new List<string>();
        if (mainExeFullPath.Length > 0)
        {
            var warning = UpdateFolderService.Create(request.ProjectPath, mainExeFullPath, request.CloseProcessName);
            if (warning is not null)
            {
                warnings.Add(warning);
            }
        }

        var config = new ProjectConfig
        {
            DisplayName = request.DisplayName,
            ProjectPath = request.ProjectPath,
            MainExePath = ProjectInputHelper.ToStoredMainExePath(request.ProjectPath, request.MainExePath)
        };

        return new ProjectSetupResult(config, warnings);
    }
}
