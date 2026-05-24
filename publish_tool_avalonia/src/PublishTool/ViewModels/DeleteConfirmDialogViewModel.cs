using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.ViewModels;

public partial class DeleteConfirmDialogViewModel : ObservableObject
{
    [ObservableProperty]
    private string _projectTitle = string.Empty;

    public DeleteConfirmDialogViewModel(string projectTitle)
    {
        _projectTitle = projectTitle;
    }
}
