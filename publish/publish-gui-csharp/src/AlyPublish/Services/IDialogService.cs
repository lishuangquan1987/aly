using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;

namespace AlyPublish.Services;

/// <summary>The single UI entry point for modal dialogs and confirmations.</summary>
public interface IDialogService
{
    Task<ProjectConfig?> ShowAddProjectAsync();

    Task<ProjectConfig?> ShowAddLocalProjectAsync();

    Task<ProjectConfig?> ShowEditProjectAsync(ProjectConfig project);

    Task<ProjectInfo?> ShowCreateProjectAsync(CreateServerProjectRequest request);

    Task<bool> ConfirmAsync(string title, string message);

    Task ErrorAsync(string title, string message);
}
