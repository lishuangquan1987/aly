using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class AddServerProjectDialogViewModel : ObservableObject
{
    private readonly ProjectService _projectService;

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
            await _projectService.CreateProjectAsync(ServerUrl, dto);
        }
        finally
        {
            IsCreating = false;
        }
    }
}
