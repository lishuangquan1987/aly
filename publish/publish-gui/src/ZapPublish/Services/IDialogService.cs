using System.Threading.Tasks;
using ZapPublish.Models.Cli;
using ZapPublish.Models.Local;

namespace ZapPublish.Services;

public interface IDialogService
{
    Task<ProjectConfig?> ShowAddProjectDialogAsync();
    Task<ProjectConfig?> ShowAddLocalProjectDialogAsync();
    Task<ProjectInfo?> ShowCreateProjectDialogAsync(string serverUrl);
    Task<ProjectConfig?> ShowEditProjectDialogAsync(ProjectConfig project);
}
