using Avalonia.Controls;
using Avalonia.Interactivity;
using PublishTool.ViewModels;
using PublishTool.Views;

namespace PublishTool.Views.Controls;

public partial class ProjectPage : UserControl
{
    public ProjectPage()
    {
        InitializeComponent();
    }

    private void OpenProjectSettings_Click(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not ProjectPageViewModel vm) return;
        if (VisualRoot is TopLevel topLevel && topLevel is Window window
            && window.Owner is MainWindow mainWindow)
        {
            _ = mainWindow.ShowProjectSettingsDialog(vm);
        }
        else if (VisualRoot is MainWindow mainWindow2)
        {
            _ = mainWindow2.ShowProjectSettingsDialog(vm);
        }
    }

    private void OpenConfigEditor_Click(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not ProjectPageViewModel vm) return;
        if (VisualRoot is TopLevel topLevel && topLevel is Window window
            && window.Owner is MainWindow mainWindow)
        {
            _ = mainWindow.ShowConfigEditorDialog(vm);
        }
        else if (VisualRoot is MainWindow mainWindow2)
        {
            _ = mainWindow2.ShowConfigEditorDialog(vm);
        }
    }
}