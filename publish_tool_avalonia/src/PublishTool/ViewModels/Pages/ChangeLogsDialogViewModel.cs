using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using PublishTool.Models;

namespace PublishTool.ViewModels.Pages;

public partial class ChangeLogsDialogViewModel : ObservableObject
{
    [ObservableProperty]
    private ObservableCollection<ProjectChangeLogDto> _changeLogs = new();

    public ChangeLogsDialogViewModel(ObservableCollection<ProjectChangeLogDto> changeLogs)
    {
        _changeLogs = changeLogs;
    }
}
