using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class AddServerProjectDialogViewModel : ObservableObject
{
    private readonly ProjectService _projectService;

    public event Action? CloseRequested;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private string _name = string.Empty;

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private bool _isForceUpdate;

    [ObservableProperty]
    private bool _isCreating;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public AddServerProjectDialogViewModel(ProjectService projectService)
    {
        _projectService = projectService;
    }

    [RelayCommand]
    private async Task Create()
    {
        if (string.IsNullOrEmpty(ServerUrl) || string.IsNullOrEmpty(Name) || string.IsNullOrEmpty(Title))
            return;

        IsCreating = true;
        try
        {
            var dto = new CreateProjectDto
            {
                Name = Name,
                Title = Title,
                IsForceUpdate = IsForceUpdate
            };
            var response = await _projectService.CreateProjectAsync(ServerUrl, dto);
            if (response.IsSuccess)
            {
                StatusMessage = "创建成功";
                CloseRequested?.Invoke();
            }
            else
            {
                StatusMessage = $"创建失败: {response.ErrorMsg}";
            }
        }
        finally
        {
            IsCreating = false;
        }
    }
}
