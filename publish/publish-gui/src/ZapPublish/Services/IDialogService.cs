using System.Threading.Tasks;
using ZapPublish.Models.Cli;
using ZapPublish.Models.Local;

namespace ZapPublish.Services;

public interface IDialogService
{
    Task<ProjectConfig?> ShowAddProjectDialogAsync();
    Task<ProjectInfo?> ShowCreateProjectDialogAsync(string serverUrl);
}
