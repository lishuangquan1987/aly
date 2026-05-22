using System;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.Controls;
using PublishTool.ViewModels;
using PublishTool.Views.Dialogs;

namespace PublishTool.Views.Controls;

public partial class ProjectPage : UserControl
{
    private ProjectPageViewModel? _currentVm;

    public ProjectPage()
    {
        InitializeComponent();
        AttachedToVisualTree += OnAttachedToVisualTree;
    }

    private void OnAttachedToVisualTree(object? sender, VisualTreeAttachmentEventArgs e)
    {
        WireEvents();
    }

    public void WireEvents()
    {
        if (DataContext is ProjectPageViewModel vm && vm != _currentVm)
        {
            if (_currentVm != null)
            {
                _currentVm.SettingsRequested -= OnSettingsRequested;
                _currentVm.ConfigEditorRequested -= OnConfigEditorRequested;
                _currentVm.ChangeLogsRequested -= OnChangeLogsRequested;
            }

            _currentVm = vm;
            _currentVm.SettingsRequested += OnSettingsRequested;
            _currentVm.ConfigEditorRequested += OnConfigEditorRequested;
            _currentVm.ChangeLogsRequested += OnChangeLogsRequested;
        }
    }

    private async Task OnSettingsRequested()
    {
        var window = TopLevel.GetTopLevel(this) as MainWindow;
        if (window != null && _currentVm != null)
            await window.ShowProjectSettingsDialog(_currentVm);
    }

    private async Task OnConfigEditorRequested()
    {
        var window = TopLevel.GetTopLevel(this) as MainWindow;
        if (window != null && _currentVm != null)
            await window.ShowConfigEditorDialog(_currentVm);
    }

    private async Task OnChangeLogsRequested()
    {
        var window = TopLevel.GetTopLevel(this) as MainWindow;
        if (window != null && _currentVm != null)
            await window.ShowChangeLogsDialog(_currentVm);
    }
}
