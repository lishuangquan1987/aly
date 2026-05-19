using System;
using System.Threading.Tasks;
using Avalonia.Controls;
using PublishTool.ViewModels;
using PublishTool.Views.Dialogs;

namespace PublishTool.Views.Controls;

public partial class ProjectPage : UserControl
{
    public ProjectPage()
    {
        InitializeComponent();
        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, EventArgs e)
    {
        if (DataContext is ProjectPageViewModel vm)
        {
            vm.SettingsRequested += () => ShowDialog(dialog =>
                dialog.ShowProjectSettingsDialog(vm));
            vm.ConfigEditorRequested += () => ShowDialog(dialog =>
                dialog.ShowConfigEditorDialog(vm));
        }
    }

    private async Task ShowDialog(Func<MainWindow, Task> action)
    {
        if (VisualRoot is MainWindow window)
        {
            await action(window);
        }
    }
}